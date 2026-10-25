package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// fakeRecordingRepository returns the configured recording state and records the writes it receives.
type fakeRecordingRepository struct {
	state        domain.RecordingState
	stateErr     error
	uploadOK     bool
	uploadKey    string
	uploadCalls  int
	resultOK     bool
	resultStatus domain.RecordStatus
	resultKey    string
	resultCalls  int
	owned        bool
	ownedCalls   int
	turns        []domain.TranscriptTurn
	turnsCalls   int
}

func (r *fakeRecordingRepository) GetCandidateRecording(_ context.Context, _, _ uuid.UUID) (domain.RecordingState, error) {
	return r.state, r.stateErr
}

func (r *fakeRecordingRepository) SetRecordingUpload(_ context.Context, _, _ uuid.UUID, key string) (bool, error) {
	r.uploadCalls++
	r.uploadKey = key
	return r.uploadOK, nil
}

func (r *fakeRecordingRepository) SetRecordingResult(_ context.Context, _, _ uuid.UUID, key string, status domain.RecordStatus) (bool, error) {
	r.resultCalls++
	r.resultKey = key
	r.resultStatus = status
	return r.resultOK, nil
}

func (r *fakeRecordingRepository) RecruiterOwnsApplication(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
	r.ownedCalls++
	return r.owned, nil
}

func (r *fakeRecordingRepository) GetApplicationRecording(_ context.Context, _ uuid.UUID) (domain.RecordingState, error) {
	return r.state, r.stateErr
}

func (r *fakeRecordingRepository) ListTranscript(_ context.Context, _ uuid.UUID) ([]domain.TranscriptTurn, error) {
	r.turnsCalls++
	return r.turns, nil
}

// trackingBody records whether the caller closed the object body.
type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

// fakeRecordingStore returns fixed grants and objects and counts the presigned reads.
type fakeRecordingStore struct {
	grantKey     string
	putCalls     int
	putSize      int64
	putTTL       time.Duration
	putErr       error
	getCalls     int
	getTTL       time.Duration
	object       storage.Object
	downloadErr  error
	downloadKeys []string
}

func (s *fakeRecordingStore) PresignPut(_ context.Context, _ string, size int64, ttl time.Duration) (storage.UploadGrant, error) {
	s.putCalls++
	s.putSize = size
	s.putTTL = ttl
	if s.putErr != nil {
		return storage.UploadGrant{}, s.putErr
	}
	return storage.UploadGrant{
		Key: s.grantKey,
		Request: storage.SignedRequest{
			URL:       "https://storage.example.test/put?signature=abc",
			Method:    "PUT",
			Headers:   map[string]string{"Content-Type": "video/webm"},
			ExpiresAt: time.Date(2026, 10, 25, 12, 15, 0, 0, time.UTC),
		},
	}, nil
}

func (s *fakeRecordingStore) PresignGet(_ context.Context, key string, ttl time.Duration) (storage.SignedRequest, error) {
	s.getCalls++
	s.getTTL = ttl
	return storage.SignedRequest{
		URL:       "https://storage.example.test/get/" + key,
		Method:    "GET",
		ExpiresAt: time.Date(2026, 10, 25, 12, 5, 0, 0, time.UTC),
	}, nil
}

func (s *fakeRecordingStore) Download(_ context.Context, key string) (storage.Object, error) {
	s.downloadKeys = append(s.downloadKeys, key)
	if s.downloadErr != nil {
		return storage.Object{}, s.downloadErr
	}
	return s.object, nil
}

const testRecordingKey = "3f1c2a4e-0000-4000-8000-000000000001"

func recordingPath(key string) *string { return &key }

func newTestRecordingService(t *testing.T, repo *fakeRecordingRepository, store *fakeRecordingStore) *RecordingService {
	t.Helper()
	svc, err := NewRecordingService(repo, store)
	if err != nil {
		t.Fatalf("new recording service: %v", err)
	}
	return svc
}

func validUpload() domain.RecordingUploadInput {
	return domain.RecordingUploadInput{ContentType: "video/webm", Size: 1024}
}

func recruitmentRecordingState(status domain.RecordStatus, path *string) domain.RecordingState {
	return domain.RecordingState{InterviewID: uuid.New(), Type: domain.TypeRecruitment, RecordStatus: status, RecordPath: path}
}

