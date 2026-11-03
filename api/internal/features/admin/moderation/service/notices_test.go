package service

import (
	"context"
	"errors"
	"html"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/google/uuid"
)

type noticeRecorder struct{ messages []email.Message }

func (r *noticeRecorder) Dispatch(_ string, m email.Message) { r.messages = append(r.messages, m) }

func TestDecisionNotices(t *testing.T) {
	for _, approve := range []bool{true, false} {
		t.Run(map[bool]string{true: "approve", false: "reject"}[approve], func(t *testing.T) {
			repo := postingIn(domain.PostingStatusPending)
			repo.posting.Title = "<Engineer>"
			repo.posting.Recruiter.Email = "owner@example.test"
			s := newService(t, repo)
			recorder := &noticeRecorder{}
			if err := s.SetNotices(recorder, "https://rolecue.test/", "approved-template", "rejected-template"); err != nil {
				t.Fatal(err)
			}
			decide := func() {
				var err error
				if approve {
					_, err = s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID)
				} else {
					_, err = s.Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, "<Add salary>")
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			decide()
			decide()
			if len(recorder.messages) != 1 {
				t.Fatal("duplicate dispatch")
			}
			m := recorder.messages[0]
			if m.To != repo.posting.Recruiter.Email || m.ID == "" || m.Template == nil || m.Template.Variables["POSTING_TITLE"] != "&lt;Engineer&gt;" || m.Template.Variables["POSTING_URL"] != "https://rolecue.test/recruiter/job-postings/"+repo.posting.ID.String() {
				t.Fatal("wrong notice data")
			}
			if approve {
				if m.Template.ID != "approved-template" || m.Subject != "" || m.Body != "" {
					t.Fatal("approval must reference hosted content only")
				}
			} else {
				if m.Template.ID != "rejected-template" || m.Template.Variables["REJECT_REASON_1"] != "&lt;Add salary&gt;" {
					t.Fatal("missing rejection data")
				}
				repo.posting.Status = domain.PostingStatusPending
				repo.posting.ReviewSubmitted = true
				decide()
				if len(recorder.messages) != 2 || recorder.messages[1].ID == m.ID {
					t.Fatal("resubmission needs a new notice")
				}
			}
		})
	}
}

type failingNoticeSender struct {
	calls int
	ids   []string
}

func (s *failingNoticeSender) Send(_ context.Context, m email.Message) error {
	s.calls++
	s.ids = append(s.ids, m.ID)
	return errors.New("provider unavailable")
}

func TestNoticeFailureNeverRollsBackDecision(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	s := newService(t, repo)
	sender := &failingNoticeSender{}
	dispatcher := email.NewDispatcher(sender, email.DispatcherOptions{MaxAttempts: 3})
	if err := s.SetNotices(dispatcher, "https://rolecue.test", "approved-template", "rejected-template"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := dispatcher.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if repo.posting.Status != domain.PostingStatusOpen || sender.calls != 3 {
		t.Fatal("decision changed or retry count incorrect")
	}
	for _, id := range sender.ids {
		if id != sender.ids[0] {
			t.Fatal("retry changed idempotency key")
		}
	}
}

type blockedNoticeSender struct{ started, release chan struct{} }

func (s blockedNoticeSender) Send(ctx context.Context, _ email.Message) error {
	close(s.started)
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func TestSlowNoticeDoesNotBlockDecision(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	s := newService(t, repo)
	sender := blockedNoticeSender{make(chan struct{}), make(chan struct{})}
	dispatcher := email.NewDispatcher(sender, email.DispatcherOptions{})
	if err := s.SetNotices(dispatcher, "https://rolecue.test", "approved-template", "rejected-template"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Error("decision blocked by sender")
	}
	close(sender.release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := dispatcher.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestFailedDecisionDoesNotDispatch(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	repo.decideErr = errors.New("database unavailable")
	s := newService(t, repo)
	recorder := &noticeRecorder{}
	if err := s.SetNotices(recorder, "https://rolecue.test", "approved-template", "rejected-template"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(context.Background(), uuid.New(), "admin", repo.posting.ID); err == nil {
		t.Fatal("expected decision failure")
	}
	if len(recorder.messages) != 0 {
		t.Fatal("uncommitted decision dispatched")
	}
}

func TestHostedRejectionPreservesMaximumEscapedReason(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	s := newService(t, repo)
	recorder := &noticeRecorder{}
	if err := s.SetNotices(recorder, "https://rolecue.test", "approved", "rejected"); err != nil {
		t.Fatal(err)
	}
	reason := strings.Repeat("'&<界", 250)
	if _, err := s.Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, reason); err != nil {
		t.Fatal(err)
	}
	variables := recorder.messages[0].Template.Variables
	var joined strings.Builder
	for _, key := range []string{"REJECT_REASON_1", "REJECT_REASON_2", "REJECT_REASON_3", "REJECT_REASON_4"} {
		value, ok := variables[key]
		if !ok || len(value) > 2000 {
			t.Fatal("missing variable or provider length limit exceeded")
		}
		joined.WriteString(value)
	}
	if html.UnescapeString(joined.String()) != reason {
		t.Fatal("reason was truncated or changed")
	}
}
