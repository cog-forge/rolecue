package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type s3Store struct {
	client    *s3.Client
	bucket    string
	presigner *s3.PresignClient
	sse       types.ServerSideEncryption
	kmsKeyID  string
}

// kmsKeyIDPtr returns the KMS key id to send, or nil so the bucket's default KMS key applies.
func (s *s3Store) kmsKeyIDPtr() *string {
	if s.kmsKeyID == "" {
		return nil
	}
	return aws.String(s.kmsKeyID)
}

func New(cfg config.StorageConfig) (Store, error) {
	client, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	store := &s3Store{client: client, bucket: cfg.Bucket, presigner: s3.NewPresignClient(client)}
	if mode := cfg.EffectiveSSE(); mode != config.SSENone {
		store.sse = types.ServerSideEncryption(mode)
		if mode == config.SSEKMS {
			store.kmsKeyID = cfg.SSEKMSKeyID
		}
	}
	return store, nil
}

func newClient(cfg config.StorageConfig) (*s3.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configure storage: %w", err)
	}
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       cfg.Region,
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		HTTPClient: &http.Client{
			Transport:     awshttp.NewBuildableClient().GetTransport(),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		Retryer:                    retry.NewStandard(func(o *retry.StandardOptions) { o.MaxAttempts = 3 }),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}), nil
}

func (s *s3Store) Upload(ctx context.Context, body io.ReadSeeker, contentType string) (string, error) {
	if body == nil {
		return "", fmt.Errorf("%w: upload body is required", ErrInvalidInput)
	}
	if err := validateContentType(contentType); err != nil {
		return "", err
	}
	if offset, err := body.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
		return "", fmt.Errorf("%w: upload body must be seekable and positioned at its start", ErrInvalidInput)
	}
	key := uuid.NewString()
	done := logger.Start(ctx, "storage.upload", logger.String("content_class", contentTypeClass(contentType)))
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: body, ContentType: aws.String(contentType), ServerSideEncryption: s.sse, SSEKMSKeyId: s.kmsKeyIDPtr()})
	if err != nil {
		err = fmt.Errorf("upload object: %w", err)
		done(err, failureFields(err)...)
		return "", err
	}
	done(nil)
	return key, nil
}

func (s *s3Store) Download(ctx context.Context, key string) (Object, error) {
	if err := validateKey(key); err != nil {
		return Object{}, err
	}
	done := logger.Start(ctx, "storage.download")
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		err = notFoundAware("download object", err)
		done(err, failureFields(err)...)
		return Object{}, err
	}
	done(nil, zap.Int64("bytes", aws.ToInt64(out.ContentLength)), zap.String("content_class", contentTypeClass(aws.ToString(out.ContentType))))
	return Object{Body: out.Body, ContentType: aws.ToString(out.ContentType), Size: aws.ToInt64(out.ContentLength)}, nil
}

// Head returns the metadata of an object. A HEAD response has no body, so the SDK reports a missing object
// as NotFound rather than NoSuchKey; both map to ErrNotFound.
func (s *s3Store) Head(ctx context.Context, key string) (ObjectInfo, error) {
	if err := validateKey(key); err != nil {
		return ObjectInfo{}, err
	}
	done := logger.Start(ctx, "storage.head")
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		err = notFoundAware("head object", err)
		done(err, failureFields(err)...)
		return ObjectInfo{}, err
	}
	done(nil, zap.Int64("bytes", aws.ToInt64(out.ContentLength)))
	return ObjectInfo{ETag: aws.ToString(out.ETag), Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType)}, nil
}

// maxPrefixBytes bounds ReadPrefix so a caller cannot use it to read an object into memory.
const maxPrefixBytes = 1 << 16

// ReadPrefix reads a ranged GET of the first n bytes. An object shorter than n returns what it has.
func (s *s3Store) ReadPrefix(ctx context.Context, key string, n int64) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if n < 1 || n > maxPrefixBytes {
		return nil, fmt.Errorf("%w: prefix length must be between 1 and %d bytes", ErrInvalidInput, maxPrefixBytes)
	}
	done := logger.Start(ctx, "storage.read_prefix", zap.Int64("max_bytes", n))
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Range: aws.String(fmt.Sprintf("bytes=0-%d", n-1)),
	})
	if err != nil {
		err = notFoundAware("read object prefix", err)
		done(err, failureFields(err)...)
		return nil, err
	}
	defer func() { _ = out.Body.Close() }()
	buf, err := io.ReadAll(io.LimitReader(out.Body, n))
	if err != nil {
		err = fmt.Errorf("read object prefix: %w", err)
		done(err, failureFields(err)...)
		return nil, err
	}
	done(nil, zap.Int64("bytes", int64(len(buf))))
	return buf, nil
}