func TestNewRecordingServiceRequiresDependencies(t *testing.T) {
	if _, err := NewRecordingService(nil, &fakeRecordingStore{}); err == nil {
		t.Fatal("expected an error without a repository")
	}
	if _, err := NewRecordingService(&fakeRecordingRepository{}, nil); err == nil {
		t.Fatal("expected an error without a store")
	}
}

func TestGrantUploadRequiresCandidate(t *testing.T) {
	repo := &fakeRecordingRepository{}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.GrantUpload(context.Background(), uuid.New(), "recruiter", uuid.New(), validUpload())
	assertCode(t, err, apperror.CodeForbidden)
	if store.putCalls != 0 || repo.uploadCalls != 0 {
		t.Fatal("a non-candidate must not reach storage or the repository")
	}
}

func TestGrantUploadValidatesContentTypeAndSize(t *testing.T) {
	cases := map[string]domain.RecordingUploadInput{
		"unknown type": {ContentType: "video/quicktime", Size: 1024},
		"empty type":   {ContentType: "", Size: 1024},
		"zero size":    {ContentType: "video/webm", Size: 0},
		"negative":     {ContentType: "video/webm", Size: -1},
		"too large":    {ContentType: "video/webm", Size: domain.RecordingMaxBytes + 1},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRecordingRepository{}
			store := &fakeRecordingStore{grantKey: testRecordingKey}
			svc := newTestRecordingService(t, repo, store)
			_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), input)
			assertCode(t, err, apperror.CodeValidation)
			if store.putCalls != 0 {
				t.Fatal("invalid input must not presign an upload")
			}
		})
	}
}

func TestGrantUploadRefusesPracticeInterview(t *testing.T) {
	repo := &fakeRecordingRepository{state: domain.RecordingState{Type: domain.TypePractice, RecordStatus: domain.RecordNone}}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	assertCode(t, err, apperror.CodeRecordingNotAllowed)
	if store.putCalls != 0 || repo.uploadCalls != 0 {
		t.Fatal("a practice interview must not get a grant")
	}
}

func TestGrantUploadRefusesProcessingAndReadyRecordings(t *testing.T) {
	for _, status := range []domain.RecordStatus{domain.RecordProcessing, domain.RecordReady} {
		t.Run(string(status), func(t *testing.T) {
			repo := &fakeRecordingRepository{state: recruitmentRecordingState(status, recordingPath("old-key"))}
			store := &fakeRecordingStore{grantKey: testRecordingKey}
			svc := newTestRecordingService(t, repo, store)
			_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
			assertCode(t, err, apperror.CodeRecordingNotAllowed)
			if store.putCalls != 0 || repo.uploadCalls != 0 {
				t.Fatal("a processing or ready recording must not be replaced")
			}
		})
	}
}

func TestGrantUploadStoresKeyBeforeReturningGrant(t *testing.T) {
	repo := &fakeRecordingRepository{state: recruitmentRecordingState(domain.RecordNone, nil), uploadOK: true}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)

	grant, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	if err != nil {
		t.Fatalf("grant upload: %v", err)
	}
	if repo.uploadKey != testRecordingKey {
		t.Fatalf("stored key = %q, want the granted key", repo.uploadKey)
	}
	if store.putSize != 1024 || store.putTTL != domain.RecordingUploadTTL {
		t.Fatalf("presign size/ttl = %d/%s, want 1024/%s", store.putSize, store.putTTL, domain.RecordingUploadTTL)
	}
	if grant.UploadURL != "https://storage.example.test/put?signature=abc" || grant.Method != "PUT" {
		t.Fatalf("grant request = %q %q", grant.Method, grant.UploadURL)
	}
	if grant.RecordStatus != domain.RecordUploading || grant.Headers["Content-Type"] != "video/webm" {
		t.Fatalf("grant = %+v", grant)
	}
}

func TestGrantUploadNeverExposesKeyOrReadLink(t *testing.T) {
	repo := &fakeRecordingRepository{state: recruitmentRecordingState(domain.RecordFailed, recordingPath("old-key")), uploadOK: true}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)

	grant, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	if err != nil {
		t.Fatalf("grant upload: %v", err)
	}
	body, err := json.Marshal(grant)
	if err != nil {
		t.Fatalf("marshal grant: %v", err)
	}
	if strings.Contains(string(body), testRecordingKey) || strings.Contains(string(body), "record_path") {
		t.Fatalf("grant leaks the storage key: %s", body)
	}
	if strings.Contains(string(body), "/get/") {
		t.Fatalf("grant leaks a read link: %s", body)
	}
}

