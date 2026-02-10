# Observability Package

This package provides logging and tracing infrastructure using OpenTelemetry and slog. It supports local development (stdout) and production environments (OTLP export).

## Quick Start

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/emillamm/goext/observability"
)

func main() {
    ctx := context.Background()

    // Load config from environment
    config, err := observability.LoadConfig(os.Getenv)
    if err != nil {
        slog.Error("failed to load observability config", "error", err)
        os.Exit(1)
    }

    // Initialize observability
    provider, err := observability.New(ctx, config)
    if err != nil {
        slog.Error("failed to initialize observability", "error", err)
        os.Exit(1)
    }
    defer provider.Shutdown(ctx)

    // Your application code here
}
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVICE_NAME` | Service name for traces and logs | `unknown-service` |
| `SERVICE_VERSION` | Service version | `0.0.0` |
| `ENVIRONMENT` | Environment type: `prod` or `local` | required |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint (e.g., `localhost:4317` for gRPC, `localhost:4318` for HTTP) | empty (stdout mode) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | OTLP export protocol: `grpc` or `http/protobuf` | `grpc` |
| `OTEL_TRACES_EXPORTER` | Traces exporter: `otlp`, `console`, or `none` | auto-detect (`otlp` if endpoint set, `console` otherwise) |
| `LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` | `debug` (local) / `info` (production) |

Here `SERVICE` is referring to the otel definition of a service which could refer to a job, service, deployment etc. in other contexts.
When `OTEL_TRACES_EXPORTER` is `otlp`, traces are exported via the configured protocol and logs use JSON format. When `console`, traces are printed to stdout and logs use text format. When `none`, tracing is disabled entirely (noop provider) and logs use text format. If `OTEL_TRACES_EXPORTER` is not set, it defaults to `otlp` when an endpoint is configured, or `console` otherwise.

---

## Context Helpers

The package provides helper functions for common tracing operations, defined in `context.go`:

| Function | Description |
|----------|-------------|
| `StartSpan(ctx, tracerName, spanName, opts...)` | Start a new span (combines tracer lookup + start) |
| `SpanFromContext(ctx)` | Get the current span from context |
| `SetSpanError(span, err)` | Record error and set span status to Error |
| `SetSpanOK(span)` | Set span status to OK |
| `AddSpanAttributes(span, attrs...)` | Add attributes to a span |
| `TraceID(ctx)` | Extract trace ID from context (empty string if none) |
| `SpanID(ctx)` | Extract span ID from context (empty string if none) |

Basic usage pattern:

```go
ctx, span := observability.StartSpan(ctx, tracerName, "ProcessPayment",
    trace.WithAttributes(attribute.String("order_id", orderID)),
)
defer span.End()

// ... do work ...

if err != nil {
    observability.SetSpanError(span, err)
    return err
}
observability.SetSpanOK(span)
```

---

## Logging Best Practices

### Use Context-Aware Logging

Always use `slog.*Context` methods to include trace context in logs:

```go
// Good - includes trace_id and span_id automatically
slog.InfoContext(ctx, "processing request", "user_id", userID)
slog.ErrorContext(ctx, "operation failed", "error", err)

// Avoid - loses trace context
slog.Info("processing request", "user_id", userID)
```

### Use Appropriate Log Levels

| Level | Use Case |
|-------|----------|
| `Debug` | Detailed diagnostic information, not needed in production |
| `Info` | Normal operations, significant events |
| `Warn` | Unexpected situations that are handled gracefully |
| `Error` | Failures that need attention |

```go
slog.DebugContext(ctx, "cache lookup", "key", cacheKey)
slog.InfoContext(ctx, "user logged in", "user_id", userID)
slog.WarnContext(ctx, "rate limit approaching", "current", count, "limit", limit)
slog.ErrorContext(ctx, "database query failed", "error", err, "query", queryName)
```

### Structure Your Log Attributes

Use consistent attribute names across your codebase:

```go
// Good - structured, consistent naming
slog.InfoContext(ctx, "notification sent",
    "notification_id", notificationID,
    "recipient", recipient,
    "channel", "email",
    "duration_ms", duration.Milliseconds(),
)

// Avoid - unstructured, inconsistent
slog.InfoContext(ctx, fmt.Sprintf("sent notification %s to %s via email in %v", id, recipient, duration))
```

### Common Attribute Naming Conventions

| Attribute | Description |
|-----------|-------------|
| `error` | Error object or message |
| `user_id` | User identifier |
| `request_id` | Request correlation ID |
| `duration_ms` | Duration in milliseconds |
| `count` | Numeric count |
| `status` | Status code or string |
| `*_id` | Identifiers (e.g., `order_id`, `session_id`) |

### Log at Boundaries

Log at system boundaries (entry/exit points):

```go
func ProcessOrder(ctx context.Context, orderID string) error {
    slog.InfoContext(ctx, "processing order", "order_id", orderID)

    // ... processing logic ...

    if err != nil {
        slog.ErrorContext(ctx, "order processing failed",
            "order_id", orderID,
            "error", err,
        )
        return err
    }

    slog.InfoContext(ctx, "order processed successfully", "order_id", orderID)
    return nil
}
```

### Don't Log Sensitive Data

Never log passwords, tokens, PII, or other sensitive information:

```go
// Good
slog.InfoContext(ctx, "user authenticated", "user_id", userID)

