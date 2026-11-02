package email_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const (
	secretRecipient = "secret-candidate@example.com"
	secretSubject   = "SECRET-SUBJECT-LINE"
	secretBody      = "SECRET-BODY-TEXT"
	secretKey       = "re_SECRET-API-KEY"
	secretLabel     = "welcome-label"
)

// syncBuffer lets zap write from several workers at once while the test reads the result.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newBufferLogger(out *syncBuffer) *logger.Logger {
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	return &logger.Logger{Logger: zap.New(zapcore.NewCore(enc, zapcore.AddSync(out), zapcore.DebugLevel))}
}

// TestLogsNeverContainSensitiveData runs each outcome through a logging dispatcher and checks that no log
// line carries the recipient, subject, body or API key. The sender's error deliberately includes the secrets,
// so the test also proves the dispatcher does not log error text.
func TestLogsNeverContainSensitiveData(t *testing.T) {
	secretMsg := email.Message{To: secretRecipient, Subject: secretSubject, Body: secretBody}
	leakyErr := errors.New("provider rejected " + secretRecipient + " " + secretSubject + " " + secretBody + " " + secretKey)

	cases := []struct {
		name string
		send func(call int) error
	}{
		{"success", func(int) error { return nil }},
		{"retryable failure", func(int) error { return leakyErr }},
		{"permanent failure", func(int) error { return email.Permanent(leakyErr) }},
		{"panic", func(int) error { panic(leakyErr.Error()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out syncBuffer
			s := &fakeSender{send: func(_ context.Context, call int, _ email.Message) error { return tc.send(call) }}
			d := email.NewDispatcher(s, email.DispatcherOptions{MaxAttempts: 2, Logger: newBufferLogger(&out)})
			d.Dispatch(secretLabel, secretMsg)
			closeNow(t, d)

			logs := out.String()
			if !strings.Contains(logs, secretLabel) {
				t.Fatalf("expected the safe label in the logs, got:\n%s", logs)
			}
			for _, secret := range []string{secretRecipient, secretSubject, secretBody, secretKey} {
				if strings.Contains(logs, secret) {
					t.Fatalf("log leaked %q:\n%s", secret, logs)
				}
			}
		})
	}
}

func TestQueueFullLogIsSafe(t *testing.T) {
	var out syncBuffer
	release := make(chan struct{})
	s := &fakeSender{send: func(context.Context, int, email.Message) error { <-release; return nil }}
	d := email.NewDispatcher(s, email.DispatcherOptions{Workers: 1, QueueSize: 1, Logger: newBufferLogger(&out)})
	d.Dispatch("first", email.Message{To: "keep@example.com"})
	d.Dispatch("second", email.Message{To: "keep@example.com"})
	d.Dispatch(secretLabel, email.Message{To: secretRecipient, Subject: secretSubject, Body: secretBody})
	close(release)
	closeNow(t, d)

	logs := out.String()
	if !strings.Contains(logs, "queue full") || !strings.Contains(logs, secretLabel) {
		t.Fatalf("expected a queue-full warning with the label, got:\n%s", logs)
	}
	for _, secret := range []string{secretRecipient, secretSubject, secretBody} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log leaked %q", secret)
		}
	}
}
