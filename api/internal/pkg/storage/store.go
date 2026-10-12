// Package storage provides a small object-storage contract backed by the S3 SDK.
// Consumers own authorization, database references, and the streams they open.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid storage input")
	ErrNotFound     = errors.New("storage object not found")
)

type Store interface {
	Upload(ctx context.Context, body io.ReadSeeker, contentType string) (string, error)
	Download(ctx context.Context, key string) (Object, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (SignedRequest, error)
	// PresignPut binds a positive byte size and permits TTLs up to 15 minutes.
	// The caller must validate size and MIME against the feature policy before
	// issuing a grant, then verify the stored object during finalization.
	PresignPut(ctx context.Context, contentType string, size int64, ttl time.Duration) (UploadGrant, error)
}

// SignedRequest is a bearer grant. It may be reused before expiration; do not log
// it. Headers contains client-settable headers. Host and Content-Length are
// managed by the HTTP client/browser.
type SignedRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	// ExpiresAt is nominal; revocation/credential expiry may invalidate it earlier.
	ExpiresAt time.Time
}

// UploadGrant assigns a fresh key, but does not prove an object was uploaded.
type UploadGrant struct {
	Key     string
	Request SignedRequest
}

type Object struct {
	// Body must be closed by the caller. Keep ctx alive until reading finishes.
	Body        io.ReadCloser
	ContentType string
	Size        int64
}
