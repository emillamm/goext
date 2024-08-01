package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestServer(t *testing.T) {

	ctx := context.Background()

	t.Run("ListenAndServe should return ErrServerClosed if the server is closed", func (t *testing.T) {
		ctx, _ := context.WithTimeout(ctx, 50 * time.Millisecond)
		server := &http.Server{Addr: net.JoinHostPort("localhost", "5001")}
		server.Close()
		select {
		case err:= <-ListenAndServe(ctx, server):
			if !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("want ErrServerClosed, got %v", err)
			}
		}
	})

	t.Run("ListenAndServe should shutdown server and return DeadlineExceeded error when context expires", func (t *testing.T) {
		ctx, _ := context.WithTimeout(ctx, 10 * time.Millisecond)
		server := &http.Server{Addr: net.JoinHostPort("localhost", "5001")}
		select {
		case err:= <-ListenAndServe(ctx, server):
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("want DeadlineExceeded, got %v", err)
			}
		}
	})

	t.Run("WaitForReady should return nil error when check succeeds", func (t *testing.T) {
		ctx, _ := context.WithTimeout(ctx, 50 * time.Millisecond)
		ticker := time.NewTicker(10 * time.Millisecond)
		timeout := 10 * time.Millisecond
		var checks atomic.Int32
		readyCheck := func(ctx context.Context) bool {
			checks.Add(1)
			return true
		}
		select {
		case err := <-WaitForReady(ctx, ticker, timeout, readyCheck):
			if err != nil {
				t.Errorf("ready check failed: %v", err)
			}
		}
		if checks.Load() != 1 {
			t.Errorf("checks: got %d, want %d", checks.Load(), 1)
		}
	})

	t.Run("WaitForReady should return DeadlineExceeded error when all checks failed", func (t *testing.T) {
		ctx, _ := context.WithTimeout(ctx, 50 * time.Millisecond)
		ticker := time.NewTicker(10 * time.Millisecond)
		timeout := 10 * time.Millisecond
		var checks atomic.Int32
		readyCheck := func(ctx context.Context) bool {
			checks.Add(1)
			return false
		}
		select {
		case err := <-WaitForReady(ctx, ticker, timeout, readyCheck):
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("ready check failed: %v", err)
			}
		}
		if checks.Load() != 5 {
			t.Errorf("checks: got %d, want %d", checks.Load(), 5)
		}
	})

	// This test is solely for demonstrating errors returned in the happy path when gracefully shutting down the server
	t.Run("server.Shutdown(ctx) should return no error while ListenAndServe returns ErrServerClosed", func (t *testing.T) {
		ctx, _ := context.WithTimeout(ctx, 50 * time.Millisecond)
		server := &http.Server{Addr: net.JoinHostPort("localhost", "5001")}
		done := make(chan struct{})
		go func() {
			select {
			// start server
			case err:= <-ListenAndServe(ctx, server):
				if !errors.Is(err, http.ErrServerClosed) {
					t.Errorf("want ErrServerClosed, got %#v", err)
				}
				// signal the server is closed and testing is done
				close(done)
			}
		}()
		shutdownCtx, _ := context.WithTimeout(ctx, 10 * time.Millisecond)
		// gracefully shut down the server
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Errorf("shutdown returned error: %v", err)
		}
		select {
		case <-done:
			// testing is done
			break
		}
	})
}