func TestGrantUploadReportsLostRace(t *testing.T) {
	repo := &fakeRecordingRepository{state: recruitmentRecordingState(domain.RecordNone, nil), uploadOK: false}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	assertCode(t, err, apperror.CodeRecordingNotAllowed)
}

func TestGrantUploadPassesOwnershipMissThrough(t *testing.T) {
	repo := &fakeRecordingRepository{stateErr: domain.ErrInterviewNotFound}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	assertCode(t, err, apperror.CodeInterviewNotFound)
	if store.putCalls != 0 {
		t.Fatal("a missing interview must not presign an upload")
	}
}

func TestCompleteUploadMarksPresentObjectReady(t *testing.T) {
	body := &trackingBody{Reader: bytes.NewReader([]byte("webm-bytes"))}
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{object: storage.Object{Body: body, Size: 10}}
	svc := newTestRecordingService(t, repo, store)

	status, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if status.RecordStatus != domain.RecordReady {
		t.Fatalf("status = %s, want ready", status.RecordStatus)
	}
	if repo.resultKey != testRecordingKey || repo.resultStatus != domain.RecordReady {
		t.Fatalf("result write = %q %s", repo.resultKey, repo.resultStatus)
	}
	if !body.closed {
		t.Fatal("the object body must be closed")
	}
	body2, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if strings.Contains(string(body2), testRecordingKey) {
		t.Fatalf("status leaks the storage key: %s", body2)
	}
}

func TestCompleteUploadMarksMissingObjectFailed(t *testing.T) {
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{downloadErr: storage.ErrNotFound}
	svc := newTestRecordingService(t, repo, store)

	_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeRecordingNotFound)
	if repo.resultStatus != domain.RecordFailed || repo.resultKey != testRecordingKey {
		t.Fatalf("missing object must mark the upload failed, got %q %s", repo.resultKey, repo.resultStatus)
	}
}

func TestCompleteUploadRejectsEmptyObject(t *testing.T) {
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{object: storage.Object{Body: &trackingBody{Reader: strings.NewReader("")}, Size: 0}}
	svc := newTestRecordingService(t, repo, store)

	_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeValidation)
	if repo.resultStatus != domain.RecordFailed {
		t.Fatalf("empty object must mark the upload failed, got %s", repo.resultStatus)
	}
}

func TestCompleteUploadIsIdempotentWhenReady(t *testing.T) {
	repo := &fakeRecordingRepository{state: recruitmentRecordingState(domain.RecordReady, recordingPath(testRecordingKey))}
	store := &fakeRecordingStore{}
	svc := newTestRecordingService(t, repo, store)

	status, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	if err != nil {
		t.Fatalf("complete ready upload: %v", err)
	}
	if status.RecordStatus != domain.RecordReady {
		t.Fatalf("status = %s, want ready", status.RecordStatus)
	}
	if len(store.downloadKeys) != 0 || repo.resultCalls != 0 {
		t.Fatal("a ready recording must not be verified or written again")
	}
}

func TestCompleteUploadRequiresAnUploadInProgress(t *testing.T) {
	cases := map[string]domain.RecordingState{
		"no grant yet": recruitmentRecordingState(domain.RecordNone, nil),
		"failed":       recruitmentRecordingState(domain.RecordFailed, recordingPath(testRecordingKey)),
		"practice":     {Type: domain.TypePractice, RecordStatus: domain.RecordUploading, RecordPath: recordingPath(testRecordingKey)},
	}
	for name, state := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRecordingRepository{state: state}
			store := &fakeRecordingStore{}
			svc := newTestRecordingService(t, repo, store)
			_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
			assertCode(t, err, apperror.CodeRecordingNotAllowed)
			if len(store.downloadKeys) != 0 {
				t.Fatal("no object must be verified without an upload in progress")
			}
		})
	}
}

