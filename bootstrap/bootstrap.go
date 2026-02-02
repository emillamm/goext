package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/emillamm/envx"
	"github.com/emillamm/goext/observability"
	"github.com/emillamm/goext/service"
)

// ServiceFactory creates a service from context and environment.
// This is the primary extension point for consuming applications.
type ServiceFactory func(ctx context.Context, env envx.EnvX) (service.Service, error)

// Config holds bootstrap configuration.
type Config struct {
	// Env is the environment variable accessor (typically os.Getenv)
	Env envx.EnvX
	// ServiceFactory creates the application's main service
	ServiceFactory ServiceFactory
	// Signals to listen for (defaults to SIGINT, SIGHUP, SIGQUIT, SIGTERM if nil)
	Signals []os.Signal
}

// DefaultSignals returns the standard set of termination signals.
func DefaultSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM}
}

// Run executes the full service lifecycle:
// 1. Initialize observability (tracing + logging)
// 2. Create and start the service
// 3. Wait for ready
// 4. Block until termination signal
// 5. Graceful shutdown
//
// Returns exit code: 0 for success, 1 for error.
func Run(cfg Config) int {
	ctx := context.Background()

	// Set default signals if not provided
	if len(cfg.Signals) == 0 {
		cfg.Signals = DefaultSignals()
	}

	// Initialize observability (tracing + logging)
	obsConfig, err := observability.LoadConfig(cfg.Env)
	if err != nil {
		slog.Error("failed to load observability config", "error", err)
		return 1
	}

	obsProvider, err := observability.New(ctx, obsConfig)
	if err != nil {
		slog.Error("failed to initialize observability", "error", err)
		return 1
	}
	defer obsProvider.Shutdown(ctx)

	slog.Info("observability initialized",
		"service", obsConfig.ServiceName,
		"environment", obsConfig.Environment,
	)

	// Create the service
	svc, err := cfg.ServiceFactory(ctx, cfg.Env)
	if err != nil {
		slog.Error("failed to initialize service", "error", err)
		return 1
	}

	// Start and wait for ready
	svc.Start(ctx)
	svc.WaitForReady()
	if err := svc.Err(); err != nil {
		slog.Error("failed to start service", "error", err)
		return 1
	}

	slog.Info("service started")

	// Register shutdown signal handler
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, cfg.Signals...)
	go func() {
		sig := <-osSignal
		slog.Info("gracefully shutting down server", "signal", sig.String())
		svc.Stop()
	}()

	// Wait for service to complete
	svc.WaitForDone()
	if err := svc.Err(); err != nil {
		slog.Error("service terminated with error", "error", err)
		return 1
	}

	slog.Info("service stopped")
	return 0
}
