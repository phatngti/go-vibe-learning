package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Factory struct {
	Logger Logger
}

func NewFactory(logger Logger) Factory {
	return Factory{
		Logger: logger,
	}
}

// Bg creates a context-unaware logger.

func (f Factory) Bg() Logger {
	return f.Logger
}

func (f Factory) For(ctx context.Context) Logger {
	if span := trace.SpanFromContext(ctx); span != nil {
		logger := &spanLogger{span: span, logger: f.Logger}
		logger.spanFields = []zapcore.Field{
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		}
		return logger
	}

	return f.Bg()
}
