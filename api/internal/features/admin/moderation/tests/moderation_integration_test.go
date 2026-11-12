//go:build integration

// Package tests holds the slow, opt-in tests of Job Posting moderation. The approval guard and the
// JD columns live in SQL, so only a real database can prove them. They need a migrated PostgreSQL
// database:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/admin/moderation/tests -count=1
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/email"
	"github.com/cog-forge/rolecue/api/internal/pkg/email/logsender"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return ctx, pool
}

// seed inserts rows and deletes exactly those rows afterwards, in reverse order.
type seed struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	undo []func()
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	return s
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) later(sql string, args ...any) {
	s.undo = append(s.undo, func() {
		if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	})
}

func (s *seed) recruiter() uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, 'mod recruiter', $2, 'recruiter', true, 'Acme')`, id, fmt.Sprintf("mod-%s@example.test", id))
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

// submittedPosting adds a pending posting that is waiting for review, with one core question. A nil
// contentPath stores NULL, as a posting created from a Role Profile draft does.
func (s *seed) submittedPosting(recruiterID uuid.UUID, contentPath, rawText *string) uuid.UUID {
	s.t.Helper()
	var levelID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	jdID, id := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, raw_text, level_id) VALUES ($1, $2, $3, $4)`,
		jdID, contentPath, rawText, levelID)
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	s.exec(`INSERT INTO public.core_questions (jd_id, content, rubric_id, expected_answer) VALUES ($1, 'question', $2, 'Key points of a strong answer.')`, jdID, s.rubric(recruiterID))
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jdID)
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, review_submitted)
		VALUES ($1, 'pending', $2, $3, 'moderation posting', true)`, id, jdID, recruiterID)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	return id
}

func (s *seed) interviewSlot(postingID uuid.UUID) *int32 {
	s.t.Helper()
	var slot *int32
	if err := s.pool.QueryRow(s.ctx, `SELECT interview_slot FROM public.job_postings WHERE id = $1`, postingID).Scan(&slot); err != nil {
		s.t.Fatal(err)
	}
	return slot
}

// refusingTexts fails the way object storage does for an empty key, so a request that still tries to
// sign or read the empty path fails the test.
type refusingTexts struct{}

func (refusingTexts) Download(_ context.Context, key string) (storage.Object, error) {
	return storage.Object{}, fmt.Errorf("unexpected download of %q", key)
}

func (refusingTexts) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	if key == "" {
		return storage.SignedRequest{}, errors.New("empty storage key")
	}
	return storage.SignedRequest{URL: "https://storage.example.test/" + key, Method: "GET", ExpiresAt: time.Now().Add(ttl)}, nil
}

func newService(t *testing.T, pool *pgxpool.Pool) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), refusingTexts{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// Approval moves the posting to open and nothing else: it holds no interview capacity until its
// Recruiter funds it, and the Admin still sees it as open.
func TestApprovedPostingIsOpenWithoutCapacityForTheAdmin(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	text := "Senior Go engineer."
	postingID := s.submittedPosting(s.recruiter(), nil, &text)

	approved, err := svc.Approve(ctx, uuid.New(), "admin", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != domain.PostingStatusOpen || approved.InterviewSlot != nil {
		t.Fatalf("approved = status %s slot %v, want open with no slot", approved.Status, approved.InterviewSlot)
	}
	if slot := s.interviewSlot(postingID); slot != nil {
		t.Fatalf("stored interview_slot = %d, want NULL after approval", *slot)
	}
	got, err := svc.Get(ctx, "admin", postingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.PostingStatusOpen || got.InterviewSlot != nil {
		t.Fatalf("admin detail = status %s slot %v, want open with no slot", got.Status, got.InterviewSlot)
	}
	status := domain.PostingStatusOpen
	items, _, err := svc.List(ctx, "admin", domain.ListFilter{Status: &status, RecruiterID: &got.Recruiter.ID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != postingID || items[0].Status != domain.PostingStatusOpen {
		t.Fatalf("admin list = %+v, want the approved posting as open", items)
	}
}

// content_path is nullable. A posting that holds its JD text but no file reviews fine: the Admin gets
// the text and no document link, not a failed signing.
func TestAdminDetailOfAPostingWithTextButNoContentPath(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	text := "JD text without a file."
	postingID := s.submittedPosting(s.recruiter(), nil, &text)

	got, err := svc.Get(ctx, "admin", postingID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.RawText == nil || *got.RawText != text {
		t.Fatalf("raw_text = %v, want %q", got.RawText, text)
	}
	if got.DocumentURL != nil || got.DocumentExpiresAt != nil {
		t.Fatalf("document_url=%v expires=%v, want no document link", got.DocumentURL, got.DocumentExpiresAt)
	}
}

// CommittedNoticeSender verifies that a separate connection sees the committed status before
// delivery, then forwards to the real log-only adapter. No external email is sent.
type committedNoticeSender struct {
	pool     *pgxpool.Pool
	id       uuid.UUID
	expected string
	next     email.Sender
	calls    int
}

func (s *committedNoticeSender) Send(ctx context.Context, msg email.Message) error {
	var status string
	if err := s.pool.QueryRow(ctx, "SELECT status::text FROM public.job_postings WHERE id=$1", s.id).Scan(&status); err != nil {
		return err
	}
	if status != s.expected {
		return email.Permanent(errors.New("decision not committed"))
	}
	s.calls++
	return s.next.Send(ctx, msg)
}

func TestModerationNoticeAfterCommit(t *testing.T) {
	ctx, pool := openPool(t)
	seed := newSeed(t, ctx, pool)
	recruiter := seed.recruiter()
	for _, status := range []string{"open", "rejected"} {
		t.Run(status, func(t *testing.T) {
			text := "Stored job description"
			id := seed.submittedPosting(recruiter, nil, &text)
			core, logs := observer.New(zap.InfoLevel)
			log := &logger.Logger{Logger: zap.New(core)}
			sender := &committedNoticeSender{pool: pool, id: id, expected: status, next: logsender.New(log)}
			dispatcher := email.NewDispatcher(sender, email.DispatcherOptions{MaxAttempts: 3, Logger: log})
			svc, err := service.New(repository.NewRepository(pool), refusingTexts{}, log)
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.SetNotices(dispatcher, "https://rolecue.test", "approved-template", "rejected-template"); err != nil {
				t.Fatal(err)
			}
			decide := func() {
				var err error
				if status == "open" {
					_, err = svc.Approve(ctx, uuid.New(), "admin", id)
				} else {
					_, err = svc.Reject(ctx, uuid.New(), "admin", id, "Add salary")
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			decide()
			decide()
			if err := dispatcher.Close(ctx); err != nil {
				t.Fatal(err)
			}
			if sender.calls != 1 || logs.FilterMessage("email sent").Len() != 1 {
				t.Fatal("expected one log-only delivery after commit")
			}
		})
	}
}

// rubric seeds a rubric owned by ownerID with one criterion. Tests use their own rubric, never the seeded
// system templates, which other packages' tests archive and restore while they run in parallel.
func (s *seed) rubric(ownerID uuid.UUID) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.rubrics (id, name, description, question_type, scope, owner_id) VALUES ($1, 'test rubric', 'd', 'concept_explanation', 'recruiter', $2)`, id, ownerID)
	s.later(`DELETE FROM public.rubrics WHERE id = $1`, id)
	s.exec(`INSERT INTO public.rubric_criteria (rubric_id, metric_id, name, max_points, scoring_guide, position)
		SELECT $1, m.id, 'c', 3, '0 = no; 3 = yes', row_number() OVER (ORDER BY m.id) FROM public.metrics m`, id)
	return id
}