// Bad - leaks sensitive data
slog.InfoContext(ctx, "user authenticated", "password", password, "token", token)
```

---

## Tracing Best Practices

### Create Spans for Significant Operations

Create spans for operations that:
- Cross service boundaries (RPC calls, HTTP requests)
- Access external systems (databases, caches, message queues)
- Perform significant business logic

```go
import (
    "github.com/emillamm/goext/observability"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

const tracerName = "myservice/mypackage"

func ProcessPayment(ctx context.Context, orderID string, amount float64) error {
    ctx, span := observability.StartSpan(ctx, tracerName, "ProcessPayment",
        trace.WithAttributes(
            attribute.String("order_id", orderID),
            attribute.Float64("amount", amount),
        ),
    )
    defer span.End()

    // ... payment logic ...

    if err != nil {
        observability.SetSpanError(span, err)
        return err
    }

    observability.SetSpanOK(span)
    return nil
}
```

### Tracer and Span Naming Convention

**Tracer name**: Use the full package path (e.g., `myservice/app/payments`). This identifies the instrumentation scope.

**Span name**: Use just the operation/function name (e.g., `ProcessPayment`). Don't repeat the package name since the tracer already provides that context.

```go
const tracerName = "myservice/app/payments"

func ProcessPayment(ctx context.Context, orderID string) error {
    // Good - operation name only, tracer provides package context
    ctx, span := observability.StartSpan(ctx, tracerName, "ProcessPayment")

    // Avoid - redundant package prefix
    ctx, span := observability.StartSpan(ctx, tracerName, "payments.ProcessPayment")
}
```

This produces clean traces:
```
Tracer: myservice/app/payments
  Span: ProcessPayment
```

### Avoid Variable Data in Span Names

Span names should be static. Use attributes for variable data:

```go
// Good - static name, variable data in attributes
observability.StartSpan(ctx, tracerName, "ProcessOrder",
    trace.WithAttributes(attribute.String("order_id", orderID)),
)

// Bad - includes variable data (causes high cardinality)
observability.StartSpan(ctx, tracerName, fmt.Sprintf("ProcessOrder-%s", orderID))
observability.StartSpan(ctx, tracerName, "SendEmail to " + email)
```

### Add Relevant Attributes

Add attributes that help with debugging and analysis:

```go
observability.AddSpanAttributes(span,
    attribute.String("order_id", orderID),
    attribute.String("customer_id", customerID),
    attribute.Int("item_count", len(items)),
    attribute.String("payment_method", paymentMethod),
)
```

### Record Errors Properly

Always record errors and set span status:

```go
if err != nil {
    observability.SetSpanError(span, err)  // Records error + sets span status
    return err
}

observability.SetSpanOK(span)  // Mark successful completion
```

### Propagate Context

Always pass context through your call chain to maintain trace continuity:

```go
func Handler(ctx context.Context, req Request) error {
    // Context flows through all calls
    user, err := fetchUser(ctx, req.UserID)
    if err != nil {
        return err
    }

    return processUser(ctx, user)
}
```

### Use Appropriate Span Kinds

Set the correct span kind for your operation:

```go
// Server - handling incoming requests
trace.WithSpanKind(trace.SpanKindServer)

// Client - making outgoing requests
trace.WithSpanKind(trace.SpanKindClient)

// Producer - sending messages to a queue
trace.WithSpanKind(trace.SpanKindProducer)

// Consumer - receiving messages from a queue
trace.WithSpanKind(trace.SpanKindConsumer)

// Internal - internal operations (default)
trace.WithSpanKind(trace.SpanKindInternal)
```

---

## Combining Logging and Tracing

### Log Within Spans

Logs emitted within a span automatically include trace context:

```go
func ProcessOrder(ctx context.Context, orderID string) error {
    ctx, span := observability.StartSpan(ctx, tracerName, "ProcessOrder")
    defer span.End()

    // This log will include trace_id and span_id
    slog.InfoContext(ctx, "validating order", "order_id", orderID)

    if err := validate(ctx, orderID); err != nil {
        slog.ErrorContext(ctx, "validation failed", "error", err)
        observability.SetSpanError(span, err)
        return err
    }

    slog.InfoContext(ctx, "order validated successfully")
    observability.SetSpanOK(span)
    return nil
}
```

### Use Logs for Details, Spans for Structure

- **Spans**: Capture the structure and timing of operations
- **Logs**: Capture detailed information within operations

```go
func SendNotification(ctx context.Context, userID, message string) error {
    ctx, span := observability.StartSpan(ctx, tracerName, "SendNotification",
        trace.WithAttributes(attribute.String("user_id", userID)),
    )
    defer span.End()

    // Log detailed steps
    slog.DebugContext(ctx, "looking up user preferences")
    prefs, err := getUserPrefs(ctx, userID)
    if err != nil {
        slog.WarnContext(ctx, "failed to get preferences, using defaults", "error", err)
        prefs = defaultPrefs
    }

    slog.DebugContext(ctx, "sending via preferred channel", "channel", prefs.Channel)

    // ... send logic ...

    slog.InfoContext(ctx, "notification sent", "channel", prefs.Channel)
    return nil
}
```

---

## Kafka Tracing (with mika)

When using Kafka with the mika library, use the tracing functions provided by mika.

> **Note:** Use `mika.SetSpanError()` and `mika.SetSpanOK()` for Kafka consumer/producer code. The mika library provides its own span helpers to remain independent of goext.

### Producer Side

```go
import "github.com/emillamm/mika"

func publishMessage(ctx context.Context, kafka *mika.KafkaClient, data []byte) error {
    record := &kgo.Record{
        Topic: "my-topic",
        Value: data,
    }

    // Inject trace context into Kafka headers
    mika.InjectTraceContext(ctx, record)

    return kafka.PublishRecord(ctx, "my-topic", record)
}
```

### Consumer Side

```go
func (c *MyConsumer) Consume(record *mika.ConsumeRecord) {
    // Extract trace context and start a consumer span
    ctx, span := mika.StartConsumeSpan(context.Background(), record.Underlying)
    defer span.End()

    // Process with traced context
    if err := c.process(ctx, record); err != nil {
        mika.SetSpanError(span, err)
        record.Fail(err)
        return
    }

    mika.SetSpanOK(span)
    record.Ack()
}
```

---

## PostgreSQL Tracing (with otelpgx)

Configure pgxpool with OpenTelemetry tracing:

```go
import "github.com/exaring/otelpgx"

func setupDatabase(ctx context.Context, connString string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, err
    }

    // Add OpenTelemetry tracer
    config.ConnConfig.Tracer = otelpgx.NewTracer()

    return pgxpool.NewWithConfig(ctx, config)
}
```

All database queries will now be automatically traced.

---

## ConnectRPC Tracing (with otelconnect)

Add the otelconnect interceptor to your RPC handlers:

```go
import "connectrpc.com/otelconnect"

