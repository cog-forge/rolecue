package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStorageLogsNeverContainObjectKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "body")
	}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, logs := loggedContext(t)
	key, err := store.Upload(ctx, strings.NewReader("x"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if obj, err := store.Download(ctx, key); err == nil {
		_ = obj.Body.Close()
	}
	_, _ = store.Head(ctx, key)
	_, _ = store.ReadPrefix(ctx, key, 1)
	_, _ = store.PresignGet(ctx, key, time.Minute)
	grant, err := store.PresignPut(ctx, "text/plain", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Delete(ctx, key)
	out := rendered(logs.All())
	for _, k := range []string{key, grant.Key} {
		if strings.Contains(out, k) {
			t.Fatalf("object key %q leaked into logs:\n%s", k, out)
		}
	}
}
