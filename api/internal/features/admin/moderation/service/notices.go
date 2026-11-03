package service

import (
	"fmt"
	"html"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/google/uuid"
)

// MailDispatcher enqueues without waiting for the provider.
type MailDispatcher interface{ Dispatch(string, email.Message) }

type Notices struct {
	dispatcher             MailDispatcher
	origin                 string
	approvedID, rejectedID string
}

// SetNotices binds provider-hosted templates at startup. No email copy or layout is rendered here.
func (s *Service) SetNotices(dispatcher MailDispatcher, origin, approvedID, rejectedID string) error {
	if dispatcher == nil || strings.TrimSpace(approvedID) == "" || strings.TrimSpace(rejectedID) == "" {
		return fmt.Errorf("moderation email dispatcher and both template IDs are required")
	}
	s.notices = &Notices{dispatcher: dispatcher, origin: strings.TrimRight(origin, "/"), approvedID: approvedID, rejectedID: rejectedID}
	return nil
}

func (n *Notices) send(posting domain.ReviewPosting, status domain.PostingStatus, reason *string) {
	variables := map[string]string{
		"POSTING_TITLE": html.EscapeString(posting.Title),
		"POSTING_URL":   n.origin + "/recruiter/job-postings/" + posting.ID.String(),
	}
	templateID := n.approvedID
	if status == domain.PostingStatusRejected {
		templateID = n.rejectedID
		// Resend limits a string variable to 2,000 characters. A valid 1,000-rune reason
		// can expand after HTML escaping; four 250-rune chunks preserve it without truncation.
		var runes []rune
		if reason != nil {
			runes = []rune(*reason)
		}
		for i, key := range []string{"REJECT_REASON_1", "REJECT_REASON_2", "REJECT_REASON_3", "REJECT_REASON_4"} {
			start, end := min(i*250, len(runes)), min((i+1)*250, len(runes))
			variables[key] = html.EscapeString(string(runes[start:end]))
		}
	}
	// Only a successful SQL transition gets here. A resubmission creates a new message, while
	// dispatcher retries reuse the same ID and template variables.
	msg := email.Message{
		ID:       uuid.NewString(),
		To:       posting.Recruiter.Email,
		Template: &email.HostedTemplate{ID: templateID, Variables: variables},
	}
	n.dispatcher.Dispatch("moderation:"+posting.ID.String()+":"+string(status), msg)
}
