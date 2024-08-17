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
	ReadyCheckTimeout time.Duration 	// How long a ready check is allowed to take before a timeout
	ReadyTickInterval time.Duration 	// How long to wait before a new ready check is fired
	ReadyTickTimeout time.Duration 		// How long to wait in total for a successful ready check
	ShutdownTimeout time.Duration 		// How long to wait for shutdown
	ReadyCheck func(context.Context) bool 	// Ready check to perform
	// private
	baseUrl string
	doneChan chan struct{}
	doneWG sync.WaitGroup
	err error
}

func NewService(env envx.EnvX) (*Service, error) {

	var errs envx.Errors

	host := env.Getenv(
		"HTTP_HOST", envx.Default("localhost"))
	hostScheme := env.Getenv(
		"HTTP_HOST_SCHEME", envx.Default("http"))
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

	baseUrl := fmt.Sprintf("%s://%s:%d", hostScheme, host, port)
	readyCheck := defaultReadyCheck(fmt.Sprintf("%s/%s", baseUrl, readyCheckPath))
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
		baseUrl: baseUrl,
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
	select {
	case <-s.doneChan:
		return
	default:
		close(s.doneChan)
	}
}

func (s *Service) WaitForReady() {
	select {
	case <-s.doneChan:
		// short circuit if server is already done
		return
	default:
		break
	}

	ctx, cancel := context.WithCancel(context.Background())
	select {
	case <-s.doneChan:
		// if server becomes done, stop the ongoing ready check by cancelling the context
		cancel()
	case err := <-waitForReadyChan(ctx, s.ReadyCheckTimeout, s.ReadyTickInterval, s.ReadyTickTimeout, s.ReadyCheck):
		if err != nil {
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

func (s *Service) BaseUrl() string {
	return s.baseUrl
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

