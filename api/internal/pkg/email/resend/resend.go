// Package resend implements email.Sender on the Resend HTTP API (POST /emails).
//
// Errors are classified for the Dispatcher: 429, 5xx and transport failures are retryable, other 4xx
// responses are Permanent because the same request will be rejected again. Responses are never echoed into
// errors, since a provider error body can contain the recipient or the request payload.
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
)

const (
	defaultBaseURL = "https://api.resend.com"
	defaultTimeout = 10 * time.Second
	// maxDrainBytes bounds how much of a response body is read so a hostile or huge reply cannot hold the
	// connection. Nothing from the body is kept.
	maxDrainBytes = 64 << 10
)

// Config selects the Resend account and sender address. Timeout bounds one request; zero means 10 seconds.
// BaseURL is for tests and an empty value means the public API.
type Config struct {
	APIKey  string
	From    string
	BaseURL string
	Timeout time.Duration
}

type client struct {
	apiKey   string
	from     string
	endpoint string
	http     *http.Client
}

// New returns a Sender for cfg. Configuration is not validated here: a missing key surfaces as a Permanent
// 401/403 on the first send, which the Dispatcher logs without retrying.
func New(cfg Config) email.Sender {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &client{
		apiKey:   cfg.APIKey,
		from:     cfg.From,
		endpoint: strings.TrimRight(base, "/") + "/emails",
		// Redirects are refused so the bearer key is never sent to another host.
		http: &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// payload is the Resend send-email body. Only text is sent; the dispatcher never produces HTML.
type payload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (c *client) Send(ctx context.Context, msg email.Message) error {
	body, err := json.Marshal(payload{From: c.from, To: []string{msg.To}, Subject: msg.Subject, Text: msg.Body})
	if err != nil {
		return email.Permanent(errors.New("resend: encode request"))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return email.Permanent(errors.New("resend: build request"))
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// The same key on every retry of one message, so Resend drops a duplicate if an earlier attempt had
	// already been accepted when it timed out.
	if msg.ID != "" {
		req.Header.Set("Idempotency-Key", msg.ID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("resend: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))

	code := resp.StatusCode
	switch {
	case code >= 200 && code < 300:
		return nil
	// 409 is Resend's concurrent_idempotent_requests: the retry arrived while the first attempt with the same
	// key was still running, so it is worth another try (400 and 422 stay permanent).
	case code == http.StatusTooManyRequests || code == http.StatusRequestTimeout || code == http.StatusConflict || code >= 500:
		return fmt.Errorf("resend: provider returned HTTP %d", code)
	default:
		return email.Permanent(fmt.Errorf("resend: provider rejected email: HTTP %d", code))
	}
}
