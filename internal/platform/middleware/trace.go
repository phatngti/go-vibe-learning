package middleware

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Tracer struct {
	tracer trace.Tracer
}

type spanTraceConfig struct {
	attributes []attribute.KeyValue
	spanKind   trace.SpanKind
	err        *error
}

type SpanTraceOption func(*spanTraceConfig)

func NewTracer(tracer trace.Tracer) Tracer {
	return Tracer{
		tracer: tracer,
	}
}

func (t Tracer) Trace(ctx *context.Context, label string, options ...SpanTraceOption) func() {
	cfg := spanTraceConfig{
		spanKind: trace.SpanKindInternal,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(&cfg)
	}

	startCtx := context.Background()
	if ctx != nil && *ctx != nil {
		startCtx = *ctx
	}

	spanCtx, span := t.tracer.Start(startCtx, label, trace.WithSpanKind(cfg.spanKind))
	if ctx != nil {
		*ctx = spanCtx
	}

	if len(cfg.attributes) > 0 {
		span.SetAttributes(cfg.attributes...)
	}
	return func() {
		if cfg.err != nil && *cfg.err != nil {
			span.RecordError(*cfg.err)
			span.SetStatus(codes.Error, (*cfg.err).Error())
		}
		span.End()
	}
}

func TraceAttributes(attributes ...attribute.KeyValue) SpanTraceOption {
	return func(cfg *spanTraceConfig) {
		cfg.attributes = append(cfg.attributes, attributes...)
	}
}

func TraceSpanKind(spanKind trace.SpanKind) SpanTraceOption {
	return func(cfg *spanTraceConfig) {
		cfg.spanKind = spanKind
	}
}

func TraceError(err *error) SpanTraceOption {
	return func(cfg *spanTraceConfig) {
		cfg.err = err
	}
}
