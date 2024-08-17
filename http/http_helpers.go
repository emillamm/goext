package http

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Convenient wrapper around http.Server.ListenAndServe which takes a context and handles shutdown
// if the context is done.
func ListenAndServe(
	ctx context.Context,
	server *http.Server,
) <-chan error {

	listenChan := make(chan error)
	errChan := make(chan error)

	// start serving if context isn't already done
	if ctx.Err() == nil {
		go func() {
			listenChan <- server.ListenAndServe()
		}()
	}

	go func() {
		select  {
		case <-ctx.Done():
			// Forcefully shutdown server ignoring error if context expires (No graceful shutdown).
			// In a production setting, provide context.Background() which never expires and call Shutdown(...)
			// to gracefully shut down the server instead.
			server.Close()
			errChan <- fmt.Errorf("server context is done: %w", context.Cause(ctx))
		case err := <- listenChan:
			errChan <- err
		}
	}()
	return errChan
}

type ReadyCheckConfig struct {
	// How long a ready check is allowed to take before a timeout
	ReadyCheckTimeout time.Duration
	// How long to wait before a new ready check is fired
	ReadyTickInterval time.Duration
	// How long to wait in total for a successful ready check
	ReadyTickTimeout time.Duration
	// Ready check to perform
	ReadyCheck func(context.Context) bool
}

func WaitForReady(
	ctx context.Context,
	readyCheckTimeout time.Duration,
	readyTickInterval time.Duration,
	readyTickTimeout time.Duration,
	readyCheck func(context.Context) bool,
) error {
	select {
	case err := <- waitForReadyChan(ctx, readyCheckTimeout, readyTickInterval, readyTickTimeout, readyCheck):
		return err
	}
}

func waitForReadyChan(
	ctx context.Context,
	readyCheckTimeout time.Duration,
	readyTickInterval time.Duration,
	readyTickTimeout time.Duration,
	readyCheck func(context.Context) bool,
) <-chan error {
	ctx, _ = context.WithTimeout(ctx, readyTickTimeout)
	ticker := time.NewTicker(readyTickInterval)
	errChan := make(chan error)
	doneChan := make(chan struct{})
	closeDoneOnce := sync.OnceFunc(func() {
		close(doneChan)
	})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): //context cancelled or expired
				errChan <- fmt.Errorf("context is done: %w", context.Cause(ctx))
				break
			case <-ticker.C: // it is time to perform a readycheck
				ctx, _ := context.WithTimeout(ctx, readyCheckTimeout)
				go func() {
					if (readyCheck(ctx)) {
						closeDoneOnce()
					}
				}()
			case <-doneChan: // a readycheck succeeded
				errChan <- nil
				break
			}
		}
	}()
	return errChan
}

