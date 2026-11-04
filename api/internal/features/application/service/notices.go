package service

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// noticeLookupTimeout bounds the one context read a notice needs, so a slow database never holds a
// decision open for long.
const noticeLookupTimeout = 5 * time.Second

// noticeDeadlineLayout is how a candidate reads the interview deadline, always in UTC.
const noticeDeadlineLayout = "2006-01-02 15:04 UTC"

// MailDispatcher enqueues without waiting for the provider. The email Dispatcher satisfies it.
type MailDispatcher interface{ Dispatch(string, email.Message) }

// NoticeTemplates names the provider-hosted template of each Application notice. An empty ID skips
// that notice.
type NoticeTemplates struct {
	Submitted, NewApplication, CVRejected, CVApproved, DecisionApproved, DecisionRejected, AutoRejected string
}

// NoticeRepository is the optional read of the people and posting a notice is about. The Repository
// implementation provides it; the service asserts it at call time.
type NoticeRepository interface {
	NoticeContexts(ctx context.Context, ids []uuid.UUID) ([]domain.NoticeContext, error)
}

// noticeEvent names one Application notice. It also forms the dispatch label.
type noticeEvent string

const (
	noticeSubmitted        noticeEvent = "submitted"
	noticeNewApplication   noticeEvent = "new_application"
	noticeCVRejected       noticeEvent = "cv_rejected"
	noticeCVApproved       noticeEvent = "cv_approved"
	noticeDecisionApproved noticeEvent = "decision_approved"
	noticeDecisionRejected noticeEvent = "decision_rejected"
	noticeAutoRejected     noticeEvent = "auto_rejected"
)

type notices struct {
	dispatcher MailDispatcher
	origin     string
	templates  NoticeTemplates
}

// SetNotices binds the dispatcher and the hosted templates at startup. No email copy is rendered here.
func (s *Service) SetNotices(dispatcher MailDispatcher, origin string, t NoticeTemplates) error {
	if dispatcher == nil {
		return fmt.Errorf("application notice dispatcher is required")
	}
	for name, id := range map[string]string{"submitted": t.Submitted, "new_application": t.NewApplication, "cv_rejected": t.CVRejected,
		"cv_approved": t.CVApproved, "decision_approved": t.DecisionApproved, "decision_rejected": t.DecisionRejected, "auto_rejected": t.AutoRejected} {
		if strings.TrimSpace(id) == "" {
			logger.FromContext(context.Background()).Warn("application notice disabled: no template configured", logger.Op("application.notices"), zap.String("event", name))
		}
	}
	s.notices = &notices{dispatcher: dispatcher, origin: strings.TrimRight(origin, "/"), templates: t}
	return nil
}

// template returns the hosted template ID of an event, or an empty string when it has none.
func (t NoticeTemplates) template(event noticeEvent) string {
	switch event {
	case noticeSubmitted:
		return t.Submitted
	case noticeNewApplication:
		return t.NewApplication
	case noticeCVRejected:
		return t.CVRejected
	case noticeCVApproved:
		return t.CVApproved
	case noticeDecisionApproved:
		return t.DecisionApproved
	case noticeDecisionRejected:
		return t.DecisionRejected
	case noticeAutoRejected:
		return t.AutoRejected
	}
	return ""
}

// notify sends the notice of event to each of ids. It runs after the repository call has succeeded and
// never returns an error or panics: a failed lookup is logged and the decision stands.
func (s *Service) notify(ctx context.Context, event noticeEvent, ids ...uuid.UUID) {
	if len(ids) == 0 {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.FromContext(ctx).Warn("application notice panicked",
				logger.Op("application.notify"), logger.String("event", string(event)), logger.Any("panic", r))
		}
	}()
	if s.notices == nil || s.notices.templates.template(event) == "" {
		return
	}
	repository, ok := s.repository.(NoticeRepository)
	if !ok {
		return
	}
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), noticeLookupTimeout)
	defer cancel()
	contexts, err := repository.NoticeContexts(lookupCtx, ids)
	if err != nil {
		logger.FromContext(ctx).Warn("application notice lookup failed",
			logger.Op("application.notify"), logger.String("event", string(event)), logger.Cause(err))
		return
	}
	for _, nc := range contexts {
		msg, ok := s.notices.message(event, nc)
		if !ok {
			continue
		}
		s.notices.dispatcher.Dispatch("application:"+nc.ApplicationID.String()+":"+string(event), msg)
	}
}

// message builds the notice of event for one Application. It returns false when there is nothing to
// send. The variables carry the posting title, the Candidate's name, a link and (for a CV approval) the
// interview deadline. Reject reasons, CVs, transcripts, recordings, scores and storage links never go in.
func (n *notices) message(event noticeEvent, nc domain.NoticeContext) (email.Message, bool) {
	to := nc.CandidateEmail
	link := n.origin + "/applications/" + nc.ApplicationID.String()
	if event == noticeNewApplication {
		to = nc.RecruiterEmail
		link = n.origin + "/recruiter/job-postings/" + nc.JobPostingID.String() + "/applications/" + nc.ApplicationID.String()
	}
	if to == "" {
		return email.Message{}, false
	}
	variables := map[string]string{
		"POSTING_TITLE":   html.EscapeString(nc.PostingTitle),
		"CANDIDATE_NAME":  html.EscapeString(nc.CandidateName),
		"APPLICATION_URL": html.EscapeString(link),
	}
	if event == noticeCVApproved {
		if nc.InterviewDeadline == nil {
			return email.Message{}, false
		}
		variables["INTERVIEW_DEADLINE"] = html.EscapeString(nc.InterviewDeadline.UTC().Format(noticeDeadlineLayout))
	}
	return email.Message{
		ID:       uuid.NewString(),
		To:       to,
		Template: &email.HostedTemplate{ID: n.templates.template(event), Variables: variables},
	}, true
}
