package logger

import (
	"context"
	"errors"
	"regexp"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
)

var defaultLogger atomic.Pointer[Logger]

// SetDefault installs the process logger that FromContext falls back to when a context carries none.
func SetDefault(l *Logger) { defaultLogger.Store(l) }

// WithRequestID stores the request ID on ctx, where Logger.WithContext picks it up.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// RequestID returns the request ID stored on ctx, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey).(string)
	return id
}

const (
	maxCauseLength = 400
	maxCauseDepth  = 8
)

// A URL keeps its path but loses its query string, where presigned signatures and tokens live.
var urlQueryPattern = regexp.MustCompile(`(https?://[^\s"'?]+)\?[^\s"']*`)

var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+`)

var secretPattern = regexp.MustCompile(`(?i)(key|token|secret|signature|password|credential|x-amz-[a-z-]+|authorization)(=|:\s*|%3d)(?:bearer\s+|basic\s+)?[^&\s"',}]+|bearer\s+\S+`)

// Sanitize removes credential-looking values from text and bounds its length, so an error
// message can be logged without leaking secrets from URLs or headers.
func Sanitize(text string) string {
	text = urlQueryPattern.ReplaceAllString(text, "${1}?[redacted]")
	text = secretPattern.ReplaceAllString(text, "[redacted]")
	text = emailPattern.ReplaceAllString(text, "[email]")
	if len(text) > maxCauseLength {
		// Cut on a rune boundary: the app handles Vietnamese text, and a mid-rune cut is invalid UTF-8.
		cut := maxCauseLength
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut] + "…"
	}
	return text
}

// ErrorChain returns the sanitized messages of err and everything it wraps, outermost first.
// It is the root-cause trail that a bare error field would flatten into one line.
func ErrorChain(err error) []string {
	var chain []string
	for depth := 0; err != nil && depth < maxCauseDepth; depth++ {
		chain = append(chain, Sanitize(err.Error()))
		next := errors.Unwrap(err)
		if next == nil {
			if joined, ok := err.(interface{ Unwrap() []error }); ok {
				for _, e := range joined.Unwrap() {
					chain = append(chain, ErrorChain(e)...)
				}
			}
		}
		err = next
	}
	return chain
}

// Cause is the structured field for a failure: the sanitized message plus the wrapped chain.
func Cause(err error) zap.Field {
	if err == nil {
		return zap.Skip()
	}
	return zap.Strings("cause_chain", ErrorChain(err))
}

// Code attaches an application error code.
func Code(code string) zap.Field { return zap.String("error_code", code) }

// Op names the operation a log line belongs to, e.g. "roleprofile.analyze".
func Op(name string) zap.Field { return zap.String("op", name) }

// Step records one operation. Call Start before the work and the returned function once with the
// outcome:
//
//	done := logger.Start(ctx, "roleprofile.extract", logger.String("profile_id", id))
//	result, err := extract(ctx)
//	done(err, logger.Int("requirements", len(result)))
//
// Success logs at debug; a failure logs at warn with the sanitized cause chain and duration, so a
// 5xx can be traced to the step that produced it. Never pass document text or secrets as fields.
func Start(ctx context.Context, op string, fields ...zap.Field) func(err error, extra ...zap.Field) {
	begin := time.Now()
	log := FromContext(ctx).WithOptions(zap.AddCallerSkip(1))
	base := append([]zap.Field{Op(op)}, fields...)
	return func(err error, extra ...zap.Field) {
		all := make([]zap.Field, 0, len(base)+len(extra)+1)
		all = append(all, base...)
		all = append(all, zap.Duration("duration", time.Since(begin)))
		all = append(all, extra...)
		if err != nil {
			log.Warn("step failed", append(all, Cause(err))...)
			return
		}
		log.Debug("step ok", all...)
	}
}

// WithOptions returns a logger with extra zap options, keeping the wrapper type.
func (l *Logger) WithOptions(opts ...zap.Option) *Logger {
	return &Logger{Logger: l.Logger.WithOptions(opts...)}
}
