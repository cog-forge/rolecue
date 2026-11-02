// Package email is the vendor-neutral transactional email boundary. Features build a Message from a Template
// and hand it to a Dispatcher; a provider (internal/pkg/email/resend) or the log-only adapter
// (internal/pkg/email/logsender) sits behind Sender, so swapping vendors never touches the features.
package email

import (
	"context"
	"errors"
)

// Message is one plain-text email.
type Message struct {
	To      string
	Subject string
	Body    string
	// ID names this message across every retry; a provider that supports it uses it to refuse a duplicate
	// when an attempt timed out after the provider had already accepted the mail. Dispatch fills it in
	// when the caller leaves it empty.
	ID string
}

// Sender delivers one message. A returned error is retried by the Dispatcher unless it is Permanent.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// permanentError marks a failure that retrying cannot fix, such as a rejected address or a bad API key.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so the Dispatcher never retries it. Use it for rejections (4xx from a provider), not
// for outages, which are worth another attempt.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent reports whether err, or any error it wraps, was marked with Permanent.
func IsPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}