// notFoundAware wraps an SDK error for op, adding ErrNotFound when the object does not exist.
func notFoundAware(op string, err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
		return fmt.Errorf("%s: %w: %w", op, ErrNotFound, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// contentTypeClass keeps only the media type family ("text", "audio"), which is enough to diagnose
// a bad object without logging the full type string.
func contentTypeClass(contentType string) string {
	class, _, _ := strings.Cut(contentType, "/")
	return class
}

// failureFields classifies a storage failure for the step log. The sanitized cause chain carries the
// SDK message; the reason and HTTP status make the failure class searchable.
func failureFields(err error) []zap.Field {
	if err == nil {
		return nil
	}
	reason := "other"
	switch {
	case errors.Is(err, ErrNotFound):
		reason = "not_found"
	case errors.Is(err, context.Canceled):
		reason = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "timeout"
	}
	fields := []zap.Field{logger.String("reason", reason)}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		fields = append(fields, logger.Int("status_code", respErr.HTTPStatusCode()))
	}
	return fields
}

func validateContentType(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.Contains(mediaType, "/") || strings.ContainsAny(contentType, "\r\n") {
		return fmt.Errorf("%w: valid content type is required", ErrInvalidInput)
	}
	return nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	done := logger.Start(ctx, "storage.delete")
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		err = fmt.Errorf("delete object: %w", err)
		done(err, failureFields(err)...)
		return err
	}
	done(nil)
	return nil
}

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" || len(key) > 1024 || strings.ContainsAny(key, "\x00\r\n") {
		return fmt.Errorf("%w: valid object key is required", ErrInvalidInput)
	}
	return nil
}

func (s *s3Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (SignedRequest, error) {
	return s.presignGet(ctx, key, ttl, GetOverrides{})
}

func (s *s3Store) PresignGetWithOverrides(ctx context.Context, key string, ttl time.Duration, overrides GetOverrides) (SignedRequest, error) {
	for _, v := range []string{overrides.ContentType, overrides.ContentDisposition} {
		if strings.ContainsAny(v, "\r\n\x00") {
			return SignedRequest{}, fmt.Errorf("%w: response override must be one header line", ErrInvalidInput)
		}
	}
	if overrides.ContentType != "" {
		if err := validateContentType(overrides.ContentType); err != nil {
			return SignedRequest{}, err
		}
	}
	return s.presignGet(ctx, key, ttl, overrides)
}

func (s *s3Store) presignGet(ctx context.Context, key string, ttl time.Duration, overrides GetOverrides) (SignedRequest, error) {
	if err := ctx.Err(); err != nil {
		return SignedRequest{}, err
	}
	if err := validateKey(key); err != nil {
		return SignedRequest{}, err
	}
	if err := validateTTL(ttl); err != nil {
		return SignedRequest{}, err
	}
	input := &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}
	// The overrides are signed into the URL, so the browser cannot change the type or the download name.
	if overrides.ContentType != "" {
		input.ResponseContentType = aws.String(overrides.ContentType)
	}
	if overrides.ContentDisposition != "" {
		input.ResponseContentDisposition = aws.String(overrides.ContentDisposition)
	}
	done := logger.Start(ctx, "storage.presign_get", zap.Duration("ttl", ttl))
	out, err := s.presigner.PresignGetObject(ctx, input, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		err = fmt.Errorf("presign GET: %w", err)
		done(err, failureFields(err)...)
		return SignedRequest{}, err
	}
	done(nil)
	return signedRequest(out, ttl)
}

func (s *s3Store) PresignPut(ctx context.Context, contentType string, size int64, ttl time.Duration) (UploadGrant, error) {
	if err := ctx.Err(); err != nil {
		return UploadGrant{}, err
	}
	if err := validateContentType(contentType); err != nil {
		return UploadGrant{}, err
	}
	if size <= 0 {
		return UploadGrant{}, fmt.Errorf("%w: upload grant size must be positive", ErrInvalidInput)
	}
	if err := validateTTL(ttl); err != nil {
		return UploadGrant{}, err
	}
	if ttl > 15*time.Minute {
		return UploadGrant{}, fmt.Errorf("%w: upload grant TTL must not exceed 15 minutes", ErrInvalidInput)
	}
	key := uuid.NewString()
	done := logger.Start(ctx, "storage.presign_put", zap.Int64("bytes", size), logger.String("content_class", contentTypeClass(contentType)), zap.Duration("ttl", ttl))
	out, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType), ContentLength: aws.Int64(size), ServerSideEncryption: s.sse, SSEKMSKeyId: s.kmsKeyIDPtr()}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		err = fmt.Errorf("presign PUT: %w", err)
		done(err, failureFields(err)...)
		return UploadGrant{}, err
	}
	done(nil)
	request, err := signedRequest(out, ttl)
	if err != nil {
		return UploadGrant{}, err
	}
	return UploadGrant{Key: key, Request: request}, nil
}

func validateTTL(ttl time.Duration) error {
	if ttl < time.Second || ttl > 7*24*time.Hour || ttl%time.Second != 0 {
		return fmt.Errorf("%w: presign TTL must be whole seconds between 1s and 7 days", ErrInvalidInput)
	}
	return nil
}

func signedRequest(out *v4.PresignedHTTPRequest, ttl time.Duration) (SignedRequest, error) {
	u, err := url.Parse(out.URL)
	if err != nil {
		return SignedRequest{}, errors.New("SDK returned invalid presigned URL")
	}
	signedAt, err := time.Parse("20060102T150405Z", u.Query().Get("X-Amz-Date"))
	if err != nil {
		return SignedRequest{}, errors.New("SDK returned invalid signing timestamp")
	}
	headers := make(map[string]string, len(out.SignedHeader))
	for name, values := range out.SignedHeader {
		if !strings.EqualFold(name, "Host") && !strings.EqualFold(name, "Content-Length") {
			headers[http.CanonicalHeaderKey(name)] = strings.Join(values, ",")
		}
	}
	return SignedRequest{URL: out.URL, Method: out.Method, Headers: headers, ExpiresAt: signedAt.Add(ttl)}, nil
}
