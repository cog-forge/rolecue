//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeRecordingStore stands in for object storage. PresignPut hands out a fresh key the way the real
// store does; the test marks an object present to play the Candidate's PUT. Its signed URLs never embed
// the key, so a leak check on a response can only find the key if the service returned it.
type fakeRecordingStore struct {
	mu      sync.Mutex
	keys    []string
	objects map[string]int64
}

func newFakeRecordingStore() *fakeRecordingStore {
	return &fakeRecordingStore{objects: map[string]int64{}}
}

func (f *fakeRecordingStore) PresignPut(_ context.Context, contentType string, _ int64, ttl time.Duration) (storage.UploadGrant, error) {
	key := "recordings/" + uuid.NewString() + ".webm"
	f.mu.Lock()
	f.keys = append(f.keys, key)
	f.mu.Unlock()
	return storage.UploadGrant{
		Key: key,
		Request: storage.SignedRequest{
			URL:       "https://storage.example.test/upload/" + uuid.NewString(),
			Method:    http.MethodPut,
			Headers:   map[string]string{"Content-Type": contentType},
			ExpiresAt: time.Now().Add(ttl),
		},
	}, nil
}

func (f *fakeRecordingStore) PresignGet(_ context.Context, _ string, ttl time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{
		URL:       "https://storage.example.test/read/" + uuid.NewString(),
		Method:    http.MethodGet,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (f *fakeRecordingStore) Download(_ context.Context, key string) (storage.Object, error) {
	f.mu.Lock()
	size, ok := f.objects[key]
	f.mu.Unlock()
	if !ok {
		return storage.Object{}, fmt.Errorf("download %s: %w", key, storage.ErrNotFound)
	}
	return storage.Object{Body: io.NopCloser(strings.NewReader("recording")), ContentType: "video/webm", Size: size}, nil
}

// lastKey returns the key of the most recent upload grant.
func (f *fakeRecordingStore) lastKey(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.keys) == 0 {
		t.Fatal("no upload grant was issued")
	}
	return f.keys[len(f.keys)-1]
}

// putObject marks an object present, as the Candidate's upload would.
func (f *fakeRecordingStore) putObject(key string, size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = size
}

// newRecordingService builds the recording service over the real repository and the fake store.
func newRecordingService(t *testing.T, pool *pgxpool.Pool, store *fakeRecordingStore) *service.RecordingService {
	t.Helper()
	svc, err := service.NewRecordingService(repository.NewRepository(pool), store)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// recordingFixture is a Candidate with a recruitment session for an eligible Application of a posting.
type recordingFixture struct {
	recruiter   uuid.UUID
	candidate   uuid.UUID
	posting     uuid.UUID
	application uuid.UUID
	session     domain.Session
}

func newRecordingFixture(t *testing.T, ctx context.Context, s *seed, svc *service.Service) recordingFixture {
	t.Helper()
	recruiter, posting, _ := s.openPosting(slotOf(2), 4)
	candidate := s.user("candidate")
	s.confirmedProfile(candidate, 1)
	application := s.eligibleApplication(posting, candidate)
	session, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil || !created {
		t.Fatalf("recruitment Create() created=%v error=%v", created, err)
	}
	return recordingFixture{recruiter: recruiter, candidate: candidate, posting: posting, application: application, session: session}
}

// recordingRow reads the recording columns of an interview.
func (s *seed) recordingRow(interviewID uuid.UUID) (status string, path *string) {
	s.t.Helper()
	if err := s.pool.QueryRow(s.ctx,
		`SELECT record_status, record_path FROM public.interviews WHERE id = $1`, interviewID).Scan(&status, &path); err != nil {
		s.t.Fatal(err)
	}
	return status, path
}

// jsonOf marshals a response the way the handler would send it.
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

var validUpload = domain.RecordingUploadInput{ContentType: "video/webm", Size: 1024}

// TestRecordingGrantIsRecruitmentOnly proves a practice session never gets an upload grant: it is refused
// before any storage call, and its recording stays none.
func TestRecordingGrantIsRecruitmentOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	practice, _ := practiceSession(t, ctx, s, svc, candidate, 4)

	_, err := recording.GrantUpload(ctx, candidate, "candidate", practice.ID, validUpload)
	assertCode(t, err, apperror.CodeRecordingNotAllowed)
	if len(store.keys) != 0 {
		t.Fatalf("storage was asked for %d grants for a practice session, want none", len(store.keys))
	}
	if status, _ := s.recordingRow(practice.ID); status != "none" {
		t.Fatalf("practice record_status = %s, want none", status)
	}
}

// TestRecordingGrantStoresKeyAndNeverReturnsIt proves the grant stores the key on the interview and marks
// it uploading, and the response carries the upload target but no storage key.
func TestRecordingGrantStoresKeyAndNeverReturnsIt(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	grant, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload)
	if err != nil {
		t.Fatalf("GrantUpload() error = %v", err)
	}
	if grant.RecordStatus != domain.RecordUploading || grant.Method != http.MethodPut || grant.UploadURL == "" {
		t.Fatalf("grant = %+v, want an uploading PUT target", grant)
	}
	if grant.ExpiresAt.Before(time.Now().Add(14 * time.Minute)) {
		t.Fatalf("grant expires at %v, want about 15 minutes ahead", grant.ExpiresAt)
	}
	key := store.lastKey(t)
	status, path := s.recordingRow(fx.session.ID)
	if status != "uploading" || path == nil || *path != key {
		t.Fatalf("recording row = %s %v, want uploading with the granted key", status, path)
	}
	if body := jsonOf(t, grant); strings.Contains(body, key) || strings.Contains(body, "record_path") {
		t.Fatalf("grant response leaks the storage key: %s", body)
	}
}

