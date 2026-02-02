package service

import (
	"context"

	"github.com/emillamm/envx"
)

// Service is the core async lifecycle interface for long-running services.
type Service interface {
	// Start service async. If startup fails, it will transition to done.
	Start(ctx context.Context)
	// Stop service async. Will be marked as stopped even if it was never started.
	Stop()
	// Waits for service to be ready or done. If it is already done, it should not have an effect calling this method.
	WaitForReady()
	// Waits for service to be done
	WaitForDone()
	// Any error that happened during startup, running or shutdown
	Err() error
}

// EnvProvider is optionally implemented by services that modify the environment.
// Used by e2etest to compose env from infrastructure services.
type EnvProvider interface {
	Env() envx.EnvX
}

// HealthChecker is optionally implemented by services that provide health status.
// Used by ResourceManager to aggregate health checks.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}
