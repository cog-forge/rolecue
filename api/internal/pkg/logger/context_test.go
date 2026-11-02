package logger_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// newObserved returns a logger that records entries in memory, with caller reporting enabled
// so tests can check which source line a log call came from.
func newObserved(level zapcore.Level) (*logger.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return &logger.Logger{Logger: zap.New(core, zap.AddCaller())}, logs
}

// setDefault installs l as the process default and clears it when the test ends. There is no getter
// for the previous default, so the baseline is nil, which is what a fresh process starts with.
func setDefault(t *testing.T, l *logger.Logger) {
	t.Helper()
	logger.SetDefault(l)
	t.Cleanup(func() { logger.SetDefault(nil) })
}

func fieldsOf(t *testing.T, logs *observer.ObservedLogs, msg string) observer.LoggedEntry {
	t.Helper()
	found := logs.FilterMessage(msg).All()
	if len(found) != 1 {
		t.Fatalf("want exactly one %q entry, got %d (all: %v)", msg, len(found), logs.AllUntimed())
	}
	return found[0]
}

func TestSanitize(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"plain text untouched", "connection refused", "connection refused"},
		{"key value redacted", "dial tcp: key=abc123 host=x", "dial tcp: [redacted] host=x"},
		{"token stops at ampersand", "GET /cb?token=sekret&state=ok", "GET /cb?[redacted]&state=ok"},
		{"password with colon", "password: hunter2", "[redacted]"},
		{"bearer token", "Bearer tok123 and more", "[redacted] and more"},
		{"aws signature redacted", "https://s3/x?X-Amz-Signature=deadbeef&a=1", "https://s3/x?[redacted]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := logger.Sanitize(tc.in); got != tc.want {
				t.Fatalf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Authorization headers carry a scheme and a credential. Both must be gone, not only the scheme.
// This test is expected to FAIL on the current code: the authorization alternative consumes the
// word "Bearer" as its value and the bearer alternative never runs, so the token survives.
func TestSanitize_AuthorizationHeaderHidesCredential(t *testing.T) {
	got := logger.Sanitize("Authorization: Bearer eyJhbGciOi.secret.value")
	if strings.Contains(got, "eyJhbGciOi") || strings.Contains(got, "secret.value") {
		t.Fatalf("credential leaked: %q", got)
	}
}

func TestSanitize_TruncatesLongText(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := logger.Sanitize(long)
	if want := strings.Repeat("a", 400) + "…"; got != want {
		t.Fatalf("truncated length %d, want %d bytes with ellipsis", len(got), len(want))
	}

	exact := strings.Repeat("a", 400)
	if got := logger.Sanitize(exact); got != exact {
		t.Fatalf("400-byte input changed: len %d", len(got))
	}
}

func TestErrorChain(t *testing.T) {
	t.Run("nil has no chain", func(t *testing.T) {
		if got := logger.ErrorChain(nil); len(got) != 0 {
			t.Fatalf("ErrorChain(nil) = %v, want empty", got)
		}
	})

	t.Run("nested %w outermost first", func(t *testing.T) {
		root := errors.New("root")
		err := fmt.Errorf("outer: %w", fmt.Errorf("middle: %w", root))
		want := []string{"outer: middle: root", "middle: root", "root"}
		assertChain(t, logger.ErrorChain(err), want)
	})

	t.Run("errors.Join lists every branch", func(t *testing.T) {
		err := errors.Join(errors.New("a"), errors.New("b"))
		want := []string{"a\nb", "a", "b"}
		assertChain(t, logger.ErrorChain(err), want)
	})

	t.Run("sanitizes each link", func(t *testing.T) {
		err := fmt.Errorf("wrap: %w", errors.New("dial token=secret"))
		chain := logger.ErrorChain(err)
		if len(chain) != 2 || chain[1] != "dial [redacted]" {
			t.Fatalf("chain = %v, want second link sanitized", chain)
		}
	})

	t.Run("depth is capped at eight", func(t *testing.T) {
		err := errors.New("bottom")
		for i := 0; i < 20; i++ {
			err = fmt.Errorf("layer %d: %w", i, err)
		}
		if got := logger.ErrorChain(err); len(got) != 8 {
			t.Fatalf("chain length %d, want 8", len(got))
		}
	})
}

func assertChain(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("chain = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chain[%d] = %q, want %q (full %q)", i, got[i], want[i], got)
		}
	}
}