// TestRecordingFinalizeWithPresentObjectIsReady proves a present upload is verified and the recording
// becomes ready. The Recruiter then gets a read link, and the Candidate's status response has no key.
func TestRecordingFinalizeWithPresentObjectIsReady(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload); err != nil {
		t.Fatalf("GrantUpload() error = %v", err)
	}
	key := store.lastKey(t)
	store.putObject(key, 1024)

	status, err := recording.CompleteUpload(ctx, fx.candidate, "candidate", fx.session.ID)
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if status.RecordStatus != domain.RecordReady {
		t.Fatalf("status = %s, want ready", status.RecordStatus)
	}
	if row, _ := s.recordingRow(fx.session.ID); row != "ready" {
		t.Fatalf("stored record_status = %s, want ready", row)
	}
	if body := jsonOf(t, status); strings.Contains(body, key) {
		t.Fatalf("complete response leaks the storage key: %s", body)
	}

	evidence, err := recording.Recording(ctx, fx.recruiter, "recruiter", fx.posting, fx.application)
	if err != nil {
		t.Fatalf("Recording() error = %v", err)
	}
	if evidence.RecordStatus != domain.RecordReady || evidence.URL == nil || *evidence.URL == "" || evidence.ExpiresAt == nil {
		t.Fatalf("recruiter recording = %+v, want ready with a read link", evidence)
	}
	if strings.Contains(*evidence.URL, key) {
		t.Fatal("read link embeds the storage key")
	}
}

// TestRecordingFinalizeMissingObjectFailsThenRegrant proves a missing object marks the recording failed
// and reports RECORDING_NOT_FOUND, and the Candidate can grant a new upload and finish it.
func TestRecordingFinalizeMissingObjectFailsThenRegrant(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload); err != nil {
		t.Fatalf("GrantUpload() error = %v", err)
	}
	firstKey := store.lastKey(t)

	_, err := recording.CompleteUpload(ctx, fx.candidate, "candidate", fx.session.ID)
	assertCode(t, err, apperror.CodeRecordingNotFound)
	if status, _ := s.recordingRow(fx.session.ID); status != "failed" {
		t.Fatalf("record_status after a missing object = %s, want failed", status)
	}

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload); err != nil {
		t.Fatalf("regrant after a failed upload error = %v", err)
	}
	secondKey := store.lastKey(t)
	if secondKey == firstKey {
		t.Fatal("regrant reused the failed key")
	}
	store.putObject(secondKey, 1024)
	if _, err := recording.CompleteUpload(ctx, fx.candidate, "candidate", fx.session.ID); err != nil {
		t.Fatalf("CompleteUpload() after regrant error = %v", err)
	}
	if status, path := s.recordingRow(fx.session.ID); status != "ready" || path == nil || *path != secondKey {
		t.Fatalf("recording row = %s %v, want ready on the second key", status, path)
	}
}

