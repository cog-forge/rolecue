package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/google/uuid"
)

// noticeRecorder keeps every dispatched label and message in order.
type noticeRecorder struct {
	labels   []string
	messages []email.Message
}

func (r *noticeRecorder) Dispatch(label string, m email.Message) {
	r.labels = append(r.labels, label)
	r.messages = append(r.messages, m)
}

// noticeFake is the shared fake of every trigger: the screening and intake reads, the Candidate and
// decision writes, the expiry pass and the notice lookup. NoticeContexts answers only the Applications
// it is asked about, as the SQL does.
type noticeFake struct {
	*fakeRepository

	submission domain.Submission
	insertErr  error
	checkErr   error
	decideErr  error
	decisions  []domain.Decision

	expiredIDs []uuid.UUID
	expireErr  error

	contexts  []domain.NoticeContext
	lookupErr error
	lookups   [][]uuid.UUID
}

func (f *noticeFake) CheckIntakeFunded(_ context.Context, _ uuid.UUID) error { return f.checkErr }

func (f *noticeFake) InsertCandidateApplication(_ context.Context, _, _ uuid.UUID, _ string) (domain.Submission, error) {
	if f.insertErr != nil {
		return domain.Submission{}, f.insertErr
	}
	return f.submission, nil
}

func (f *noticeFake) ListCandidateApplications(_ context.Context, _ uuid.UUID, _, _ int32) ([]domain.CandidateApplication, int64, error) {
	return nil, 0, nil
}

func (f *noticeFake) GetCandidateApplication(_ context.Context, _, _ uuid.UUID) (domain.CandidateApplication, error) {
	return domain.CandidateApplication{}, nil
}

func (f *noticeFake) DecideApplication(_ context.Context, _, _, _ uuid.UUID, decision domain.Decision, _ *string) error {
	f.decisions = append(f.decisions, decision)
	return f.decideErr
}

func (f *noticeFake) AutoRejectExpiredIDs(_ context.Context, _ time.Time) ([]uuid.UUID, error) {
	if f.expireErr != nil {
		return nil, f.expireErr
	}
	return f.expiredIDs, nil
}

func (f *noticeFake) NoticeContexts(_ context.Context, ids []uuid.UUID) ([]domain.NoticeContext, error) {
	f.lookups = append(f.lookups, slices.Clone(ids))
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	var out []domain.NoticeContext
	for _, nc := range f.contexts {
		if slices.Contains(ids, nc.ApplicationID) {
			out = append(out, nc)
		}
	}
	return out, nil
}

var (
	noticeAppA = uuid.MustParse("55555555-5555-4555-8555-555555555555")
	noticeAppB = uuid.MustParse("66666666-6666-4666-8666-666666666666")
	noticeAppC = uuid.MustParse("77777777-7777-4777-8777-777777777777")
	// noticeDeadline is 09:30 in UTC+7, which is 02:30 UTC.
	noticeDeadline = time.Date(2026, 11, 6, 9, 30, 0, 0, time.FixedZone("ICT", 7*3600))
)

func noticeContextFor(app uuid.UUID, candidateEmail string, deadline *time.Time) domain.NoticeContext {
	return domain.NoticeContext{
		ApplicationID: app, JobPostingID: postingID, PostingTitle: "Backend Engineer",
		CandidateName: "Ada Lovelace", CandidateEmail: candidateEmail, RecruiterEmail: "owner@example.test",
		InterviewDeadline: deadline,
	}
}

func fullNoticeTemplates() NoticeTemplates {
	return NoticeTemplates{
		Submitted: "tpl-submitted", NewApplication: "tpl-new", CVRejected: "tpl-cv-rejected", CVApproved: "tpl-cv-approved",
		DecisionApproved: "tpl-decision-approved", DecisionRejected: "tpl-decision-rejected", AutoRejected: "tpl-auto-rejected", IntakeClosed: "tpl-intake-closed",
	}
}

// newNoticeService builds a Service on the fake with the notices bound. The CV store is the in-memory
// one, so Apply can upload.
func newNoticeService(t *testing.T, repo *noticeFake, d MailDispatcher, tpl NoticeTemplates) *Service {
	t.Helper()
	s, err := New(repo, newApplyCVStore())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := s.SetNotices(d, "https://rolecue.test/", tpl); err != nil {
		t.Fatalf("SetNotices() error = %v", err)
	}
	return s
}

// defaultNoticeContexts are the three Applications the triggers name. Each has its own Candidate email
// so a test can tell which recipient got which notice.
func defaultNoticeContexts() []domain.NoticeContext {
	return []domain.NoticeContext{
		noticeContextFor(noticeAppA, "ada@example.test", &noticeDeadline),
		noticeContextFor(noticeAppB, "bob@example.test", &noticeDeadline),
		noticeContextFor(noticeAppC, "cara@example.test", &noticeDeadline),
	}
}

