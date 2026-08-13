package logger

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github/master_go_cqrs/internal/platform/config"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	WithFields(fields ...zapcore.Field) Logger
	WithShowTerminal(show bool) Logger
	Level() zapcore.Level
	Debug(template string, args ...interface{}) Logger
	Info(template string, args ...interface{}) Logger
	Warn(template string, args ...interface{}) Logger
	Error(template string, args ...interface{}) Logger
	DPanic(template string, args ...interface{}) Logger
	Panic(template string, args ...interface{}) Logger
	Fatal(template string, args ...interface{}) Logger
	Sync() error
}

// For mapping config logger to app logger levels
var loggerLevelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

// Logger
type apiLogger struct {
	config *config.Logger
	logger *zap.Logger
}

// App Logger constructor
func NewapiLogger(cfg *config.Logger) Logger {

	logger := &apiLogger{config: cfg}
	logger.InitLogger()

	return logger
}

func (l *apiLogger) getLoggerLevel(cfg *config.Logger) zapcore.Level {
	level, exist := loggerLevelMap[cfg.LogLevel]
	if !exist {
		return zapcore.DebugLevel
	}

	return level
}

const (
	colorRed     = "\x1b[31m"
	colorGreen   = "\x1b[32m"
	colorYellow  = "\x1b[33m"
	colorGrey    = "\x1b[90m" // Using bright black (grey) for debug
	colorReset   = "\x1b[0m"
	colorDarkRed = "\x1b[31;2m"
	colorViolet  = "\x1b[35m"
	colorOrange  = "\x1b[38;5;208m"
	colorBlue    = "\x1b[34m"
)

// Create a custom encoder that wraps the console encoder
type customEncoder struct {
	zapcore.Encoder
}

func (e *customEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// Color the message based on level
	switch entry.Level {
	case zapcore.DebugLevel:
		entry.Message = colorViolet + entry.Message + colorReset
	case zapcore.InfoLevel:
		entry.Message = colorBlue + entry.Message + colorReset
	case zapcore.WarnLevel:
		entry.Message = colorYellow + entry.Message + colorReset
	case zapcore.ErrorLevel:
		entry.Message = colorOrange + entry.Message + colorReset
	case zapcore.DPanicLevel, zapcore.PanicLevel:
		entry.Message = colorRed + entry.Message + colorReset
	case zapcore.FatalLevel:
		entry.Message = colorDarkRed + entry.Message + colorReset
	}

	return e.Encoder.EncodeEntry(entry, fields)
}

func customEncodeLevel(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch level {
	case zapcore.DebugLevel:
		enc.AppendString(colorViolet + "DEBUG" + colorReset)
	case zapcore.InfoLevel:
		enc.AppendString(colorBlue + "INFO" + colorReset)
	case zapcore.WarnLevel:
		enc.AppendString(colorYellow + "WARN" + colorReset)
	case zapcore.ErrorLevel:
		enc.AppendString(colorOrange + "ERROR" + colorReset)
	case zapcore.DPanicLevel:
		enc.AppendString(colorRed + "DPANIC" + colorReset)
	case zapcore.PanicLevel:
		enc.AppendString(colorRed + "PANIC" + colorReset)
	case zapcore.FatalLevel:
		enc.AppendString(colorDarkRed + "FATAL" + colorReset)
	default:
		enc.AppendString(level.String())
	}
}

// Init logger
func (l *apiLogger) InitLogger() {
	logLevel := l.getLoggerLevel(l.config)

	logWriter := zapcore.AddSync(os.Stderr)

	var encoderCfg zapcore.EncoderConfig
	if l.config.Mode == "development" {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderCfg = zap.NewProductionEncoderConfig()
	}

	encoderCfg.LevelKey = "LEVEL"
	encoderCfg.CallerKey = "CALLER"
	encoderCfg.TimeKey = "TIME"
	encoderCfg.NameKey = "NAME"
	encoderCfg.MessageKey = "MESSAGE"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = customEncodeLevel

	var encoder zapcore.Encoder
	if l.config.LogEncoding == "console" {
		baseEncoder := zapcore.NewConsoleEncoder(encoderCfg)
		encoder = &customEncoder{baseEncoder}
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, logWriter, zap.NewAtomicLevelAt(logLevel))
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	l.logger = logger

}

// Logger methods

func (l *apiLogger) Debug(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.Debug(msg, fields...)
	return l
}

func (l *apiLogger) Info(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.Info(msg, fields...)
	return l
}

func (l *apiLogger) Warn(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.Warn(msg, fields...)
	return l
}

func (l *apiLogger) Error(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.Error(msg, fields...)
	return l
}

func (l *apiLogger) DPanic(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.DPanic(msg, fields...)
	return l
}

func (l *apiLogger) Panic(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.Panic(msg, fields...)
	return l
}

func (l *apiLogger) Fatal(template string, args ...interface{}) Logger {
	msg, fields := l.getMessageAndFields(template, args)
	l.logger.Fatal(msg, fields...)
	return l
}

func (l *apiLogger) Sync() error {
	err := l.logger.Sync()
	if isIgnorableSyncError(err) {
		return nil
	}
	return err
}

func (l *apiLogger) Level() zapcore.Level {
	return l.logger.Level()
}

func (l *apiLogger) WithShowTerminal(show bool) Logger {
	return l
}

func (l *apiLogger) getMessageAndFields(template string, args []interface{}) (string, []zapcore.Field) {
	fmtArgs := make([]interface{}, 0, len(args))
	fields := make([]zapcore.Field, 0, len(args))
	for _, arg := range args {
		if field, ok := arg.(zapcore.Field); ok {
			fields = append(fields, field)
			continue
		}
		fmtArgs = append(fmtArgs, arg)
	}

	if len(fmtArgs) == 0 {
		return template, fields
	}

	if template != "" {
		return fmt.Sprintf(template, fmtArgs...), fields
	}

	if len(fmtArgs) == 1 {
		if str, ok := fmtArgs[0].(string); ok {
			return str, fields
		}
	}
	return fmt.Sprint(fmtArgs...), fields
}

func isIgnorableSyncError(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY)
}

func (l *apiLogger) WithFields(fields ...zapcore.Field) Logger {
	zapFields := make([]zapcore.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, zapcore.Field(field))
	}

	return &apiLogger{
		config: l.config,
		logger: l.logger.With(zapFields...),
	}
}