// TestRecordingReadyCannotBeReplaced proves a ready recording keeps its object: a new grant is refused and
// the stored key does not move.
func TestRecordingReadyCannotBeReplaced(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload); err != nil {
		t.Fatalf("GrantUpload() error = %v", err)
	}
	key := store.lastKey(t)
	store.putObject(key, 1024)
	if _, err := recording.CompleteUpload(ctx, fx.candidate, "candidate", fx.session.ID); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	_, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload)
	assertCode(t, err, apperror.CodeRecordingNotAllowed)
	if status, path := s.recordingRow(fx.session.ID); status != "ready" || path == nil || *path != key {
		t.Fatalf("recording row = %s %v, want ready on the original key", status, path)
	}
}

// TestRecordingUploadRejectsBadInput proves a media type outside the list and a size outside 1 byte to
// 500 MiB are refused before any storage call.
func TestRecordingUploadRejectsBadInput(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	cases := []domain.RecordingUploadInput{
		{ContentType: "image/png", Size: 1024},
		{ContentType: "video/webm", Size: 0},
		{ContentType: "video/webm", Size: domain.RecordingMaxBytes + 1},
	}
	for _, input := range cases {
		_, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, input)
		assertCode(t, err, apperror.CodeValidation)
	}
	if len(store.keys) != 0 {
		t.Fatalf("storage was asked for %d grants for invalid input, want none", len(store.keys))
	}
}

// TestRecordingIsOnlyForTheOwningCandidate proves another Candidate neither grants nor completes an
// upload for the interview: both are not found.
func TestRecordingIsOnlyForTheOwningCandidate(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)
	intruder := s.user("candidate")

	_, err := recording.GrantUpload(ctx, intruder, "candidate", fx.session.ID, validUpload)
	assertCode(t, err, apperror.CodeInterviewNotFound)
	_, err = recording.CompleteUpload(ctx, intruder, "candidate", fx.session.ID)
	assertCode(t, err, apperror.CodeInterviewNotFound)
	if len(store.keys) != 0 {
		t.Fatalf("storage was asked for a grant by a non-owner")
	}
}

// TestRecruiterRecordingAccessIsOwnerOnly proves only the owning Recruiter reads the recording and the
// transcript. The other Recruiter, a Candidate, an Admin and a wrong posting all get APPLICATION_NOT_FOUND.
func TestRecruiterRecordingAccessIsOwnerOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)
	otherRecruiter := s.user("recruiter")

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload); err != nil {
		t.Fatalf("GrantUpload() error = %v", err)
	}
	store.putObject(store.lastKey(t), 1024)
	if _, err := recording.CompleteUpload(ctx, fx.candidate, "candidate", fx.session.ID); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	if _, err := recording.Recording(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); err != nil {
		t.Fatalf("owner Recording() error = %v", err)
	}
	if _, err := recording.Transcript(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); codeOf(err) != apperror.CodeInterviewNotCompleted {
		t.Fatalf("owner Transcript() of an open interview = %v, want INTERVIEW_NOT_COMPLETED", err)
	}

	denied := []struct {
		name string
		call func() error
	}{
		{"other recruiter recording", func() error {
			_, err := recording.Recording(ctx, otherRecruiter, "recruiter", fx.posting, fx.application)
			return err
		}},
		{"other recruiter transcript", func() error {
			_, err := recording.Transcript(ctx, otherRecruiter, "recruiter", fx.posting, fx.application)
			return err
		}},
		{"candidate recording", func() error {
			_, err := recording.Recording(ctx, fx.candidate, "candidate", fx.posting, fx.application)
			return err
		}},
		{"admin recording", func() error {
			_, err := recording.Recording(ctx, fx.recruiter, "admin", fx.posting, fx.application)
			return err
		}},
		{"owner with a wrong posting", func() error {
			_, err := recording.Recording(ctx, fx.recruiter, "recruiter", uuid.New(), fx.application)
			return err
		}},
	}
	for _, tc := range denied {
		if err := tc.call(); codeOf(err) != apperror.CodeApplicationNotFound {
			t.Fatalf("%s: error = %v, want APPLICATION_NOT_FOUND", tc.name, err)
		}
	}
}

