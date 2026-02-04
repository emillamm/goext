// Package observability provides logging and tracing infrastructure using
// OpenTelemetry and slog. It supports local development (stdout) and production
// environments (OTLP export to collectors like Grafana Cloud).
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/emillamm/envx"
	"github.com/emillamm/goext/environment"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for observability setup.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    environment.Environment
	OTLPEndpoint   string // OTLP collector endpoint (e.g., "localhost:4317" for gRPC, "localhost:4318" for HTTP)
	OTLPProtocol   string // OTLP protocol: "grpc" or "http/protobuf" (default: "grpc")
	LogLevel       string // "debug", "info", "warn", "error" (optional, defaults based on environment)
}

// Provider wraps the trace provider and logger for cleanup.
type Provider struct {
	TracerProvider *sdktrace.TracerProvider
	Logger         *slog.Logger
	config         Config
}

// LoadConfig loads observability configuration from environment variables.
// Environment variables:
//   - SERVICE_NAME: Service name (default: "unknown-service")
//   - SERVICE_VERSION: Service version (default: "0.0.0")
//   - ENVIRONMENT: Environment type, "prod" or "local" (required, loaded via environment package)
//   - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP collector endpoint (default: "", uses stdout tracing when empty)
//   - OTEL_EXPORTER_OTLP_PROTOCOL: OTLP export protocol, "grpc" or "http/protobuf" (default: "grpc")
//   - LOG_LEVEL: Log level "debug", "info", "warn", "error" (optional, defaults based on environment)
func LoadConfig(env envx.EnvX) (Config, error) {
	checks := envx.NewChecks()

	serviceName := envx.Check(env.String("SERVICE_NAME").Default("unknown-service"))(checks)
	serviceVersion := envx.Check(env.String("SERVICE_VERSION").Default("0.0.0"))(checks)
	otlpEndpoint := envx.Check(env.String("OTEL_EXPORTER_OTLP_ENDPOINT").Default(""))(checks)
	otlpProtocol := envx.Check(env.String("OTEL_EXPORTER_OTLP_PROTOCOL").Default("grpc"))(checks)
	logLevel := envx.Check(env.String("LOG_LEVEL").Default(""))(checks)

	if err := checks.Err(); err != nil {
		return Config{}, err
	}

	// Validate protocol value
	switch otlpProtocol {
	case "grpc", "http/protobuf":
		// valid
	default:
		return Config{}, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_PROTOCOL %q: must be \"grpc\" or \"http/protobuf\"", otlpProtocol)
	}

	// Load environment using the environment package (will panic if invalid)
	environ := environment.LoadEnvironment(env)

	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Environment:    environ,
		OTLPEndpoint:   otlpEndpoint,
		OTLPProtocol:   otlpProtocol,
		LogLevel:       logLevel,
	}, nil
}

// parseLogLevel converts a string log level to slog.Level.
// Returns the parsed level and true if valid, or the default level and false if invalid/empty.
func parseLogLevel(level string, defaultLevel slog.Level) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	case "":
		return defaultLevel, false
	default:
		return defaultLevel, false
	}
}

// New creates a new observability provider with tracing and logging configured.
// In local mode (no OTLP endpoint), traces are printed to stdout.
// In production mode, traces are exported via OTLP to the configured endpoint.
func New(ctx context.Context, config Config) (*Provider, error) {
	// Create resource with service information
	// Note: We don't use resource.Merge with resource.Default() to avoid schema version conflicts
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(config.ServiceVersion),
		semconv.DeploymentEnvironment(config.Environment.String()),
		semconv.TelemetrySDKLanguageGo,
		semconv.TelemetrySDKName("opentelemetry"),
	)

	// Create trace exporter based on environment
	var exporter sdktrace.SpanExporter
	var err error
	if config.OTLPEndpoint != "" {
		// Production mode: export to OTLP collector
		switch config.OTLPProtocol {
		case "http/protobuf":
			exporter, err = otlptracehttp.New(ctx,
				otlptracehttp.WithEndpoint(config.OTLPEndpoint),
				otlptracehttp.WithInsecure(), // TLS handled by service mesh/ingress
			)
		default: // "grpc"
			exporter, err = otlptracegrpc.New(ctx,
				otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
				otlptracegrpc.WithInsecure(), // TLS handled by service mesh/ingress
			)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
		}
	} else {
		// Local mode: print to stdout
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global trace provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Set error handler for export failures (e.g., collector unreachable)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("opentelemetry error", "error", err)
	}))

	// Determine log level: use configured value or default based on environment
	var logLevel slog.Level
	var handler slog.Handler
	if config.OTLPEndpoint != "" {
		// Production: JSON output for log aggregation, default to info
		logLevel, _ = parseLogLevel(config.LogLevel, slog.LevelInfo)
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
		})
	} else {
		// Local: human-readable text output, default to debug
		logLevel, _ = parseLogLevel(config.LogLevel, slog.LevelDebug)
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
		})
	}

	// Wrap handler with trace context injection
	handler = &traceContextHandler{Handler: handler}

	logger := slog.New(handler).With(
		slog.String("service", config.ServiceName),
		slog.String("version", config.ServiceVersion),
		slog.String("env", config.Environment.String()),
	)

	// Set as default logger
	slog.SetDefault(logger)

	return &Provider{
		TracerProvider: tp,
		Logger:         logger,
		config:         config,
	}, nil
}

// Shutdown gracefully shuts down the trace provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.TracerProvider != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return p.TracerProvider.Shutdown(shutdownCtx)
	}
	return nil
}

// Tracer returns a tracer for the given name.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.TracerProvider.Tracer(name)
}

// ServiceTracer returns a tracer for the configured service.
func (p *Provider) ServiceTracer() trace.Tracer {
	return p.Tracer(p.config.ServiceName)
}

// traceContextHandler wraps an slog.Handler to inject trace context.
type traceContextHandler struct {
	slog.Handler
}

func (h *traceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceContextHandler) WithGroup(name string) slog.Handler {
	return &traceContextHandler{Handler: h.Handler.WithGroup(name)}
}
