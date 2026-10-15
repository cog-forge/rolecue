package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/google/uuid"
)

func testConfig(endpoint string) config.StorageConfig {
	return config.StorageConfig{Endpoint: endpoint, Region: "us-east-1", Bucket: "test-bucket", AccessKeyID: "test-key", SecretAccessKey: "test-secret"}
}

func TestSDKUploadDownload(t *testing.T) {
	var saved, key string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test-key/") {
			t.Error("SDK did not sign with configured credentials")
		}
		if !strings.HasPrefix(r.URL.Path, "/test-bucket/") {
			t.Error("SDK did not use configured bucket/path addressing")
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			saved = string(body)
			key = strings.TrimPrefix(r.URL.Path, "/test-bucket/")
			if r.Header.Get("Content-Type") != "text/plain" {
				t.Error("missing content type")
			}
		case http.MethodGet:
			if r.URL.Path != "/test-bucket/"+key {
				t.Error("wrong download key")
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, saved)
		}
	}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	objectKey, err := store.Upload(context.Background(), strings.NewReader("hello storage"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(objectKey); err != nil || objectKey != key {
		t.Fatal("upload did not return opaque generated key")
	}
	object, err := store.Download(context.Background(), objectKey)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = object.Body.Close() }()
	body, err := io.ReadAll(object.Body)
	if err != nil || string(body) != "hello storage" || object.ContentType != "text/plain" || object.Size != 13 {
		t.Fatalf("unexpected download: size=%d err=%v", object.Size, err)
	}
}

func TestDownloadStreamsAndPreservesCancellation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "6")
		_, _ = io.WriteString(w, "first")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	object, err := store.Download(ctx, "object-key")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = object.Body.Close() }()
	prefix := make([]byte, 5)
	if _, err := io.ReadFull(object.Body, prefix); err != nil || string(prefix) != "first" {
		t.Fatal("stream unavailable to caller")
	}
	cancel()
	if _, err := io.ReadAll(object.Body); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestDownloadErrors(t *testing.T) {
	for _, code := range []string{"NoSuchKey", "NoSuchBucket", "AccessDenied"} {
		t.Run(code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				if code == "AccessDenied" {
					w.WriteHeader(403)
				} else {
					w.WriteHeader(404)
				}
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code></Error>")
			}))
			defer server.Close()
			store, err := New(testConfig(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.Download(context.Background(), "missing")
			if err == nil || errors.Is(err, ErrNotFound) != (code == "NoSuchKey") {
				t.Fatalf("wrong error classification: %v", err)
			}
		})
	}
}

func TestInvalidInputsMakeNoRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("invalid input reached SDK transport") }))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if key, err := store.Upload(context.Background(), nil, "text/plain"); key != "" || !errors.Is(err, ErrInvalidInput) {
		t.Fatal("nil upload accepted")
	}
	if _, err := store.Upload(context.Background(), strings.NewReader("x"), "text/plain\r\nX: bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("invalid MIME accepted")
	}
	if _, err := store.Upload(context.Background(), strings.NewReader("x"), "not-a-mime-type"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("content type without type/subtype accepted")
	}
	if _, err := store.Download(context.Background(), ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("empty key accepted")
	}
	if err := store.Delete(context.Background(), ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("empty delete key accepted")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if key, err := store.Upload(canceled, strings.NewReader("x"), "text/plain"); key != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled upload reported success")
	}
}

func TestUploadFailureReturnsNoKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "<Error><Code>AccessDenied</Code></Error>")
	}))
	defer server.Close()
	store, err := New(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.Upload(context.Background(), strings.NewReader("failed upload"), "text/plain")
	if key != "" || err == nil {
		t.Fatal("failed SDK upload returned a usable object key")
	}
}

func TestConstructorDoesNotFallBackToMachineCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "machine-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "machine-secret")
	cfg := testConfig("http://localhost:9000")
	cfg.AccessKeyID, cfg.SecretAccessKey = "", ""
	if _, err := New(cfg); err == nil {
		t.Fatal("constructor accepted missing configured credentials")
	}
}

func TestDeleteObject(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(fmt.Sprint("denied=", denied), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/test-bucket/converted.vrm" {
					t.Errorf("wrong delete target: %s %s", r.Method, r.URL.Path)
				}
				if denied {
					w.WriteHeader(http.StatusForbidden)
					_, _ = io.WriteString(w, "<Error><Code>AccessDenied</Code></Error>")
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			store, err := New(testConfig(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			err = store.Delete(context.Background(), "converted.vrm")
			if (err != nil) != denied {
				t.Fatalf("delete error = %v", err)
			}
		})
	}
}
