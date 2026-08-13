package logger

import (
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type spanLogger struct {
	logger       Logger
	span         trace.Span
	spanFields   []zapcore.Field
	level        zapcore.Level
	msg          string
	showTerminal bool
}

func (sl *spanLogger) Debug(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.Debug(template, args...)
	}
	sl.level = zapcore.DebugLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Info(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.Info(template, args...)
	}

	sl.level = zapcore.InfoLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Error(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.Error(template, args...)
	}
	sl.level = zapcore.ErrorLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Fatal(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.Fatal(template, args...)
	}
	sl.level = zapcore.FatalLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Sync() error {
	return sl.logger.Sync()
}

func (sl *spanLogger) DPanic(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.DPanic(template, args...)
	}
	sl.level = zapcore.DPanicLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Panic(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.Panic(template, args...)
	}
	sl.level = zapcore.PanicLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Warn(template string, args ...interface{}) Logger {
	if sl.showTerminal {
		sl.logger.Warn(template, args...)
	}
	sl.level = zapcore.WarnLevel
	sl.msg = template
	return sl
}

func (sl *spanLogger) Level() zapcore.Level {
	return sl.level
}

func (sl *spanLogger) WithShowTerminal(show bool) Logger {
	sl.showTerminal = show
	return sl
}

func (sl *spanLogger) WithFields(fields ...zapcore.Field) Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, zap.Field(field))
	}
	sl.logToSpan(sl.level.String(), sl.msg, zapFields...)
	return &spanLogger{
		logger:     sl.logger,
		span:       sl.span,
		spanFields: zapFields,
	}
}

func (sl spanLogger) logToSpan(level, msg string, fields ...zapcore.Field) {
	fields = append(fields, zap.String("level", level))
	sl.span.AddEvent(
		msg,
		trace.WithAttributes(logFieldsToOTelAttrs(fields)...),
	)
}

func logFieldsToOTelAttrs(fields []zapcore.Field) []attribute.KeyValue {
	encoder := &bridgeFieldEncoder{}
	for _, field := range fields {
		field.AddTo(encoder)
	}
	return encoder.pairs
}

type bridgeFieldEncoder struct {
	pairs []attribute.KeyValue
}

func (e *bridgeFieldEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(marshaler)))
	return nil
}

func (e *bridgeFieldEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(marshaler)))
	return nil
}

func (e *bridgeFieldEncoder) AddBinary(key string, value []byte) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (e *bridgeFieldEncoder) AddByteString(key string, value []byte) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (e *bridgeFieldEncoder) AddBool(key string, value bool) {
	e.pairs = append(e.pairs, attribute.Bool(key, value))
}

func (e *bridgeFieldEncoder) AddComplex128(key string, value complex128) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (e *bridgeFieldEncoder) AddComplex64(key string, value complex64) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (e *bridgeFieldEncoder) AddDuration(key string, value time.Duration) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (e *bridgeFieldEncoder) AddFloat64(key string, value float64) {
	e.pairs = append(e.pairs, attribute.Float64(key, value))
}

func (e *bridgeFieldEncoder) AddFloat32(key string, value float32) {
	e.pairs = append(e.pairs, attribute.Float64(key, float64(value)))
}

func (e *bridgeFieldEncoder) AddInt(key string, value int) {
	e.pairs = append(e.pairs, attribute.Int(key, value))
}

func (e *bridgeFieldEncoder) AddInt64(key string, value int64) {
	e.pairs = append(e.pairs, attribute.Int64(key, value))
}

func (e *bridgeFieldEncoder) AddInt32(key string, value int32) {
	e.pairs = append(e.pairs, attribute.Int64(key, int64(value)))
}

func (e *bridgeFieldEncoder) AddInt16(key string, value int16) {
	e.pairs = append(e.pairs, attribute.Int64(key, int64(value)))
}

func (e *bridgeFieldEncoder) AddInt8(key string, value int8) {
	e.pairs = append(e.pairs, attribute.Int64(key, int64(value)))
}

func (e *bridgeFieldEncoder) AddString(key, value string) {
	e.pairs = append(e.pairs, attribute.String(key, value))
}

func (e *bridgeFieldEncoder) AddTime(key string, value time.Time) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (e *bridgeFieldEncoder) AddUint(key string, value uint) {
	e.pairs = append(e.pairs, attribute.String(key, strconv.FormatUint(uint64(value), 10)))
}

func (e *bridgeFieldEncoder) AddUint64(key string, value uint64) {
	e.pairs = append(e.pairs, attribute.String(key, strconv.FormatUint(value, 10)))
}

func (e *bridgeFieldEncoder) AddUint32(key string, value uint32) {
	e.pairs = append(e.pairs, attribute.Int64(key, int64(value)))
}

func (e *bridgeFieldEncoder) AddUint16(key string, value uint16) {
	e.pairs = append(e.pairs, attribute.Int64(key, int64(value)))
}

func (e *bridgeFieldEncoder) AddUint8(key string, value uint8) {
	e.pairs = append(e.pairs, attribute.Int64(key, int64(value)))
}

func (e *bridgeFieldEncoder) AddUintptr(key string, value uintptr) {
	e.pairs = append(e.pairs, attribute.String(key, fmt.Sprint(value)))
}

func (*bridgeFieldEncoder) AddReflected(string /* key */, any /* value */) error { return nil }
func (*bridgeFieldEncoder) OpenNamespace(string /* key */)                       {}
