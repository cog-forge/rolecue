//go:build storage_integration

package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cog-forge/rolecue/api/internal/config"
)

func TestS3RoundTrip(t *testing.T) {
	// config.Load requires PayOS settings, but this storage test never calls PayOS.
	t.Setenv("PAYOS_CLIENT_ID", "test-client-id")
	t.Setenv("PAYOS_API_KEY", "test-api-key")
	t.Setenv("PAYOS_CHECKSUM_KEY", "test-checksum-key")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	client, err := newClient(cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Storage.Bucket)}); err != nil {
		t.Fatalf("storage integration requires a reachable provisioned bucket: %v", err)
	}
	var keys []string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, key := range keys {
			if _, err := client.DeleteObject(cleanupCtx, &s3.DeleteObjectInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(key)}); err != nil {
				t.Errorf("cleanup owned fixture: %v", err)
			}
		}
	})
	for _, payload := range []string{"storage integration fixture", ""} {
		key, err := store.Upload(ctx, strings.NewReader(payload), "text/plain")
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
		assertObject(t, ctx, store, key, payload)
		grant, err := store.PresignGet(ctx, key, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		status, body, _ := requestGrant(t, grant, "", "")
		if status != 200 || body != payload {
			t.Fatal("presigned GET round trip failed")
		}
		unsigned := SignedRequest{URL: strings.TrimRight(cfg.Storage.Endpoint, "/") + "/" + cfg.Storage.Bucket + "/" + key, Method: "GET"}
		status, _, _ = requestGrant(t, unsigned, "", "")
		if status != http.StatusForbidden {
			t.Fatalf("anonymous GET allowed: %d", status)
		}
		if payload == "" {
			continue
		}
		put, err := store.PresignPut(ctx, "text/plain", int64(len(payload)), time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, put.Key)
		if _, err := store.Download(ctx, put.Key); !errors.Is(err, ErrNotFound) {
			t.Fatal("grant incorrectly implies uploaded object")
		}
		status, _, headers := requestGrant(t, put.Request, payload, "http://localhost:3000")
		if status != 200 || headers.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatalf("direct PUT or CORS failed: %d", status)
		}
		assertObject(t, ctx, store, put.Key, payload)
		get, err := store.PresignGet(ctx, put.Key, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		status, body, headers = requestGrant(t, get, "", "http://localhost:3000")
		if status != 200 || body != payload || headers.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatal("direct PUT → GET with CORS failed")
		}
		assertTamperingRejected(t, get, put.Request, payload)
		for _, wrongSize := range []string{payload + "x", payload[:len(payload)-1]} {
			status, _, _ = requestGrant(t, put.Request, wrongSize, "")
			if status != http.StatusForbidden {
				t.Fatalf("upload with wrong byte size accepted: %d", status)
			}
		}
		assertObject(t, ctx, store, put.Key, payload)
		assertPreflight(t, put.Request, "http://localhost:3000")
		status, _, headers = requestGrant(t, get, "", "https://untrusted.example.test")
		if status != 200 || headers.Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("untrusted origin received permission to read signed GET response")
		}
	}
	get, err := store.PresignGet(ctx, keys[0], time.Second)
	if err != nil {
		t.Fatal(err)
	}
	put, err := store.PresignPut(ctx, "text/plain", int64(len("expired")), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	keys = append(keys, put.Key)
	expires := get.ExpiresAt
	if put.Request.ExpiresAt.After(expires) {
		expires = put.Request.ExpiresAt
	}
	// Server timestamps have second resolution. Bound the expiry check to <3s.
	time.Sleep(time.Until(expires.Add(time.Second)))
	for _, grant := range []SignedRequest{get, put.Request} {
		status, _, _ := requestGrant(t, grant, "expired", "")
		if status != http.StatusForbidden {
			t.Fatalf("expired %s accepted: %d", grant.Method, status)
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if key, err := store.Upload(canceled, strings.NewReader("x"), "text/plain"); key != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled upload reported success")
	}
}

func assertObject(t *testing.T, ctx context.Context, store Store, key, want string) {
	t.Helper()
	object, err := store.Download(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = object.Body.Close() }()
	body, err := io.ReadAll(object.Body)
	if err != nil || string(body) != want || object.Size != int64(len(want)) || object.ContentType != "text/plain" {
		t.Fatal("download bytes or metadata do not match")
	}
}

func requestGrant(t *testing.T, grant SignedRequest, payload, origin string) (int, string, http.Header) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, grant.Method, grant.URL, strings.NewReader(payload))
	if err != nil {
		t.Fatal("could not construct grant request")
	}
	for name, value := range grant.Headers {
		req.Header.Set(name, value)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("signed HTTP request failed (URL omitted)")
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("could not read signed response")
	}
	return resp.StatusCode, string(body), resp.Header
}

func assertTamperingRejected(t *testing.T, get, put SignedRequest, payload string) {
	t.Helper()
	for _, original := range []SignedRequest{get, put} {
		u, _ := url.Parse(original.URL)
		u.Path += "-changed"
		changed := original
		changed.URL = u.String()
		status, _, _ := requestGrant(t, changed, payload, "")
		if status != http.StatusForbidden {
			t.Fatal("changed object key accepted")
		}
		u, _ = url.Parse(original.URL)
		q := u.Query()
		q.Set("X-Amz-Signature", strings.Repeat("0", 64))
		u.RawQuery = q.Encode()
		changed.URL = u.String()
		status, _, _ = requestGrant(t, changed, payload, "")
		if status != http.StatusForbidden {
			t.Fatal("changed signature accepted")
		}
		changed = original
		if original.Method == "GET" {
			changed.Method = "PUT"
		} else {
			changed.Method = "GET"
		}
		status, _, _ = requestGrant(t, changed, payload, "")
		if status != http.StatusForbidden {
			t.Fatal("changed request method accepted")
		}
	}
	changed := put
	changed.Headers = make(map[string]string, len(put.Headers))
	for name, value := range put.Headers {
		changed.Headers[name] = value
	}
	changed.Headers["Content-Type"] = "application/octet-stream"
	status, _, _ := requestGrant(t, changed, payload, "")
	if status != http.StatusForbidden {
		t.Fatal("changed signed Content-Type accepted")
	}
	delete(changed.Headers, "Content-Type")
	status, _, _ = requestGrant(t, changed, payload, "")
	if status != http.StatusBadRequest && status != http.StatusForbidden {
		t.Fatalf("missing signed Content-Type was not rejected as an unsigned header: %d", status)
	}
}

func assertPreflight(t *testing.T, put SignedRequest, origin string) {
	t.Helper()
	preflight := SignedRequest{URL: put.URL, Method: "OPTIONS", Headers: map[string]string{"Access-Control-Request-Method": "PUT", "Access-Control-Request-Headers": "content-type"}}
	status, _, headers := requestGrant(t, preflight, "", origin)
	allowedOrigin := headers.Get("Access-Control-Allow-Origin")
	// A public preflight may allow any origin; the actual response must enforce
	// bucket CORS. Signed requests use credentials: "omit", so * is browser-valid.
	if status >= 300 || (allowedOrigin != origin && allowedOrigin != "*") || !strings.Contains(headers.Get("Access-Control-Allow-Methods"), "PUT") || !strings.Contains(strings.ToLower(headers.Get("Access-Control-Allow-Headers")), "content-type") {
		t.Fatalf("PUT preflight not permitted: status=%d origin=%q methods=%q headers=%q", status, allowedOrigin, headers.Get("Access-Control-Allow-Methods"), headers.Get("Access-Control-Allow-Headers"))
	}
}