func TestCause(t *testing.T) {
	t.Run("nil is a skipped field", func(t *testing.T) {
		if f := logger.Cause(nil); f.Type != zapcore.SkipType {
			t.Fatalf("Cause(nil).Type = %v, want SkipType", f.Type)
		}
		log, logs := newObserved(zapcore.DebugLevel)
		log.Info("no error", logger.Cause(nil))
		// The observer keeps the skip field itself, but encoding drops it: the rendered entry has no fields.
		if m := logs.All()[0].ContextMap(); len(m) != 0 {
			t.Fatalf("expected no rendered fields, got %v", m)
		}
	})

	t.Run("error becomes cause_chain", func(t *testing.T) {
		log, logs := newObserved(zapcore.DebugLevel)
		log.Info("failed", logger.Cause(fmt.Errorf("load: %w", errors.New("db down"))))
		m := logs.All()[0].ContextMap()
		if got := fmt.Sprint(m["cause_chain"]); got != "[load: db down db down]" {
			t.Fatalf("cause_chain = %v", m["cause_chain"])
		}
	})
}

func TestStart_Success(t *testing.T) {
	log, logs := newObserved(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), log)

	done := logger.Start(ctx, "roleprofile.extract", logger.String("profile_id", "p-1"))
	done(nil, logger.Int("requirements", 3))

	entry := fieldsOf(t, logs, "step ok")
	if entry.Level != zapcore.DebugLevel {
		t.Fatalf("level = %v, want debug", entry.Level)
	}
	m := entry.ContextMap()
	if m["op"] != "roleprofile.extract" {
		t.Fatalf("op = %v", m["op"])
	}
	if m["profile_id"] != "p-1" {
		t.Fatalf("start field missing: %v", m)
	}
	if m["requirements"] != int64(3) {
		t.Fatalf("extra field missing: %v", m)
	}
	if _, ok := m["duration"]; !ok {
		t.Fatalf("duration missing: %v", m)
	}
	if _, ok := m["cause_chain"]; ok {
		t.Fatalf("success must not carry cause_chain: %v", m)
	}
	if !strings.HasSuffix(entry.Caller.File, "context_test.go") {
		t.Fatalf("caller = %s, want the test file", entry.Caller.File)
	}
}

func TestStart_Failure(t *testing.T) {
	log, logs := newObserved(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), log)

	done := logger.Start(ctx, "roleprofile.extract")
	done(fmt.Errorf("parse: %w", errors.New("bad json")), logger.Int("attempt", 2))

	entry := fieldsOf(t, logs, "step failed")
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("level = %v, want warn", entry.Level)
	}
	m := entry.ContextMap()
	if m["op"] != "roleprofile.extract" || m["attempt"] != int64(2) {
		t.Fatalf("fields = %v", m)
	}
	if _, ok := m["duration"]; !ok {
		t.Fatalf("duration missing: %v", m)
	}
	chain, ok := m["cause_chain"].([]interface{})
	if !ok || len(chain) != 2 || chain[1] != "bad json" {
		t.Fatalf("cause_chain = %#v", m["cause_chain"])
	}
	if !strings.HasSuffix(entry.Caller.File, "context_test.go") {
		t.Fatalf("caller = %s, want the test file", entry.Caller.File)
	}
}

