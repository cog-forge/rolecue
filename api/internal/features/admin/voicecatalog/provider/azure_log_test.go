package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const logTestKey = "secret-key"

func renderedLogs(entries []observer.LoggedEntry) string {
	var b strings.Builder
	for _, e := range entries {
		enc := zapcore.NewMapObjectEncoder()
		for _, f := range e.Context {
			f.AddTo(enc)
		}
		fmt.Fprintf(&b, "%s %s %v\n", e.Level, e.Message, enc.Fields)
	}
	return b.String()
}

func TestAzureListLogsCountAndSuccess(t *testing.T) {
	a := stub(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(azureBody)) })
	core, logs := observer.New(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})

	if _, err := a.ListVoices(ctx); err != nil {
		t.Fatal(err)
	}
	out := renderedLogs(logs.All())
	if !strings.Contains(out, "debug step ok") || !strings.Contains(out, "voices:2") || !strings.Contains(out, "op:voicecatalog.list_azure") {
		t.Fatalf("success not logged with count:\n%s", out)
	}
}

func TestAzureListLogsClassifiedFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		body   string
		code   int
		reason string
	}{
		{"credentials rejected", "", http.StatusUnauthorized, "auth_rejected"},
		{"rate limited", "", http.StatusTooManyRequests, "rate_limited"},
		{"client error", "", http.StatusNotFound, "http_4xx"},
		{"provider outage", "", http.StatusServiceUnavailable, "http_5xx"},
		{"malformed answer", "{not json", http.StatusOK, "decode_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := stub(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			})
			core, logs := observer.New(zapcore.DebugLevel)
			ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})

			if _, err := a.ListVoices(ctx); err == nil {
				t.Fatal("failure swallowed")
			}
			entries := logs.All()
			if len(entries) != 1 || entries[0].Level != zapcore.WarnLevel {
				t.Fatalf("want one warn, got:\n%s", renderedLogs(entries))
			}
			out := renderedLogs(entries)
			if !strings.Contains(out, "reason:"+tt.reason) {
				t.Fatalf("reason %s missing:\n%s", tt.reason, out)
			}
			if strings.Contains(out, logTestKey) {
				t.Fatalf("api key logged:\n%s", out)
			}
		})
	}
}

func TestAzureListLogsCancellationAndTimeout(t *testing.T) {
	a := stub(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	a.client.Timeout = 50 * time.Millisecond
	core, logs := observer.New(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	if _, err := a.ListVoices(ctx); err == nil {
		t.Fatal("timeout swallowed")
	}
	if out := renderedLogs(logs.All()); !strings.Contains(out, "reason:timeout") {
		t.Fatalf("timeout not classified:\n%s", out)
	}

	cancelled, cancel := context.WithCancel(logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}))
	cancel()
	if _, err := a.ListVoices(cancelled); err == nil {
		t.Fatal("cancellation swallowed")
	}
	if out := renderedLogs(logs.All()); !strings.Contains(out, "reason:canceled") {
		t.Fatalf("cancellation not classified:\n%s", out)
	}
}
