package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPresignPutCarriesSSEHeader(t *testing.T) {
	for _, tc := range []struct {
		name, sse, kms string
		wantSSE        string
		wantKMS        string
	}{
		{"default", "", "", "AES256", ""},
		{"kms with key", "aws:kms", "alias/rolecue", "aws:kms", "alias/rolecue"},
		{"kms bucket default key", "aws:kms", "", "aws:kms", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig("https://objects.example.test")
			cfg.SSE, cfg.SSEKMSKeyID = tc.sse, tc.kms
			store, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			put, err := store.PresignPut(context.Background(), "audio/webm", 10, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if got := put.Request.Headers["X-Amz-Server-Side-Encryption"]; got != tc.wantSSE {
				t.Fatalf("SSE header = %q, want %q (headers %v)", got, tc.wantSSE, put.Request.Headers)
			}
			if got := put.Request.Headers["X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id"]; got != tc.wantKMS {
				t.Fatalf("KMS header = %q, want %q", got, tc.wantKMS)
			}
		})
	}
}

func TestPresignPutWithoutSSEForLoopbackDev(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:9000")
	cfg.SSE = "none"
	store, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	put, err := store.PresignPut(context.Background(), "audio/webm", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := put.Request.Headers["X-Amz-Server-Side-Encryption"]; ok {
		t.Fatal("SSE none must not send the header")
	}
}

func TestUploadSendsSSEHeader(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Amz-Server-Side-Encryption")
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Upload(context.Background(), strings.NewReader("x"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if got != "AES256" {
		t.Fatalf("PutObject SSE header = %q", got)
	}
}