func TestFromContext_Precedence(t *testing.T) {
	t.Run("context logger wins over default", func(t *testing.T) {
		defaultLog, _ := newObserved(zapcore.DebugLevel)
		setDefault(t, defaultLog)
		ctxLog, _ := newObserved(zapcore.DebugLevel)

		ctx := logger.ToContext(context.Background(), ctxLog)
		if got := logger.FromContext(ctx); got != ctxLog {
			t.Fatalf("FromContext returned a different logger than the one on ctx")
		}
	})

	t.Run("default with request id from ctx", func(t *testing.T) {
		defaultLog, logs := newObserved(zapcore.DebugLevel)
		setDefault(t, defaultLog)

		ctx := logger.WithRequestID(context.Background(), "req-123")
		logger.FromContext(ctx).Info("from default")

		m := fieldsOf(t, logs, "from default").ContextMap()
		if m["request_id"] != "req-123" {
			t.Fatalf("request_id = %v, want req-123", m["request_id"])
		}
	})

	t.Run("nil ctx falls back to default", func(t *testing.T) {
		defaultLog, _ := newObserved(zapcore.DebugLevel)
		setDefault(t, defaultLog)
		if got := logger.FromContext(nil); got != defaultLog { //nolint:staticcheck // nil ctx is the case under test
			t.Fatalf("FromContext(nil) did not return the default logger")
		}
	})

	t.Run("nil logger stored on ctx is ignored", func(t *testing.T) {
		defaultLog, _ := newObserved(zapcore.DebugLevel)
		setDefault(t, defaultLog)
		ctx := logger.ToContext(context.Background(), nil)
		if got := logger.FromContext(ctx); got == nil || got.Logger == nil {
			t.Fatalf("FromContext returned nil logger")
		}
	})

	t.Run("no default and no ctx logger is a safe no-op", func(t *testing.T) {
		logger.SetDefault(nil)
		t.Cleanup(func() { logger.SetDefault(nil) })

		l := logger.FromContext(context.Background())
		if l == nil {
			t.Fatal("FromContext returned nil")
		}
		l.Info("discarded", logger.String("k", "v"))
		if logger.FromContext(nil) == nil { //nolint:staticcheck // nil ctx is the case under test
			t.Fatal("FromContext(nil) returned nil without a default")
		}
	})
}

func TestRequestID_RoundTrip(t *testing.T) {
	if got := logger.RequestID(context.Background()); got != "" {
		t.Fatalf("RequestID on bare ctx = %q, want empty", got)
	}
	ctx := logger.WithRequestID(context.Background(), "abc-123")
	if got := logger.RequestID(ctx); got != "abc-123" {
		t.Fatalf("RequestID = %q, want abc-123", got)
	}
}

// Direct calls on the test logger must report this file as the caller, not logger.go.
func TestLoggerCallerPointsAtCallSite(t *testing.T) {
	log, logs := newObserved(zapcore.InfoLevel)
	log.Info("direct")

	entry := logs.All()[0]
	if got := filepath.Base(entry.Caller.File); got != "context_test.go" {
		t.Fatalf("caller file = %s, want context_test.go", got)
	}
}

func TestWithContext_AddsRequestIDAndTraceID(t *testing.T) {
	log, logs := newObserved(zapcore.InfoLevel)
	ctx := context.WithValue(logger.WithRequestID(context.Background(), "req-9"), logger.TraceIDKey, "trace-7")
	log.WithContext(ctx).Info("scoped")

	m := logs.All()[0].ContextMap()
	if m["request_id"] != "req-9" || m["trace_id"] != "trace-7" {
		t.Fatalf("fields = %v", m)
	}
}

func TestSanitize_TruncatesOnRuneBoundary(t *testing.T) {
	// "ệ" is three bytes; a byte-offset cut at 400 would land inside one of them.
	text := strings.Repeat("ệ", 300)
	got := logger.Sanitize(text)
	if !utf8.ValidString(got) {
		t.Fatalf("truncated text is not valid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected truncation marker, got length %d", len(got))
	}
}

func TestSanitize_StripsURLQueryAndEmail(t *testing.T) {
	got := logger.Sanitize(`GET https://bucket.s3.example.com/obj/abc?X-Amz-Credential=AKIA123&sig=zzz failed for user@example.vn`)
	for _, leaked := range []string{"AKIA123", "sig=zzz", "user@example.vn", "X-Amz-Credential"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("%q leaked into %q", leaked, got)
		}
	}
	if !strings.Contains(got, "https://bucket.s3.example.com/obj/abc") {
		t.Fatalf("URL path should survive for diagnosis: %q", got)
	}
}
