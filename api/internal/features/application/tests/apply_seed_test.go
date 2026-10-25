//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeCVStore stands in for object storage. It records every upload and delete, so a test can tell
// whether a CV object was left behind. It is safe for concurrent use.
type fakeCVStore struct {
	mu        sync.Mutex
	uploads   []string
	deletes   []string
	uploadErr error
}

func (f *fakeCVStore) Upload(_ context.Context, body io.ReadSeeker, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	if _, err := io.Copy(io.Discard, body); err != nil {
		return "", err
	}
	key := fmt.Sprintf("cvs/%s.pdf", uuid.New())
	f.uploads = append(f.uploads, key)
	return key, nil
}

func (f *fakeCVStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, key)
	return nil
}

// PresignGet satisfies the CV link signer the service also needs. Apply never calls it.
func (f *fakeCVStore) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{URL: "https://storage.example.test/signed?sig=apply", Method: http.MethodGet, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (f *fakeCVStore) uploadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.uploads)
}

func (f *fakeCVStore) deleted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deletes...)
}

// newApplyService builds the service with the real repository and the given fake CV store.
func newApplyService(t *testing.T, pool *pgxpool.Pool, store *fakeCVStore) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), store)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// pdfBytes returns a minimal payload that passes the CV check: the %PDF- prefix and a body of n bytes.
func pdfBytes(n int) []byte {
	out := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), n)...)
	return out
}

// lockRecruiter marks a Recruiter as locked, which hides their postings from applicants.
func (s *seed) lockRecruiter(recruiterID uuid.UUID) {
	s.exec(`UPDATE public.users SET is_locked = true, lock_reason = 'test', lock_expires_at = NULL WHERE id = $1`, recruiterID)
	s.later(`UPDATE public.users SET is_locked = false, lock_reason = NULL WHERE id = $1`, recruiterID)
}

// applicationRow reads the stored Application that a Candidate created, for the columns the service
// never returns (the CV storage key and the candidate).
type applicationRow struct {
	status      string
	cvPath      *string
	candidateID uuid.UUID
	postingID   uuid.UUID
}

func (s *seed) applicationRowFor(t *testing.T, postingID, candidateID uuid.UUID) (applicationRow, int) {
	t.Helper()
	var row applicationRow
	var n int
	if err := s.pool.QueryRow(s.ctx,
		`SELECT count(*) FROM public.applications WHERE job_posting_id = $1 AND candidate_id = $2`,
		postingID, candidateID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		return row, 0
	}
	if err := s.pool.QueryRow(s.ctx,
		`SELECT status::text, cv_path, candidate_id, job_posting_id FROM public.applications
		 WHERE job_posting_id = $1 AND candidate_id = $2 ORDER BY created_at LIMIT 1`,
		postingID, candidateID).Scan(&row.status, &row.cvPath, &row.candidateID, &row.postingID); err != nil {
		t.Fatal(err)
	}
	return row, n
}

// applicationCount counts every Application row in the database for the given posting.
func applicationCount(t *testing.T, s *seed, postingID uuid.UUID) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, `SELECT count(*) FROM public.applications WHERE job_posting_id = $1`, postingID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// marshalJSON returns the serialized body a client would receive, for leak checks.
func marshalJSON(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// setStatusAndDeadline sets an Application's stored status and interview deadline, for the
// can_start_interview scenarios. The deadline may be nil.
func setStatusAndDeadline(s *seed, applicationID uuid.UUID, status string, deadline *time.Time) {
	s.exec(`UPDATE public.applications SET status = $2::public.application_status, interview_deadline = $3 WHERE id = $1`,
		applicationID, status, deadline)
}
