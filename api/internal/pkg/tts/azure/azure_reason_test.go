package azure_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
)

func TestFailedAttemptLogsClassifiedReason(t *testing.T) {
	const secretText = "Private interview question about the salary band"
	for _, tt := range []struct {
		name   string
		code   int
		reason string
		want   error
	}{
		{"credentials rejected", http.StatusUnauthorized, "auth_rejected", tts.ErrUnavailable},
		{"quota", http.StatusTooManyRequests, "rate_limited", tts.ErrQuota},
		{"rejected request", http.StatusBadRequest, "http_4xx", tts.ErrRejected},
		{"provider outage", http.StatusServiceUnavailable, "http_5xx", tts.ErrUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			log := &logger.Logger{Logger: zap.New(core)}
			c, _ := stub(t, status(tt.code), azure.WithLogger(log), azure.WithRetries(0))

			_, err := c.Synthesize(context.Background(), req(secretText))
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			entries := logs.All()
			if len(entries) != 1 || entries[0].Message != "tts attempt failed" {
				t.Fatalf("want one failed-attempt entry, got %d", len(entries))
			}
			if got := entries[0].ContextMap()["reason"]; got != tt.reason {
				t.Fatalf("reason = %v, want %s", got, tt.reason)
			}
			var b strings.Builder
			for _, e := range entries {
				b.WriteString(fmt.Sprint(e.Message, e.ContextMap()))
			}
			if dump := b.String(); strings.Contains(dump, secretText) || strings.Contains(dump, testKey) {
				t.Fatalf("log leaks text or key: %s", dump)
			}
		})
	}
}
