// Package e2etest provides utilities for running E2E tests with infrastructure services.
//
// The recommended pattern is for tests to create infrastructure services and keep typed
// references, then pass them to the Runner:
//
//	pgSvc := pgtest.NewService(os.Getenv)  // Test keeps typed reference
//	runner := e2etest.New(t, e2etest.Config{
//	    BaseEnv:        os.Getenv,
//	    ServiceFactory: app.NewService,
//	    Infrastructure: []service.Service{pgSvc},
//	    EnvModifier:    envOverrides,
//	})
//	defer runner.Stop()
//
//	pool, _ := pgSvc.Connect()  // Direct typed access, no type assertions needed
package e2etest

import (
	"context"
	"testing"

	"github.com/emillamm/envx"
	"github.com/emillamm/goext/http"
	"github.com/emillamm/goext/service"
)

// ServiceFactory creates an HTTP service from context and environment.
type ServiceFactory func(ctx context.Context, env envx.EnvX) (*http.Service, error)

// EnvModifier allows tests to customize environment after infrastructure env is set.
type EnvModifier func(env envx.EnvX) envx.EnvX

// Config holds configuration for the test runner.
type Config struct {
	// BaseEnv is the base environment (typically os.Getenv)
	BaseEnv envx.EnvX
	// ServiceFactory creates the application's HTTP service
	ServiceFactory ServiceFactory
	// EnvModifier allows customizing the test environment (optional)
	// This is called after infrastructure env is composed
	EnvModifier EnvModifier
	// Infrastructure is a list of test services to start before the app service.
	// They are started in order and stopped in reverse order.
	// Services implementing service.EnvProvider will have their Env() composed.
	Infrastructure []service.Service
}

// Runner manages test infrastructure for E2E tests.
type Runner struct {
	// Infrastructure contains all test infrastructure services
	Infrastructure []service.Service
	// HttpService is the main application HTTP service
	HttpService *http.Service
	ctx         context.Context
}

// New creates and starts a new test runner.
// It starts infrastructure services in order, composes their environments,
// then creates and starts the application HTTP service.
func New(t *testing.T, cfg Config) *Runner {
	t.Helper()

	ctx := context.Background()
	env := cfg.BaseEnv

	// Start infrastructure services in order
	for i, infra := range cfg.Infrastructure {
		infra.Start(ctx)
		infra.WaitForReady()
		if err := infra.Err(); err != nil {
			// Cleanup already started infrastructure
			for j := i - 1; j >= 0; j-- {
				cfg.Infrastructure[j].Stop()
				cfg.Infrastructure[j].WaitForDone()
			}
			t.Fatalf("failed to start infrastructure service %d: %v", i, err)
		}

		// Compose env from services that provide it
		if ep, ok := infra.(service.EnvProvider); ok {
			env = ep.Env()
		}
	}

	// Apply custom env modifications if provided
	if cfg.EnvModifier != nil {
		env = cfg.EnvModifier(env)
	}

	// Create and start the application HTTP service
	httpService, err := cfg.ServiceFactory(ctx, env)
	if err != nil {
		// Cleanup infrastructure
		for i := len(cfg.Infrastructure) - 1; i >= 0; i-- {
			cfg.Infrastructure[i].Stop()
			cfg.Infrastructure[i].WaitForDone()
		}
		t.Fatalf("failed to create HTTP service: %v", err)
	}

	httpService.Start(ctx)
	httpService.WaitForReady()
	if err := httpService.Err(); err != nil {
		// Cleanup infrastructure
		for i := len(cfg.Infrastructure) - 1; i >= 0; i-- {
			cfg.Infrastructure[i].Stop()
			cfg.Infrastructure[i].WaitForDone()
		}
		t.Fatalf("failed to start HTTP service: %v", err)
	}

	return &Runner{
		Infrastructure: cfg.Infrastructure,
		HttpService:    httpService,
		ctx:            ctx,
	}
}

// Stop gracefully shuts down the runner.
// It stops the HTTP service first, then infrastructure in reverse order.
func (r *Runner) Stop() {
	// Stop HTTP service first
	if r.HttpService != nil {
		r.HttpService.Stop()
		r.HttpService.WaitForDone()
	}

	// Stop infrastructure in reverse order
	for i := len(r.Infrastructure) - 1; i >= 0; i-- {
		r.Infrastructure[i].Stop()
		r.Infrastructure[i].WaitForDone()
	}
}

// Env returns the composed environment from all infrastructure services.
// Useful for making test HTTP requests or other test setup.
func (r *Runner) Env() envx.EnvX {
	env := func(s string) string { return "" }
	for _, infra := range r.Infrastructure {
		if ep, ok := infra.(service.EnvProvider); ok {
			env = ep.Env()
		}
	}
	return env
}
