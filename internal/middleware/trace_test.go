package middleware

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTracerTraceAppliesOptionsAndRecordsError(t *testing.T) {
	exporter, tracer, shutdown := testTracer(t)
	defer shutdown()

	ctx := context.Background()
	traceErr := errors.New("auth failed")
	hookCalled := false

	endTrace := NewTracer(tracer).Trace(&ctx, "Login",
		TraceAttributes(attribute.String("username", "alice")),
		TraceHooks(func(ctx context.Context, span trace.Span) {
			hookCalled = true
			if !span.SpanContext().IsValid() {
				t.Fatal("expected hook to receive a valid span")
			}
			if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
				t.Fatal("expected hook to receive the span context")
			}
			span.SetAttributes(attribute.String("hook", "called"))
		}),
		TraceError(&traceErr),
	)

	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		t.Fatal("expected Trace to update the caller context")
	}
	if !hookCalled {
		t.Fatal("expected trace hook to run")
	}

	endTrace()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "Login" {
		t.Fatalf("expected span name Login, got %q", spans[0].Name)
	}
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("expected error status, got %s", spans[0].Status.Code)
	}

	attrs := attributesByKey(spans[0].Attributes)
	assertAttr(t, attrs, "username", "alice")
	assertAttr(t, attrs, "hook", "called")
}

func testTracer(t *testing.T) (*tracetest.InMemoryExporter, trace.Tracer, func()) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	return exporter, provider.Tracer("trace-test"), func() {
		if err := provider.Shutdown(t.Context()); err != nil {
			t.Fatalf("failed to shut down tracer provider: %v", err)
		}
	}
}

func attributesByKey(attrs []attribute.KeyValue) map[string]string {
	values := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		values[string(attr.Key)] = attr.Value.Emit()
	}
	return values
}

func assertAttr(t *testing.T, attrs map[string]string, key string, want string) {
	t.Helper()

	if got, ok := attrs[key]; !ok || got != want {
		t.Fatalf("expected attribute %s=%q, got %q (exists=%t)", key, want, got, ok)
	}
}