// TestRecruiterSeesNoLinkBeforeReady proves a recording that is still uploading has no read link, and an
// Application with no interview at all is INTERVIEW_NOT_FOUND for its Recruiter.
func TestRecruiterSeesNoLinkBeforeReady(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, validUpload); err != nil {
		t.Fatalf("GrantUpload() error = %v", err)
	}
	evidence, err := recording.Recording(ctx, fx.recruiter, "recruiter", fx.posting, fx.application)
	if err != nil {
		t.Fatalf("Recording() error = %v", err)
	}
	if evidence.RecordStatus != domain.RecordUploading || evidence.URL != nil || evidence.ExpiresAt != nil {
		t.Fatalf("uploading recording = %+v, want no read link", evidence)
	}

	_, bare, _ := s.openPosting(slotOf(2), 4)
	bareApp := s.eligibleApplication(bare, fx.candidate)
	_, err = recording.Recording(ctx, s.recruiterOf(bare), "recruiter", bare, bareApp)
	assertCode(t, err, apperror.CodeInterviewNotFound)
}

// recruiterOf returns the recruiter of a posting, read from the table.
func (s *seed) recruiterOf(postingID uuid.UUID) uuid.UUID {
	s.t.Helper()
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT recruiter_id FROM public.job_postings WHERE id = $1`, postingID).Scan(&id); err != nil {
		s.t.Fatal(err)
	}
	return id
}

// TestTranscriptOnlyAfterCompletion proves the Recruiter gets the transcript of a completed recruitment
// interview, in position order, with the question as asked and each answer. Before completion it is refused.
func TestTranscriptOnlyAfterCompletion(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)

	if _, err := svc.AppendTurn(ctx, fx.candidate, "candidate", fx.session.ID, 1, "answer one"); err != nil {
		t.Fatalf("AppendTurn(1) error = %v", err)
	}
	if _, err := recording.Transcript(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); codeOf(err) != apperror.CodeInterviewNotCompleted {
		t.Fatalf("Transcript() before completion = %v, want INTERVIEW_NOT_COMPLETED", err)
	}

	// Answers must arrive in position order, so the list is ordered rather than a map.
	for i, answer := range []string{"answer two", "answer three"} {
		pos := int32(i + 2)
		if _, err := svc.AppendTurn(ctx, fx.candidate, "candidate", fx.session.ID, pos, answer); err != nil {
			t.Fatalf("AppendTurn(%d) error = %v", pos, err)
		}
	}
	if _, err := svc.Complete(ctx, fx.candidate, "candidate", fx.session.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	transcript, err := recording.Transcript(ctx, fx.recruiter, "recruiter", fx.posting, fx.application)
	if err != nil {
		t.Fatalf("Transcript() after completion error = %v", err)
	}
	snap := s.snapshot(fx.session.ID)
	answers := []string{"answer one", "answer two", "answer three"}
	if len(transcript.Turns) != 3 {
		t.Fatalf("transcript turns = %d, want 3", len(transcript.Turns))
	}
	for i, turn := range transcript.Turns {
		if turn.Position != int32(i+1) || turn.Question != s.contentOf(snap[i]) || turn.Answer == nil || *turn.Answer != answers[i] {
			t.Fatalf("transcript turn %d = %+v, want position %d with its question and answer", i+1, turn, i+1)
		}
	}
}
