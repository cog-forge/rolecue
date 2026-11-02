package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const secretBody = "Confidential candidate CV text"

func loggedContext(t *testing.T) (context.Context, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

// rendered flattens every logged entry, with all of its fields, into one string for absence checks.
func rendered(entries []observer.LoggedEntry) string {
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

func TestStorageLogsOutcomesWithoutContentOrBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/missing-key"):
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><Error><Code>NoSuchKey</Code><Message>no</Message></Error>`)
		default:
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, secretBody)
		}
	}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs := loggedContext(t)

	key, err := store.Upload(ctx, strings.NewReader(secretBody), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Download(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	grant, err := store.PresignPut(ctx, "audio/mpeg", 42, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PresignGet(ctx, key, time.Minute); err != nil {
		t.Fatal(err)
	}

	out := rendered(logs.All())
	for _, want := range []string{"op:storage.upload", "op:storage.download", "op:storage.delete", "op:storage.presign_put", "op:storage.presign_get", "content_class:text", "content_class:audio", "bytes:42"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	signature := ""
	if u, err := url.Parse(grant.Request.URL); err == nil {
		signature = u.Query().Get("X-Amz-Signature")
	}
	for _, secret := range []string{secretBody, "test-bucket", "test-secret", "X-Amz-Signature", "X-Amz-Credential", signature} {
		if secret != "" && strings.Contains(out, secret) {
			t.Fatalf("%q leaked into logs:\n%s", secret, out)
		}
	}
}

func TestStorageClassifiesNotFoundAndOtherFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><Error><Code>AccessDenied</Code></Error>`)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<?xml version="1.0"?><Error><Code>NoSuchKey</Code></Error>`)
	}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs := loggedContext(t)

	if _, err := store.Download(ctx, "missing-key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not-found sentinel lost: %v", err)
	}
	if err := store.Delete(ctx, "some-key"); err == nil {
		t.Fatal("delete failure swallowed")
	}

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("want one warning per failed step, got:\n%s", rendered(entries))
	}
	out := rendered(entries)
	for _, want := range []string{"reason:not_found", "status_code:404", "reason:other", "status_code:403"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if entries[0].Level != zapcore.WarnLevel || entries[1].Level != zapcore.WarnLevel {
		t.Fatalf("failures must log at warn:\n%s", out)
	}
}

func TestStorageClassifiesCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs := loggedContext(t)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	if _, err := store.Download(cancelled, "some-key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not returned: %v", err)
	}
	if out := rendered(logs.All()); !strings.Contains(out, "reason:canceled") {
		t.Fatalf("cancellation not classified:\n%s", out)
	}
}
