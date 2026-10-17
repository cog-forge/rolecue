package logger

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger
}

type Config struct {
	Level      string
	Format     string // json or console
	Output     string // stdout or file path
	TimeFormat string
}

type contextKey string

const (
	LoggerKey    contextKey = "logger"
	RequestIDKey contextKey = "request_id"
	TraceIDKey   contextKey = "trace_id"
)

func New(cfg Config) (*Logger, error) {
	level := parseLevel(cfg.Level)

	timeEncoder := zapcore.ISO8601TimeEncoder
	if cfg.TimeFormat != "" {
		timeEncoder = zapcore.TimeEncoderOfLayout(cfg.TimeFormat)
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	var writer zapcore.WriteSyncer
	if cfg.Output == "stdout" || cfg.Output == "" {
		writer = zapcore.AddSync(os.Stdout)
	} else {
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writer = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, writer, level)
	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{Logger: l}, nil
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := []zap.Field{}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		fields = append(fields, zap.String("request_id", reqID))
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if len(fields) == 0 {
		return l
	}
	return &Logger{Logger: l.With(fields...)}
}

func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{Logger: l.With(zap.Any(key, value))}
}

func (l *Logger) WithFields(fields map[string]any) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return &Logger{Logger: l.With(zapFields...)}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{Logger: l.With(zap.Error(err))}
}

func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(LoggerKey).(*Logger); ok {
		return l
	}
	return &Logger{Logger: zap.NewNop()}
}

func ToContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

func String(key, val string) zap.Field             { return zap.String(key, val) }
func Int(key string, val int) zap.Field                { return zap.Int(key, val) }
func Duration(key string, val time.Duration) zap.Field { return zap.Duration(key, val) }
func Any(key string, val any) zap.Field                { return zap.Any(key, val) }
func Error(err error) zap.Field                       { return zap.Error(err) }
