package logsender_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/email/logsender"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

func TestSendWithoutLoggerDoesNotPanic(t *testing.T) {
	s := logsender.New(nil)
	if err := s.Send(context.Background(), email.Message{To: "a@example.com"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestSendLogsSkipWithoutContent(t *testing.T) {
	var buf bytes.Buffer
	log := &logger.Logger{Logger: zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), zapcore.DebugLevel))}

	s := logsender.New(log)
	msg := email.Message{To: "secret@example.com", Subject: "SECRET-SUBJECT", Body: "SECRET-BODY"}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "skipped") {
		t.Fatalf("expected a skip line, got %q", logs)
	}
	for _, secret := range []string{msg.To, msg.Subject, msg.Body} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log leaked %q", secret)
		}
	}
}
