package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan starts a new span with the given name and options.
// It returns a new context containing the span and the span itself.
// The caller is responsible for calling span.End() when done.
func StartSpan(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, spanName, opts...)
}

// SpanFromContext returns the current span from the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// SetSpanError marks the span as having an error and records the error.
func SetSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanOK marks the span as successful.
func SetSpanOK(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}

// AddSpanAttributes adds attributes to the current span.
func AddSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// LogWithSpan logs a message with trace context from the span.
func LogWithSpan(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		attrs = append(attrs,
			"trace_id", span.SpanContext().TraceID().String(),
			"span_id", span.SpanContext().SpanID().String(),
		)
	}
	slog.Log(ctx, level, msg, attrs...)
}

// LogInfo logs an info message with trace context.
func LogInfo(ctx context.Context, msg string, attrs ...any) {
	LogWithSpan(ctx, slog.LevelInfo, msg, attrs...)
}

// LogDebug logs a debug message with trace context.
func LogDebug(ctx context.Context, msg string, attrs ...any) {
	LogWithSpan(ctx, slog.LevelDebug, msg, attrs...)
}

// LogWarn logs a warning message with trace context.
func LogWarn(ctx context.Context, msg string, attrs ...any) {
	LogWithSpan(ctx, slog.LevelWarn, msg, attrs...)
}

// LogError logs an error message with trace context.
func LogError(ctx context.Context, msg string, err error, attrs ...any) {
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	LogWithSpan(ctx, slog.LevelError, msg, attrs...)
}

// TraceID returns the trace ID from the context, or empty string if not present.
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the span ID from the context, or empty string if not present.
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// WithTraceAttrs returns a logger with trace context attributes added.
func WithTraceAttrs(ctx context.Context, logger *slog.Logger) *slog.Logger {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return logger.With(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return logger
}
