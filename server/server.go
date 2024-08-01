package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

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

func WaitForReady(
	ctx context.Context,
	ticker *time.Ticker,
	timeout time.Duration,
	readyCheck func(context.Context)bool,
) <-chan error {
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
				ctx, _ := context.WithTimeout(ctx, timeout)
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