type wantNotice struct {
	app      uuid.UUID
	event    noticeEvent
	to       string
	template string
}

func assertNotices(t *testing.T, rec *noticeRecorder, want []wantNotice) {
	t.Helper()
	if len(rec.messages) != len(want) {
		t.Fatalf("dispatched %d notices %v, want %d", len(rec.messages), rec.labels, len(want))
	}
	for i, w := range want {
		wantLabel := "application:" + w.app.String() + ":" + string(w.event)
		if rec.labels[i] != wantLabel {
			t.Fatalf("notice %d label = %q, want %q", i, rec.labels[i], wantLabel)
		}
		m := rec.messages[i]
		if m.ID == "" || m.Subject != "" || m.Body != "" || m.Template == nil {
			t.Fatalf("notice %d must be a hosted template with an ID: %+v", i, m)
		}
		if m.To != w.to || m.Template.ID != w.template {
			t.Fatalf("notice %d to %q template %q, want to %q template %q", i, m.To, m.Template.ID, w.to, w.template)
		}
	}
}

func TestNoticeTriggersRouteEachEventToItsRecipient(t *testing.T) {
	rejectReason := "Missing Go experience"
	rows := []struct {
		name      string
		contexts  []domain.NoticeContext
		templates func(*NoticeTemplates)
		setup     func(f *noticeFake)
		run       func(s *Service) error
		want      []wantNotice
	}{
		{
			name: "apply notifies the candidate and the recruiter",
			setup: func(f *noticeFake) {
				f.submission = domain.Submission{ID: noticeAppA, JobPostingID: postingID, Status: domain.StatusPending}
			},
			run: func(s *Service) error {
				_, err := s.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
				return err
			},
			want: []wantNotice{
				{noticeAppA, noticeSubmitted, "ada@example.test", "tpl-submitted"},
				{noticeAppA, noticeNewApplication, "owner@example.test", "tpl-new"},
			},
		},
		{
			name: "approving a CV notifies the candidate",
			run: func(s *Service) error {
				_, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA)
				return err
			},
			want: []wantNotice{{noticeAppA, noticeCVApproved, "ada@example.test", "tpl-cv-approved"}},
		},
		{
			name: "approving a CV that fills the slots auto-rejects each displaced application",
			setup: func(f *noticeFake) {
				f.screening = domain.Screening{ID: noticeAppA, Status: domain.StatusInterviewEligible, RejectedIDs: []uuid.UUID{noticeAppB, noticeAppC}}
			},
			run: func(s *Service) error {
				_, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA)
				return err
			},
			want: []wantNotice{
				{noticeAppA, noticeCVApproved, "ada@example.test", "tpl-cv-approved"},
				{noticeAppB, noticeIntakeClosed, "bob@example.test", "tpl-intake-closed"},
				{noticeAppC, noticeIntakeClosed, "cara@example.test", "tpl-intake-closed"},
			},
		},
		{
			name: "rejecting a CV notifies the candidate",
			run: func(s *Service) error {
				_, err := s.RejectCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, &rejectReason)
				return err
			},
			want: []wantNotice{{noticeAppA, noticeCVRejected, "ada@example.test", "tpl-cv-rejected"}},
		},
		{
			name: "an approved decision notifies the candidate",
			run: func(s *Service) error {
				_, err := s.Decide(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, domain.DecisionApprove, nil)
				return err
			},
			want: []wantNotice{{noticeAppA, noticeDecisionApproved, "ada@example.test", "tpl-decision-approved"}},
		},
		{
			name: "a rejected decision notifies the candidate",
			run: func(s *Service) error {
				_, err := s.Decide(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, domain.DecisionReject, &rejectReason)
				return err
			},
			want: []wantNotice{{noticeAppA, noticeDecisionRejected, "ada@example.test", "tpl-decision-rejected"}},
		},
		{
			name: "closing intake sends the intake-closed notice to each rejected application",
			setup: func(f *noticeFake) {
				f.closure = domain.IntakeClosure{JobPostingID: postingID, Status: "intake_closed", RejectedCount: 2, RejectedIDs: []uuid.UUID{noticeAppA, noticeAppB}}
			},
			run: func(s *Service) error {
				_, err := s.CloseIntake(context.Background(), recruiterID, recruiterRole, postingID)
				return err
			},
			want: []wantNotice{
				{noticeAppA, noticeIntakeClosed, "ada@example.test", "tpl-intake-closed"},
				{noticeAppB, noticeIntakeClosed, "bob@example.test", "tpl-intake-closed"},
			},
		},
		{
			name: "the expiry pass auto-rejects each expired application",
			setup: func(f *noticeFake) {
				f.expiredIDs = []uuid.UUID{noticeAppA, noticeAppB}
			},
			run: func(s *Service) error {
				_, err := s.AutoRejectExpired(context.Background(), time.Now())
				return err
			},
			want: []wantNotice{
				{noticeAppA, noticeAutoRejected, "ada@example.test", "tpl-auto-rejected"},
				{noticeAppB, noticeAutoRejected, "bob@example.test", "tpl-auto-rejected"},
			},
		},
		{
			name:     "an approval without an interview deadline sends nothing",
			contexts: []domain.NoticeContext{noticeContextFor(noticeAppA, "ada@example.test", nil)},
			run: func(s *Service) error {
				_, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA)
				return err
			},
		},
		{
			name:      "an unset template skips its notice",
			templates: func(t *NoticeTemplates) { t.CVRejected = "" },
			run: func(s *Service) error {
				_, err := s.RejectCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, nil)
				return err
			},
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			repo := &noticeFake{fakeRepository: &fakeRepository{}, contexts: defaultNoticeContexts()}
			if row.contexts != nil {
				repo.contexts = row.contexts
			}
			if row.setup != nil {
				row.setup(repo)
			}
			tpl := fullNoticeTemplates()
			if row.templates != nil {
				row.templates(&tpl)
			}
			rec := &noticeRecorder{}
			s := newNoticeService(t, repo, rec, tpl)
			if err := row.run(s); err != nil {
				t.Fatalf("trigger error = %v", err)
			}
			assertNotices(t, rec, row.want)
		})
	}
}

