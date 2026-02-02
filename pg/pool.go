package pg

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewTracedPool creates a pgxpool.Pool with OpenTelemetry tracing enabled.
// The tracer will automatically instrument all database operations.
func NewTracedPool(ctx context.Context, params ConnectionParams) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(params.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	return pool, nil
}

// NewPool creates a pgxpool.Pool without tracing.
// Use NewTracedPool if you want OpenTelemetry instrumentation.
func NewPool(ctx context.Context, params ConnectionParams) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, params.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}
	return pool, nil
}
