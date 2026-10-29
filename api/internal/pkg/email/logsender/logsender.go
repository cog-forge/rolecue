// Package logsender is the email.Sender used when no provider is configured. It sends nothing and only records
// that mail was skipped, so local runs and tests never reach a real inbox.
package logsender

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

type sender struct {
	log *logger.Logger
}

// New returns a Sender that logs each skipped message. The log line carries no recipient, subject or body.
func New(log *logger.Logger) email.Sender {
	return sender{log: log}
}

func (s sender) Send(_ context.Context, _ email.Message) error {
	if s.log != nil {
		s.log.Info("email skipped: no email provider configured")
	}
	return nil
}