func TestCompleteUploadRequiresCandidate(t *testing.T) {
	repo := &fakeRecordingRepository{}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	_, err := svc.CompleteUpload(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeForbidden)
}

func TestRecruiterRecordingIssuesLinkOnlyWhenReady(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, state: recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey))}
	store := &fakeRecordingStore{}
	svc := newTestRecordingService(t, repo, store)

	recording, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("recording: %v", err)
	}
	if recording.RecordStatus != domain.RecordUploading || recording.URL != nil || recording.ExpiresAt != nil {
		t.Fatalf("uploading recording must carry no link: %+v", recording)
	}
	if store.getCalls != 0 {
		t.Fatal("no read link may be presigned before the recording is ready")
	}
}

func TestRecruiterRecordingPresignsReadyLinkForFiveMinutes(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, state: recruitmentRecordingState(domain.RecordReady, recordingPath(testRecordingKey))}
	store := &fakeRecordingStore{}
	svc := newTestRecordingService(t, repo, store)

	recording, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("recording: %v", err)
	}
	if recording.URL == nil || !strings.Contains(*recording.URL, testRecordingKey) {
		t.Fatalf("ready recording must carry a link: %+v", recording)
	}
	if recording.ExpiresAt == nil || store.getTTL != domain.RecordingReadTTL {
		t.Fatalf("link ttl = %s, want %s", store.getTTL, domain.RecordingReadTTL)
	}
}

func TestRecordingIsHiddenFromOtherRolesAndRecruiters(t *testing.T) {
	cases := map[string]struct {
		role  string
		owned bool
	}{
		"candidate":       {role: "candidate", owned: true},
		"admin":           {role: "admin", owned: true},
		"other recruiter": {role: "recruiter", owned: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRecordingRepository{owned: tc.owned, state: recruitmentRecordingState(domain.RecordReady, recordingPath(testRecordingKey))}
			store := &fakeRecordingStore{}
			svc := newTestRecordingService(t, repo, store)
			_, err := svc.Recording(context.Background(), uuid.New(), tc.role, uuid.New(), uuid.New())
			assertCode(t, err, apperror.CodeApplicationNotFound)
			_, err = svc.Transcript(context.Background(), uuid.New(), tc.role, uuid.New(), uuid.New())
			assertCode(t, err, apperror.CodeApplicationNotFound)
			if store.getCalls != 0 || repo.turnsCalls != 0 {
				t.Fatal("a caller without ownership must not reach the link or the transcript")
			}
		})
	}
}

func TestRecruiterRecordingWithoutInterviewIsNotFound(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, stateErr: domain.ErrInterviewNotFound}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	_, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeInterviewNotFound)
}

func TestTranscriptRequiresCompletedInterview(t *testing.T) {
	state := recruitmentRecordingState(domain.RecordNone, nil)
	state.Completed = false
	repo := &fakeRecordingRepository{owned: true, state: state}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	_, err := svc.Transcript(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeInterviewNotCompleted)
	if repo.turnsCalls != 0 {
		t.Fatal("an unfinished interview must not return turns")
	}
}

func TestTranscriptReturnsTurnsInOrderAndEmptyListAsArray(t *testing.T) {
	answer := "I led a team of four."
	state := recruitmentRecordingState(domain.RecordNone, nil)
	state.Completed = true
	repo := &fakeRecordingRepository{owned: true, state: state, turns: []domain.TranscriptTurn{
		{Position: 1, Question: "Tell me about yourself.", Answer: &answer},
		{Position: 2, Question: "Why this role?", Answer: nil},
	}}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	transcript, err := svc.Transcript(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	if len(transcript.Turns) != 2 || transcript.Turns[0].Position != 1 || transcript.Turns[1].Answer != nil {
		t.Fatalf("turns = %+v", transcript.Turns)
	}

	repo.turns = nil
	transcript, err = svc.Transcript(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("transcript with no turns: %v", err)
	}
	encoded, err := json.Marshal(transcript)
	if err != nil {
		t.Fatalf("marshal transcript: %v", err)
	}
	if string(encoded) != `{"turns":[]}` {
		t.Fatalf("empty transcript = %s, want an empty array", encoded)
	}
}

func TestCompleteUploadPassesRepositoryErrorsThrough(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRecordingRepository{stateErr: dbErr}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want the repository error", err)
	}
}