func TestNoticeLookupFailureNeverFailsTheTrigger(t *testing.T) {
	rows := []struct {
		name string
		run  func(s *Service) error
	}{
		{"apply", func(s *Service) error {
			_, err := s.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
			return err
		}},
		{"approve CV", func(s *Service) error {
			_, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA)
			return err
		}},
		{"decide", func(s *Service) error {
			_, err := s.Decide(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, domain.DecisionApprove, nil)
			return err
		}},
		{"expiry pass", func(s *Service) error {
			_, err := s.AutoRejectExpired(context.Background(), time.Now())
			return err
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			repo := &noticeFake{
				fakeRepository: &fakeRepository{},
				submission:     domain.Submission{ID: noticeAppA, JobPostingID: postingID, Status: domain.StatusPending},
				expiredIDs:     []uuid.UUID{noticeAppA},
				contexts:       defaultNoticeContexts(),
				lookupErr:      errors.New("database unavailable"),
			}
			rec := &noticeRecorder{}
			s := newNoticeService(t, repo, rec, fullNoticeTemplates())
			if err := row.run(s); err != nil {
				t.Fatalf("notice lookup failure changed the result: %v", err)
			}
			if len(rec.messages) != 0 {
				t.Fatalf("dispatched %d notices after a failed lookup", len(rec.messages))
			}
			if len(repo.lookups) == 0 {
				t.Fatal("the notice lookup was never attempted")
			}
		})
	}
}

func TestNoticesAreSentOnlyAfterTheWriteSucceeds(t *testing.T) {
	rows := []struct {
		name string
		fail func(f *noticeFake)
		run  func(s *Service) error
	}{
		{"apply", func(f *noticeFake) { f.insertErr = errors.New("insert failed") }, func(s *Service) error {
			_, err := s.Apply(context.Background(), candidateID, candidateRole, postingID, validCV())
			return err
		}},
		{"approve CV", func(f *noticeFake) { f.err = errors.New("slot lost") }, func(s *Service) error {
			_, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA)
			return err
		}},
		{"reject CV", func(f *noticeFake) { f.err = errors.New("row locked") }, func(s *Service) error {
			_, err := s.RejectCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, nil)
			return err
		}},
		{"decide", func(f *noticeFake) { f.decideErr = errors.New("gate closed") }, func(s *Service) error {
			_, err := s.Decide(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, domain.DecisionReject, nil)
			return err
		}},
		{"close intake", func(f *noticeFake) { f.err = errors.New("posting missing") }, func(s *Service) error {
			_, err := s.CloseIntake(context.Background(), recruiterID, recruiterRole, postingID)
			return err
		}},
		{"expiry pass", func(f *noticeFake) { f.expireErr = errors.New("advisory lock") }, func(s *Service) error {
			_, err := s.AutoRejectExpired(context.Background(), time.Now())
			return err
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			repo := &noticeFake{fakeRepository: &fakeRepository{}, contexts: defaultNoticeContexts()}
			row.fail(repo)
			rec := &noticeRecorder{}
			s := newNoticeService(t, repo, rec, fullNoticeTemplates())
			if err := row.run(s); err == nil {
				t.Fatal("expected the write error")
			}
			if len(rec.messages) != 0 || len(repo.lookups) != 0 {
				t.Fatalf("failed write notified: %d messages, %d lookups", len(rec.messages), len(repo.lookups))
			}
		})
	}
}

