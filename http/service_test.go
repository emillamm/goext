package http

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func setupService(t testing.TB) (*Service, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	service, err := NewService(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}

	// Add health check for readiness
	service.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cleanup := func() {
		service.Stop()
		service.WaitForDone()
		cancel()
	}

	t.Cleanup(cleanup)
	return service, ctx
}

func startService(service *Service, ctx context.Context) {
	service.Start(ctx)
	service.WaitForReady()
}

func TestMiddleware(t *testing.T) {
	t.Run("Global middleware should be applied to all routes", func(t *testing.T) {
		service, ctx := setupService(t)

		service.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Global", "true")
				next.ServeHTTP(w, r)
			})
		})

		service.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		startService(service, ctx)

		resp, err := http.Get(service.BaseUrl() + "/test")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.Header.Get("X-Global") != "true" {
			t.Errorf("expected X-Global header to be 'true', got '%s'", resp.Header.Get("X-Global"))
		}
	})

	t.Run("Per-endpoint middleware should be applied to specific routes", func(t *testing.T) {
		service, ctx := setupService(t)

		authMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Auth", "required")
				next.ServeHTTP(w, r)
			})
		}

		service.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("protected"))
		}, authMiddleware)

		service.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("public"))
		})

		startService(service, ctx)

		// Check protected route has header
		resp, err := http.Get(service.BaseUrl() + "/protected")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.Header.Get("X-Auth") != "required" {
			t.Errorf("expected X-Auth header on /protected, got '%s'", resp.Header.Get("X-Auth"))
		}

		// Check public route does not have header
		resp2, err := http.Get(service.BaseUrl() + "/public")
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()

		if resp2.Header.Get("X-Auth") != "" {
			t.Errorf("expected no X-Auth header on /public, got '%s'", resp2.Header.Get("X-Auth"))
		}
	})

	t.Run("Middleware should execute in correct order", func(t *testing.T) {
		service, ctx := setupService(t)

		service.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Order", "global")
				next.ServeHTTP(w, r)
			})
		})

		endpointMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Order", "endpoint")
				next.ServeHTTP(w, r)
			})
		}

		service.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Order", "handler")
			w.WriteHeader(http.StatusOK)
		}, endpointMiddleware)

		startService(service, ctx)

		resp, err := http.Get(service.BaseUrl() + "/test")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		order := resp.Header.Values("X-Order")
		expected := []string{"global", "endpoint", "handler"}

		if len(order) != len(expected) {
			t.Fatalf("expected %d headers, got %d", len(expected), len(order))
		}

		for i, v := range expected {
			if order[i] != v {
				t.Errorf("at position %d: expected '%s', got '%s'", i, v, order[i])
			}
		}
	})

	t.Run("Middleware can short-circuit request", func(t *testing.T) {
		service, ctx := setupService(t)

		authMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("unauthorized"))
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		service.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}, authMiddleware)

		startService(service, ctx)

		// Request without auth header
		resp, err := http.Get(service.BaseUrl() + "/protected")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "unauthorized") {
			t.Errorf("expected 'unauthorized', got '%s'", string(body))
		}

		// Request with auth header
		req, _ := http.NewRequest("GET", service.BaseUrl()+"/protected", nil)
		req.Header.Set("Authorization", "Bearer token")
		resp2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp2.StatusCode)
		}

		body2, _ := io.ReadAll(resp2.Body)
		if !strings.Contains(string(body2), "success") {
			t.Errorf("expected 'success', got '%s'", string(body2))
		}
	})
}
