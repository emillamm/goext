package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/emillamm/envx"
)

type Service struct {
	// public
	Env               envx.EnvX
	HttpHost          string
	HttpPort          int
	Mux               *http.ServeMux
	PostShutdownHook  func()
	ReadyCheckTimeout time.Duration              // How long a ready check is allowed to take before a timeout
	ReadyTickInterval time.Duration              // How long to wait before a new ready check is fired
	ReadyTickTimeout  time.Duration              // How long to wait in total for a successful ready check
	ShutdownTimeout   time.Duration              // How long to wait for shutdown
	ReadyCheck        func(context.Context) bool // Ready check to perform
	// private
	baseUrl    string
	doneChan   chan struct{}
	doneWG     sync.WaitGroup
	err        error
	middleware []func(http.Handler) http.Handler
}

func NewService(env envx.EnvX) (*Service, error) {
	checks := envx.NewChecks()

	host := envx.Check(env.String("HTTP_HOST").Default("localhost"))(checks)
	hostScheme := envx.Check(env.String("HTTP_HOST_SCHEME").Default("http"))(checks)
	port := envx.Check(env.Int("HTTP_PORT").Default(5001))(checks)
	readyCheckPath := envx.Check(env.String("HTTP_READY_CHECK_PATH").Default("/health"))(checks)
	readyCheckTimeout := envx.Check(env.Duration("HTTP_READY_CHECK_TIMEOUT").Default(200 * time.Millisecond))(checks)
	readyTickInterval := envx.Check(env.Duration("HTTP_READY_TICK_INTERVAL").Default(200 * time.Millisecond))(checks)
	readyTickTimeout := envx.Check(env.Duration("HTTP_READY_TICK_TIMEOUT").Default(1 * time.Second))(checks)
	shutdownTimeout := envx.Check(env.Duration("HTTP_READY_TICK_TIMEOUT").Default(15 * time.Second))(checks)

	if err := checks.Err(); err != nil {
		return nil, err
	}

	baseUrl := fmt.Sprintf("%s://%s:%d", hostScheme, host, port)
	readyCheck := defaultReadyCheck(fmt.Sprintf("%s/%s", baseUrl, readyCheckPath))
	mux := http.NewServeMux()

	return &Service{
		Env:               env,
		HttpHost:          host,
		HttpPort:          port,
		Mux:               mux,
		ReadyCheckTimeout: readyCheckTimeout,
		ReadyTickInterval: readyTickInterval,
		ReadyTickTimeout:  readyTickTimeout,
		ShutdownTimeout:   shutdownTimeout,
		ReadyCheck:        readyCheck,
		baseUrl:           baseUrl,
		doneChan:          make(chan struct{}),
	}, nil
}

func (s *Service) Start(ctx context.Context) {
	s.doneWG.Add(1)

	srv := &http.Server{
		Addr:    net.JoinHostPort(s.HttpHost, strconv.Itoa(s.HttpPort)),
		Handler: s.buildHandler(),
	}

	// start server
	go func() {
		// no need to handle context.Done() on startup as that is already handled by the ListenAndServe method
		select {
		case err := <-ListenAndServe(ctx, srv):
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
		if s.PostShutdownHook != nil {
			s.PostShutdownHook()
		}
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

func (s *Service) Handle(pattern string, handler http.Handler, middleware ...func(http.Handler) http.Handler) {
	wrapped := chain(handler, middleware...)
	s.Mux.Handle(pattern, wrapped)
}

func (s *Service) HandleFunc(pattern string, handlerFunc http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	s.Handle(pattern, handlerFunc, middleware...)
}

// Add global middleware
func (s *Service) Use(mw ...func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, mw...)
}

func (s *Service) PostShutdown(postShutdownHook func()) {
	s.PostShutdownHook = postShutdownHook
}

func (s *Service) BaseUrl() string {
	return s.baseUrl
}

func (s *Service) buildHandler() http.Handler {
	return chain(s.Mux, s.middleware...)
}

func chain(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func defaultReadyCheck(url string) func(context.Context) bool {
	return func(ctx context.Context) bool {
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
