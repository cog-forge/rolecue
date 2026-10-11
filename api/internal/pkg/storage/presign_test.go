package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPresignUsesConfiguredSDK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("presigning must not call storage") }))
	defer server.Close()
	cfg := testConfig(server.URL)
	cfg.Endpoint = "https://objects.example.test"
	cfg.Region = "auto"
	cfg.SessionToken = "test-session-token"
	store, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	get, err := store.PresignGet(context.Background(), "opaque/key with spaces", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(get.URL)
	if err != nil {
		t.Fatal(err)
	}
	wantHost, wantPath := "objects.example.test", "/test-bucket/opaque/key with spaces"
	if u.Host != wantHost || u.Path != wantPath || get.Method != "GET" {
		t.Fatalf("wrong endpoint/method: host=%s path=%s method=%s", u.Host, u.Path, get.Method)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "900" || !strings.Contains(q.Get("X-Amz-Credential"), "/auto/s3/aws4_request") || q.Get("X-Amz-Security-Token") != cfg.SessionToken {
		t.Fatal("SDK signing ignored config")
	}
	signedAt, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	if err != nil || !get.ExpiresAt.Equal(signedAt.Add(15*time.Minute)) {
		t.Fatal("expiry does not match signature")
	}
	if _, ok := get.Headers["Host"]; ok {
		t.Fatal("grant requires browser to override Host")
	}
	put, err := store.PresignPut(context.Background(), "text/plain", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(put.Key); err != nil {
		t.Fatal("PUT key is not opaque")
	}
	putURL, _ := url.Parse(put.Request.URL)
	if !strings.HasSuffix(putURL.Path, "/"+put.Key) || put.Request.Method != "PUT" || put.Request.Headers["Content-Type"] != "text/plain" {
		t.Fatal("PUT grant missing key/method/metadata")
	}
	if !strings.Contains(putURL.Query().Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatal("pinned SDK did not sign the requested content type")
	}
	other, err := store.PresignPut(context.Background(), "text/plain", time.Minute)
	if err != nil || other.Key == put.Key {
		t.Fatal("independent grants share an upload key")
	}
}

func TestPresignRejectsInvalidInput(t *testing.T) {
	store, err := New(testConfig("http://localhost:9000"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ttl := range []time.Duration{-time.Second, 0, time.Millisecond, time.Second + time.Nanosecond, 7*24*time.Hour + time.Second} {
		if _, err := store.PresignGet(context.Background(), "key", ttl); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("GET accepted TTL %s", ttl)
		}
		if _, err := store.PresignPut(context.Background(), "text/plain", ttl); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("PUT accepted TTL %s", ttl)
		}
	}
	if _, err := store.PresignGet(context.Background(), "", time.Minute); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("GET accepted empty key")
	}
	if _, err := store.PresignPut(context.Background(), "", time.Minute); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("PUT accepted empty content type")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.PresignGet(ctx, "key", time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatal("GET signing ignored canceled context")
	}
}
