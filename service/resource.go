package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ResourceManager handles ordered shutdown and health check aggregation for resources.
// Resources are closed in LIFO (last-in-first-out) order during shutdown.
type ResourceManager struct {
	resources []resource
}

type resource struct {
	name        string
	closer      func()
	healthCheck func(context.Context) error
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager() *ResourceManager {
	return &ResourceManager{}
}

// Register adds a resource to be managed.
// - name: identifier for logging and health check errors
// - closer: called during shutdown (may be nil if no cleanup needed)
// - healthCheck: called for health endpoint (may be nil if no health check needed)
func (m *ResourceManager) Register(name string, closer func(), healthCheck func(context.Context) error) {
	m.resources = append(m.resources, resource{name, closer, healthCheck})
}

// Close shuts down all resources in LIFO order.
// This should be called from http.Service.PostShutdown or similar shutdown hook.
func (m *ResourceManager) Close() {
	for i := len(m.resources) - 1; i >= 0; i-- {
		r := m.resources[i]
		if r.closer != nil {
			slog.Info("closing resource", "name", r.name)
			start := time.Now()
			r.closer()
			slog.Info("closed resource", "name", r.name, "duration", time.Since(start))
		}
	}
}

// HealthHandler returns an http.HandlerFunc that checks all registered health checks.
// If any health check fails, it returns 503 Service Unavailable with the error message.
// If all health checks pass, it returns 200 OK with "ok" body.
func (m *ResourceManager) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, res := range m.resources {
			if res.healthCheck != nil {
				start := time.Now()
				if err := res.healthCheck(r.Context()); err != nil {
					slog.Warn("health check failed", "name", res.name, "error", err, "duration", time.Since(start))
					http.Error(w, fmt.Sprintf("%s: %v", res.name, err), http.StatusServiceUnavailable)
					return
				}
			}
		}
		fmt.Fprint(w, "ok")
	}
}
