# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Go library (`goext`) that provides reusable utilities for building Go applications, particularly focused on HTTP services and PostgreSQL database testing. The library emphasizes clean lifecycle management with a unified Service interface pattern.

## Commands

### Testing
```bash
# Run all tests
go test ./... -v

# Run tests for a specific package
go test ./http -v
go test ./pgtest -v

# Run a specific test
go test ./pgtest -v -run "TestSession"
```

### Building
```bash
# Build all packages
go build ./...

# Get dependencies
go mod tidy
```

## Architecture

### Service Interface Pattern

The codebase uses a consistent `Service` interface (defined in `service/service.go`) for managing component lifecycle:

```go
type Service interface {
    Start(ctx context.Context)  // Start service async
    Stop()                       // Stop service async
    WaitForReady()              // Block until service is ready or done
    WaitForDone()               // Block until service has fully stopped
    Err() error                 // Return any error from startup/running/shutdown
}
```

All services implement this interface, allowing consistent composition and lifecycle management across HTTP servers, database connections, and test utilities.

### Key Packages

#### `http/` - HTTP Service Management
- **`http.Service`**: Production HTTP server with graceful startup/shutdown
  - Configurable via environment variables (uses `envx` library)
  - Built-in health check endpoint and ready check mechanism
  - Automatic graceful shutdown with configurable timeout
  - Wraps `http.ServeMux` for route handling
- **`http.ListenAndServe()`**: Context-aware wrapper around `http.Server.ListenAndServe`
- **`http.WaitForReady()`**: Polling-based ready check with timeout configuration

Environment variables:
- `HTTP_HOST` (default: "localhost")
- `HTTP_PORT` (default: 5001)
- `HTTP_HOST_SCHEME` (default: "http")
- `HTTP_READY_CHECK_PATH` (default: "/health")
- `HTTP_READY_CHECK_TIMEOUT` (default: 200ms)
- `HTTP_READY_TICK_INTERVAL` (default: 200ms)
- `HTTP_READY_TICK_TIMEOUT` (default: 1s)
- `HTTP_SHUTDOWN_TIMEOUT` (default: 15s)

#### `pg/` - PostgreSQL Connection Utilities
- **`pg.ConnectionParams`**: Standard struct for Postgres connection details
  - Provides `ConnectionString()` method for pgx
  - Provides `EnvOverwrite(envx.EnvX)` for environment variable composition
- **`pg.LoadConnectionParams(env)`**: Loads connection params from environment

Environment variables:
- `POSTGRES_HOST` (default: "localhost")
- `POSTGRES_PORT` (default: 5432)
- `POSTGRES_DATABASE` (required)
- `POSTGRES_USER` (required)
- `POSTGRES_PASS` (required)

#### `pgtest/` - PostgreSQL Test Utilities
- **`pgtest.SessionManager`**: Creates and manages ephemeral test databases
  - Each test gets its own isolated database and role
  - Automatic cleanup after tests
  - Convention: looks for migrations in `../migrations` directory
- **`pgtest.EphemeralSession`**: Represents an isolated test database
  - Returns `*pgxpool.Pool` connection (pgx v5)
  - Auto-closes connections on cleanup
- **`pgtest.Service`**: Service wrapper for integration tests
  - Implements the standard Service interface
  - Returns environment with overwritten connection params via `Env()`
  - Useful for testing services that depend on databases

Test database defaults (works with official postgres Docker image):
```bash
docker run --rm --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres
```

Environment variables:
- `POSTGRES_HOST` (default: "localhost")
- `POSTGRES_PORT` (default: 5432)
- `POSTGRES_DATABASE` (default: "postgres")
- `POSTGRES_USER` (default: "postgres")
- `POSTGRES_PASS` (default: "postgres")

### Dependencies

- **`github.com/emillamm/envx`**: Type-safe environment variable parsing with validation
- **`github.com/emillamm/pgmigrate`**: Database migration tool (used by pgtest)
- **`github.com/jackc/pgx/v5`**: PostgreSQL driver and connection pooling
  - The library uses `pgxpool.Pool` by default (v5 pool API)

### Migration Convention

Tests in `pgtest/` expect a `../migrations` directory relative to the test file location. Migration files are executed in order during ephemeral database creation (e.g., `000.sql`, `001.sql`).

## Code Patterns

### Service Lifecycle Example
```go
ctx := context.Background()
service := http.NewService(os.Getenv)

service.Start(ctx)
service.WaitForReady()

// Use service...

service.Stop()
service.WaitForDone()

if err := service.Err(); err != nil {
    // Handle error
}
```

### Test Database Example
```go
func TestMyCode(t *testing.T) {
    sm, err := pgtest.NewSessionManager(os.Getenv)
    if err != nil {
        t.Fatal(err)
    }
    defer sm.Close()

    sm.Run(t, "test description", func(t *testing.T, conn *pgxpool.Pool) {
        // Test code with isolated database
    })
}
```

### Integration Test with Services
```go
ctx := context.Background()
pgService := pgtest.NewService(os.Getenv)
pgService.Start(ctx)
defer pgService.Stop()
pgService.WaitForReady()

// Use pgService.Env() to get environment with test DB connection params
httpService, _ := http.NewService(pgService.Env())
httpService.Start(ctx)
defer httpService.Stop()
httpService.WaitForReady()

// Both services now running with test database
```
