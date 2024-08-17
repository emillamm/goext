package server

import (
	"time"
	"net/http"
	"sync"
	"github.com/emillamm/envx"
	"context"
	"fmt"
	"net"
	"strconv"
)

type Service struct {
	// public
	Env envx.EnvX
	HttpHost string
	HttpPort int
	Mux *http.ServeMux
	PostShutdownHook func()
	// How long a ready check is allowed to take before a timeout
	ReadyCheckTimeout time.Duration
	// How long to wait before a new ready check is fired
	ReadyTickInterval time.Duration
	// How long to wait in total for a successful ready check
	ReadyTickTimeout time.Duration
	// How long to wait for shutdown
	ShutdownTimeout time.Duration
	// Ready check to perform
	ReadyCheck func(context.Context) bool
	// private
	doneChan chan struct{}
	doneWG sync.WaitGroup
	err error
}

func NewService(
	env envx.EnvX,
	//server *http.Server,
	//postShutdownHook func(),
	//readyCheckConfig ReadyCheckConfig,
	//shutdownTimeout time.Duration,
) (*Service, error) {

	var errs envx.Errors

	host := env.Getenv(
		"HTTP_HOST", envx.Default("localhost"))
	port := env.AsInt().Getenv(
		"HTTP_PORT", envx.Default[int](5001), envx.Observe[int](&errs))
	readyCheckPath := env.Getenv(
		"HTTP_READY_CHECK_PATH", envx.Default[string]("/health"))
	readyCheckTimeout := env.AsDuration().Getenv(
		"HTTP_READY_CHECK_TIMEOUT", envx.Default[time.Duration](200 * time.Millisecond), envx.Observe[time.Duration](&errs))
	readyTickInterval := env.AsDuration().Getenv(
		"HTTP_READY_TICK_INTERVAL", envx.Default[time.Duration](200 * time.Millisecond), envx.Observe[time.Duration](&errs))
	readyTickTimeout := env.AsDuration().Getenv(
		"HTTP_READY_TICK_TIMEOUT", envx.Default[time.Duration](1 * time.Second), envx.Observe[time.Duration](&errs))
	shutdownTimeout := env.AsDuration().Getenv(
		"HTTP_READY_TICK_TIMEOUT", envx.Default[time.Duration](15 * time.Second), envx.Observe[time.Duration](&errs))

	if err := errs.Error(); err != nil {
		return nil, err
	}

	readyCheck := defaultReadyCheck(fmt.Sprintf("%s:%d/%s", host, port, readyCheckPath))
	mux := http.NewServeMux()

	return &Service{
		Env: env,
		HttpHost: host,
		HttpPort: port,
		Mux: mux,
		ReadyCheckTimeout: readyCheckTimeout,
		ReadyTickInterval: readyTickInterval,
		ReadyTickTimeout: readyTickTimeout,
		ShutdownTimeout: shutdownTimeout,
		ReadyCheck: readyCheck,
		doneChan: make(chan struct{}),
	}, nil
}

func (s *Service) Start(ctx context.Context) {
	s.doneWG.Add(1)

	srv := &http.Server{
		Addr:    net.JoinHostPort(s.HttpHost, strconv.Itoa(s.HttpPort)),
		Handler: s.Mux,
	}

	// start server
	go func() {
		// no need to handle context.Done() on startup as that is already handled by the ListenAndServe method
		select {
		case err := <- ListenAndServe(ctx, srv):
			if err != nil && err != http.ErrServerClosed {
				s.err = err
				close(s.doneChan)
			}
		}
	}()

	// handle shutdown
	go func() {
		select {
		case <-ctx.Done():
			s.err = fmt.Errorf("context cancelled prematurely: %w", context.Cause(ctx))
		case <-s.doneChan:
			break
		}
		if err := shutdown(ctx, srv, s.ShutdownTimeout); err != nil {
			s.err = err
		}
		s.PostShutdownHook()
		s.doneWG.Done() // mark service as done
	}()
}

func (s *Service) Stop() {
	close(s.doneChan)
}

func (s *Service) WaitForReady() {
	select {
	case <-s.doneChan:
		// short circuit if server is done
		break
	default:
		// perform ready check
		if err := WaitForReady(context.Background(), s.ReadyCheckTimeout, s.ReadyTickInterval, s.ReadyTickTimeout, s.ReadyCheck); err != nil {
			s.err = fmt.Errorf("ready check failed: %w", err)
		}
	}
}

func (s *Service) WaitForDone() {
	s.doneWG.Wait()
}

func (s *Service) Err() error {
	return s.err
}

func (s *Service) Handle(pattern string, handler http.Handler) {
	s.Mux.Handle(pattern, handler)
}

func (s *Service) PostShutdown(postShutdownHook func()) {
	s.PostShutdownHook = postShutdownHook
}

func defaultReadyCheck(url string) func(context.Context) bool {
	return func (ctx context.Context) bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		if resp.StatusCode != 200 {
			return false
		}
		return true
	}
}

func shutdown(
	ctx context.Context,
	srv *http.Server,
	shutdownTimeout time.Duration,
) error {
	ctx, _ = context.WithTimeout(ctx, shutdownTimeout)
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shut down server gracefully: %w", err)
	}
	return nil
}