func setupRPC(mux *http.ServeMux) error {
    otelInterceptor, err := otelconnect.NewInterceptor()
    if err != nil {
        return err
    }

    mux.Handle(myv1connect.NewMyServiceHandler(
        &MyService{},
        connect.WithInterceptors(otelInterceptor),
    ))

    return nil
}
```

### What otelconnect does automatically

The interceptor handles tracing at the RPC endpoint level -- **you do not need to create spans manually for each handler**. It automatically:

- Creates a **server span** for every incoming RPC call
- Sets the span name to the full RPC method (e.g., `myservice.v1.MyService/GetUser`)
- Records errors and sets span status
- Extracts trace context from incoming request headers (distributed trace propagation)
- Sets span kind to `Server`

### When to add child spans in handlers

Create child spans inside your handler only for significant sub-operations:

```go
const tracerName = "myservice/orders"

func (s *OrderService) ProcessOrder(
    ctx context.Context,
    req *connect.Request[ordersv1.ProcessOrderRequest],
) (*connect.Response[ordersv1.ProcessOrderResponse], error) {
    // otelconnect already created a server span for this RPC.
    // Create child spans for significant sub-operations:
    ctx, span := observability.StartSpan(ctx, tracerName, "ValidateInventory")
    err := s.validateInventory(ctx, req.Msg.OrderId)
    if err != nil {
        observability.SetSpanError(span, err)
        span.End()
        return nil, err
    }
    observability.SetSpanOK(span)
    span.End()

    // Database queries are traced automatically if using otelpgx.
    // External service calls should get their own spans.

    return connect.NewResponse(&ordersv1.ProcessOrderResponse{}), nil
}
```

You typically do **not** need child spans for:
- Database queries (already traced by `otelpgx`)
- Simple field lookups or validation that doesn't call external systems

---

## Testing

In tests, you can skip observability initialization or use a no-op configuration:

```go
import "github.com/emillamm/goext/environment"

func TestMyFunction(t *testing.T) {
    ctx := context.Background()

    // Option 1: Skip initialization entirely
    // Logs go to default slog handler, traces are no-ops

    // Option 2: Initialize with test config
    config := observability.Config{
        ServiceName: "test-service",
        Environment: environment.Local,
        // Empty OTLP endpoint = stdout mode
    }
    provider, _ := observability.New(ctx, config)
    defer provider.Shutdown(ctx)

    // Your test code
}
```

---

## Checklist

Before deploying, verify:

- [ ] All significant operations have spans
- [ ] Context is propagated through all function calls
- [ ] Errors are recorded on spans with `SetSpanError` (and `SetSpanOK` for success)
- [ ] Logs use `*Context` methods for trace correlation
- [ ] No sensitive data in logs or span attributes
- [ ] Appropriate log levels are used
- [ ] Tracer names use full package path (e.g., `myservice/app/payments`)
- [ ] Span names are operation only, no package prefix (e.g., `ProcessPayment` not `payments.ProcessPayment`)
- [ ] Span names don't include variable data (use attributes instead)
- [ ] OTLP endpoint and protocol are configured for non-local environments