func TestNoticeVariablesNeverCarryRejectReasonOrPrivateData(t *testing.T) {
	repo := &noticeFake{fakeRepository: &fakeRepository{}, contexts: []domain.NoticeContext{
		{ApplicationID: noticeAppA, JobPostingID: postingID, PostingTitle: "<Backend & Go>", CandidateName: "Ada <Lovelace>",
			CandidateEmail: "ada@example.test", RecruiterEmail: "owner@example.test", InterviewDeadline: &noticeDeadline},
	}}
	rec := &noticeRecorder{}
	s := newNoticeService(t, repo, rec, fullNoticeTemplates())
	secret := "Salary secret 9000"
	if _, err := s.RejectCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA, &secret); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA); err != nil {
		t.Fatal(err)
	}
	for _, m := range rec.messages {
		for key, value := range m.Template.Variables {
			if strings.Contains(value, "secret") || strings.Contains(value, "9000") {
				t.Fatalf("variable %s carries the reject reason: %q", key, value)
			}
			if strings.Contains(key, "REJECT") || strings.Contains(key, "CV") || strings.Contains(key, "SCORE") {
				t.Fatalf("unexpected private variable %s", key)
			}
		}
	}
	title := rec.messages[0].Template.Variables["POSTING_TITLE"]
	if title != "&lt;Backend &amp; Go&gt;" || rec.messages[0].Template.Variables["CANDIDATE_NAME"] != "Ada &lt;Lovelace&gt;" {
		t.Fatalf("unescaped variables: title %q", title)
	}
	if got := rec.messages[0].Template.Variables["APPLICATION_URL"]; got != "https://rolecue.test/applications/"+noticeAppA.String() {
		t.Fatalf("candidate link = %q", got)
	}
}

func TestRecruiterNoticeLinksToTheRecruiterApplication(t *testing.T) {
	repo := &noticeFake{fakeRepository: &fakeRepository{}, contexts: defaultNoticeContexts(),
		submission: domain.Submission{ID: noticeAppA, JobPostingID: postingID, Status: domain.StatusPending}}
	rec := &noticeRecorder{}
	s := newNoticeService(t, repo, rec, fullNoticeTemplates())
	if _, err := s.Apply(context.Background(), candidateID, candidateRole, postingID, validCV()); err != nil {
		t.Fatal(err)
	}
	want := "https://rolecue.test/recruiter/job-postings/" + postingID.String() + "/applications/" + noticeAppA.String()
	if got := rec.messages[1].Template.Variables["APPLICATION_URL"]; got != want {
		t.Fatalf("recruiter link = %q, want %q", got, want)
	}
}

func TestCVApprovedStatesTheDeadlineInUTC(t *testing.T) {
	repo := &noticeFake{fakeRepository: &fakeRepository{}, contexts: defaultNoticeContexts()}
	rec := &noticeRecorder{}
	s := newNoticeService(t, repo, rec, fullNoticeTemplates())
	if _, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA); err != nil {
		t.Fatal(err)
	}
	if len(rec.messages) != 1 {
		t.Fatalf("dispatched %d notices, want 1", len(rec.messages))
	}
	if got := rec.messages[0].Template.Variables["INTERVIEW_DEADLINE"]; got != "2026-11-06 02:30 UTC" {
		t.Fatalf("INTERVIEW_DEADLINE = %q, want the UTC deadline", got)
	}
}

func TestSetNoticesRequiresDispatcherAndTrimsOrigin(t *testing.T) {
	s, err := New(&fakeRepository{}, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetNotices(nil, "https://rolecue.test", fullNoticeTemplates()); err == nil {
		t.Fatal("SetNotices(nil) error = nil, want an error")
	}
	if err := s.SetNotices(&noticeRecorder{}, "https://rolecue.test//", fullNoticeTemplates()); err != nil {
		t.Fatal(err)
	}
	if s.notices.origin != "https://rolecue.test" {
		t.Fatalf("origin = %q, want the trailing slashes trimmed", s.notices.origin)
	}
}

func TestNoticesStayOffUntilConfigured(t *testing.T) {
	repo := &noticeFake{fakeRepository: &fakeRepository{}, contexts: defaultNoticeContexts()}
	s, err := New(repo, newApplyCVStore())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveCV(context.Background(), recruiterID, recruiterRole, postingID, noticeAppA); err != nil {
		t.Fatal(err)
	}
	if len(repo.lookups) != 0 {
		t.Fatal("notice lookup ran without notices configured")
	}
}
