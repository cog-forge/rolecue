package provider

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/email/logsender"
	"github.com/cog-forge/rolecue/api/internal/pkg/email/resend"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// emailDrainTimeout is how long shutdown waits for queued mail before it gives up on the rest.
const emailDrainTimeout = 10 * time.Second

// ProvideEmailSender wires Resend when EMAIL_PROVIDER is resend (config validation already requires its
// key and sender), and otherwise a sender that only logs, so local development and tests never send mail.
func ProvideEmailSender(cfg *config.Config, log *logger.Logger) email.Sender {
	if cfg.Email.Provider == "resend" {
		log.Info("Transactional email enabled", logger.String("provider", "resend"))
		return resend.New(resend.Config{APIKey: cfg.Email.APIKey, From: cfg.Email.From, Timeout: cfg.Email.Timeout})
	}
	if cfg.IsProduction() {
		// A deploy that forgot EMAIL_PROVIDER would otherwise run silently without sending any mail.
		log.Warn("Transactional email is log-only in production: set EMAIL_PROVIDER=resend to send mail")
		return logsender.New(log)
	}
	log.Info("Transactional email logs only: EMAIL_PROVIDER is not resend")
	return logsender.New(log)
}

// ProvideEmailDispatcher starts the asynchronous sender and drains it on shutdown. Callers hand mail to
// Dispatch after their own transaction commits and never see a send error.
func ProvideEmailDispatcher(cfg *config.Config, sender email.Sender, log *logger.Logger) (*email.Dispatcher, func()) {
	dispatcher := email.NewDispatcher(sender, email.DispatcherOptions{
		Workers:     cfg.Email.Workers,
		QueueSize:   cfg.Email.QueueSize,
		MaxAttempts: cfg.Email.MaxAttempts,
		BaseBackoff: cfg.Email.Backoff,
		Logger:      log,
	})
	return dispatcher, func() {
		ctx, cancel := context.WithTimeout(context.Background(), emailDrainTimeout)
		defer cancel()
		if err := dispatcher.Close(ctx); err != nil {
			log.Warn("Queued email was not fully sent before shutdown")
		}
	}
}
