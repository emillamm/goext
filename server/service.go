package server

import (
	"time"
	"net/http"
	"sync"
	"github.com/emillamm/envx"
	"context"
	"fmt"
)

type Service struct {
	env envx.EnvX
	server *http.Server
	postShutdownHook func()
	readyCheckConfig ReadyCheckConfig
	shutdownTimeout time.Duration
	doneChan chan struct{}
	doneWG sync.WaitGroup
	err error
}

func NewService(
	env envx.EnvX,
	server *http.Server,
	postShutdownHook func(),
	readyCheckConfig ReadyCheckConfig,
	shutdownTimeout time.Duration,
) *Service {
	return &Service{
		env: env,
		server: server,
		postShutdownHook: postShutdownHook,
		readyCheckConfig: readyCheckConfig,
		shutdownTimeout: shutdownTimeout,
		doneChan: make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.doneWG.Add(1)

	// start server
	go func() {
		// no need to handle context.Done() on startup as that is already handled by the ListenAndServe method
		select {
		case err := <- ListenAndServe(ctx, s.server):
			if err != nil && err != http.ErrServerClosed {
				s.err = err
			}
			close(s.doneChan)
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
		if err := shutdown(ctx, s.server, s.shutdownTimeout); err != nil {
			s.err = err
		}
		s.postShutdownHook()
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
		if err := WaitForReady(context.Background(), s.readyCheckConfig); err != nil {
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

