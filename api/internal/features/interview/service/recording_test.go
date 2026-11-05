package service

import (
	"context"
	"encoding/json"
	"errors"
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
	state            domain.RecordingState
	stateErr         error
	uploadOK         bool
	uploadKey        string
	uploadCalls      int
	uploadMaxGrants  int32
	resultOK         bool
	resultStatus     domain.RecordStatus
	resultKey        string
	resultETag       string
	resultCalls      int
	failChangedOK    bool
	failChangedKey   string
	failChangedCalls int
	owned            bool
	ownedCalls       int
	turns            []domain.TranscriptTurn
	turnsCalls       int
}

func (r *fakeRecordingRepository) GetCandidateRecording(_ context.Context, _, _ uuid.UUID) (domain.RecordingState, error) {
	return r.state, r.stateErr
}

func (r *fakeRecordingRepository) SetRecordingUpload(_ context.Context, _, _ uuid.UUID, key string, maxGrants int32) (bool, error) {
	r.uploadCalls++
	r.uploadKey = key
	r.uploadMaxGrants = maxGrants
	return r.uploadOK, nil
}

func (r *fakeRecordingRepository) SetRecordingResult(_ context.Context, _, _ uuid.UUID, key string, status domain.RecordStatus, etag string) (bool, error) {
	r.resultCalls++
	r.resultKey = key
	r.resultStatus = status
	r.resultETag = etag
	return r.resultOK, nil
}

func (r *fakeRecordingRepository) FailChangedRecording(_ context.Context, _ uuid.UUID, key string) (bool, error) {
	r.failChangedCalls++
	r.failChangedKey = key
	return r.failChangedOK, nil
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

// fakeRecordingStore returns fixed grants and object metadata, and counts the calls that reach storage.
type fakeRecordingStore struct {
	grantKey     string
	putCalls     int
	putSize      int64
	putTTL       time.Duration
	putErr       error
	getCalls     int
	getTTL       time.Duration
	getOverrides storage.GetOverrides
	headInfo     storage.ObjectInfo
	headErr      error
	headCalls    int
	prefix       []byte
	prefixErr    error
	prefixCalls  int
	deleted      []string
	deleteErr    error
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

func (s *fakeRecordingStore) PresignGetWithOverrides(_ context.Context, key string, ttl time.Duration, overrides storage.GetOverrides) (storage.SignedRequest, error) {
	s.getCalls++
	s.getTTL = ttl
	s.getOverrides = overrides
	return storage.SignedRequest{
		URL:       "https://storage.example.test/get/" + key,
		Method:    "GET",
		ExpiresAt: time.Date(2026, 10, 25, 12, 5, 0, 0, time.UTC),
	}, nil
}

func (s *fakeRecordingStore) Head(_ context.Context, _ string) (storage.ObjectInfo, error) {
	s.headCalls++
	if s.headErr != nil {
		return storage.ObjectInfo{}, s.headErr
	}
	return s.headInfo, nil
}

func (s *fakeRecordingStore) ReadPrefix(_ context.Context, _ string, _ int64) ([]byte, error) {
	s.prefixCalls++
	if s.prefixErr != nil {
		return nil, s.prefixErr
	}
	return s.prefix, nil
}

func (s *fakeRecordingStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.deleteErr
}

const testRecordingKey = "3f1c2a4e-0000-4000-8000-000000000001"

// webmHead is the first bytes of a WebM file: the EBML header.
var webmHead = []byte{0x1A, 0x45, 0xDF, 0xA3, 0x01, 0x00, 0x00, 0x00}

// webmInfo is the metadata of a stored WebM object with the given ETag.
func webmInfo(etag string) storage.ObjectInfo {
	return storage.ObjectInfo{ETag: etag, Size: 10, ContentType: "video/webm"}
}

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

// readyRecordingWithETag is a ready recruitment recording whose verified ETag is etag.
func readyRecordingWithETag(etag *string) domain.RecordingState {
	state := recruitmentRecordingState(domain.RecordReady, recordingPath(testRecordingKey))
	state.RecordETag = etag
	return state
}

func strPtr(s string) *string { return &s }

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
			if store.putCalls != 0 || repo.uploadCalls != 0 || len(store.deleted) != 0 {
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
	if len(store.deleted) != 0 {
		t.Fatalf("a first grant has no previous object to delete, deleted %v", store.deleted)
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

func TestGrantUploadEnforcesGrantCap(t *testing.T) {
	state := recruitmentRecordingState(domain.RecordFailed, recordingPath("old-key"))
	state.GrantCount = domain.RecordingMaxGrants
	repo := &fakeRecordingRepository{state: state, uploadOK: true}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	assertCode(t, err, apperror.CodeRecordingUploadLimit)
	if store.putCalls != 0 || repo.uploadCalls != 0 || len(store.deleted) != 0 {
		t.Fatal("an exhausted recording must not presign, store or delete anything")
	}
}

func TestGrantUploadPassesGrantCapToRepository(t *testing.T) {
	state := recruitmentRecordingState(domain.RecordFailed, nil)
	state.GrantCount = domain.RecordingMaxGrants - 1
	repo := &fakeRecordingRepository{state: state, uploadOK: true}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	if _, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload()); err != nil {
		t.Fatalf("the last allowed grant was refused: %v", err)
	}
	if repo.uploadMaxGrants != domain.RecordingMaxGrants {
		t.Fatalf("repository cap = %d, want %d", repo.uploadMaxGrants, domain.RecordingMaxGrants)
	}
}

func TestGrantUploadDeletesReplacedObject(t *testing.T) {
	repo := &fakeRecordingRepository{state: recruitmentRecordingState(domain.RecordFailed, recordingPath("old-key")), uploadOK: true}
	store := &fakeRecordingStore{grantKey: testRecordingKey}
	svc := newTestRecordingService(t, repo, store)
	if _, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload()); err != nil {
		t.Fatalf("grant upload: %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "old-key" {
		t.Fatalf("replaced object deletes = %v, want [old-key]", store.deleted)
	}
}

func TestGrantUploadSurvivesFailedCleanup(t *testing.T) {
	repo := &fakeRecordingRepository{state: recruitmentRecordingState(domain.RecordUploading, recordingPath("old-key")), uploadOK: true}
	store := &fakeRecordingStore{grantKey: testRecordingKey, deleteErr: errors.New("storage unavailable")}
	svc := newTestRecordingService(t, repo, store)
	grant, err := svc.GrantUpload(context.Background(), uuid.New(), "candidate", uuid.New(), validUpload())
	if err != nil {
		t.Fatalf("a failed cleanup must not refuse the grant: %v", err)
	}
	if grant.RecordStatus != domain.RecordUploading || repo.uploadKey != testRecordingKey {
		t.Fatalf("grant = %+v, stored key %q", grant, repo.uploadKey)
	}
}

func TestCompleteUploadMarksPresentObjectReady(t *testing.T) {
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{headInfo: webmInfo(`"v1"`), prefix: webmHead}
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
	if repo.resultETag != `"v1"` {
		t.Fatalf("stored ETag = %q, want the verified object's ETag", repo.resultETag)
	}
	body, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if strings.Contains(string(body), testRecordingKey) {
		t.Fatalf("status leaks the storage key: %s", body)
	}
}

func TestCompleteUploadMarksMissingObjectFailed(t *testing.T) {
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{headErr: storage.ErrNotFound}
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
	store := &fakeRecordingStore{headInfo: storage.ObjectInfo{ETag: `"v1"`, Size: 0, ContentType: "video/webm"}}
	svc := newTestRecordingService(t, repo, store)

	_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeValidation)
	if repo.resultStatus != domain.RecordFailed {
		t.Fatalf("empty object must mark the upload failed, got %s", repo.resultStatus)
	}
	if store.prefixCalls != 0 {
		t.Fatal("an empty object must be refused before its bytes are read")
	}
}

func TestCompleteUploadRejectsMediaThatDoesNotMatchItsType(t *testing.T) {
	mp4 := []byte("\x00\x00\x00\x18ftypmp42")
	cases := map[string]struct {
		contentType string
		head        []byte
	}{
		"webm declared, mp4 bytes":  {"video/webm", mp4},
		"mp4 declared, webm bytes":  {"video/mp4", webmHead},
		"audio declared, text":      {"audio/webm", []byte("plain text, not media")},
		"mp4 declared, too short":   {"video/mp4", []byte("ftyp")},
		"webm declared, empty head": {"video/webm", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRecordingRepository{
				state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
				resultOK: true,
			}
			store := &fakeRecordingStore{headInfo: storage.ObjectInfo{ETag: `"v1"`, Size: 100, ContentType: tc.contentType}, prefix: tc.head}
			svc := newTestRecordingService(t, repo, store)

			_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
			assertCode(t, err, apperror.CodeValidation)
			if repo.resultStatus != domain.RecordFailed || repo.resultETag != "" {
				t.Fatalf("mismatched media must fail without an ETag, got %s %q", repo.resultStatus, repo.resultETag)
			}
		})
	}
}

func TestCompleteUploadRejectsUnsupportedObjectType(t *testing.T) {
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{headInfo: storage.ObjectInfo{ETag: `"v1"`, Size: 100, ContentType: "application/octet-stream"}, prefix: webmHead}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeValidation)
	if repo.resultStatus != domain.RecordFailed || store.prefixCalls != 0 {
		t.Fatal("an unsupported type must fail before its bytes are read")
	}
}

func TestRecordingMatchesMagic(t *testing.T) {
	if !domain.RecordingMatchesMagic("video/webm", webmHead) || !domain.RecordingMatchesMagic("audio/webm", webmHead) {
		t.Fatal("WebM magic bytes rejected")
	}
	if !domain.RecordingMatchesMagic("video/mp4", []byte("\x00\x00\x00\x18ftypmp42")) {
		t.Fatal("MP4 ftyp box rejected")
	}
	if domain.RecordingMatchesMagic("video/mp4", webmHead) || domain.RecordingMatchesMagic("video/webm", []byte("\x00\x00\x00\x18ftyp")) {
		t.Fatal("media of another container accepted")
	}
}

func TestCompleteUploadIsIdempotentWhenReady(t *testing.T) {
	repo := &fakeRecordingRepository{state: readyRecordingWithETag(strPtr(`"v1"`))}
	store := &fakeRecordingStore{}
	svc := newTestRecordingService(t, repo, store)

	status, err := svc.CompleteUpload(context.Background(), uuid.New(), "candidate", uuid.New())
	if err != nil {
		t.Fatalf("complete ready upload: %v", err)
	}
	if status.RecordStatus != domain.RecordReady {
		t.Fatalf("status = %s, want ready", status.RecordStatus)
	}
	if store.headCalls != 0 || repo.resultCalls != 0 {
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
			if store.headCalls != 0 {
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
	if store.getCalls != 0 || store.headCalls != 0 {
		t.Fatal("no read link may be presigned before the recording is ready")
	}
}

func TestRecruiterRecordingPresignsReadyLinkForFiveMinutes(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
	store := &fakeRecordingStore{headInfo: webmInfo(`"v1"`)}
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
	if store.getOverrides.ContentType != "video/webm" || !strings.HasPrefix(store.getOverrides.ContentDisposition, "attachment;") {
		t.Fatalf("link must be typed and downloaded as an attachment: %+v", store.getOverrides)
	}
}

func TestRecruiterRecordingRefusesOverwrittenObject(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, failChangedOK: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
	store := &fakeRecordingStore{headInfo: webmInfo(`"v2"`)}
	svc := newTestRecordingService(t, repo, store)

	recording, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeRecordingIntegrity)
	if recording.URL != nil || store.getCalls != 0 {
		t.Fatal("an overwritten recording must never get a read link")
	}
	if repo.failChangedCalls != 1 || repo.failChangedKey != testRecordingKey {
		t.Fatalf("overwritten recording must be marked failed, calls=%d key=%q", repo.failChangedCalls, repo.failChangedKey)
	}
}

func TestRecruiterRecordingKeepsLegacyReadyRecordingWithoutETagReadable(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, failChangedOK: true, state: readyRecordingWithETag(nil)}
	store := &fakeRecordingStore{headInfo: webmInfo(`"v1"`)}
	svc := newTestRecordingService(t, repo, store)
	if _, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New()); err != nil {
		t.Fatalf("a recording finalized before ETags existed must stay readable: %v", err)
	}
	if repo.failChangedCalls != 0 {
		t.Fatal("a legacy recording must not be marked failed")
	}
}

func TestRecruiterRecordingMarksVanishedObjectFailed(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, failChangedOK: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
	store := &fakeRecordingStore{headErr: storage.ErrNotFound}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeRecordingNotFound)
	if repo.failChangedCalls != 1 || store.getCalls != 0 {
		t.Fatalf("vanished object must be marked failed without a link, calls=%d", repo.failChangedCalls)
	}
}

func TestRecruiterRecordingStorageFailureDoesNotChangeState(t *testing.T) {
	repo := &fakeRecordingRepository{owned: true, state: readyRecordingWithETag(strPtr(`"v1"`))}
	store := &fakeRecordingStore{headErr: errors.New("storage unavailable")}
	svc := newTestRecordingService(t, repo, store)
	_, err := svc.Recording(context.Background(), uuid.New(), "recruiter", uuid.New(), uuid.New())
	assertCode(t, err, apperror.CodeInternal)
	if repo.failChangedCalls != 0 {
		t.Fatal("a storage outage must not mark the recording failed")
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
			repo := &fakeRecordingRepository{owned: tc.owned, state: readyRecordingWithETag(strPtr(`"v1"`))}
			store := &fakeRecordingStore{headInfo: webmInfo(`"v1"`)}
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

func TestRecordingStatusReturnsOnlyTheStateToTheCandidate(t *testing.T) {
	for _, status := range []domain.RecordStatus{domain.RecordNone, domain.RecordUploading, domain.RecordReady, domain.RecordFailed} {
		t.Run(string(status), func(t *testing.T) {
			repo := &fakeRecordingRepository{state: recruitmentRecordingState(status, recordingPath(testRecordingKey))}
			store := &fakeRecordingStore{grantKey: testRecordingKey}
			svc := newTestRecordingService(t, repo, store)
			got, err := svc.Status(context.Background(), uuid.New(), "candidate", uuid.New())
			if err != nil {
				t.Fatalf("Status() error = %v", err)
			}
			if got.RecordStatus != status {
				t.Fatalf("record_status = %s, want %s", got.RecordStatus, status)
			}
			if store.putCalls != 0 || store.getCalls != 0 || store.headCalls != 0 || repo.uploadCalls != 0 || repo.resultCalls != 0 {
				t.Fatal("reading the status must not touch storage or write the recording")
			}
		})
	}
}

func TestRecordingStatusRefusesPracticeAndOtherRoles(t *testing.T) {
	repo := &fakeRecordingRepository{state: domain.RecordingState{Type: domain.TypePractice, RecordStatus: domain.RecordNone}}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	_, err := svc.Status(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeRecordingNotAllowed)
	_, err = svc.Status(context.Background(), uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeForbidden)
}

func TestRecordingStatusOfAnotherCandidatesInterviewIsNotFound(t *testing.T) {
	repo := &fakeRecordingRepository{stateErr: domain.ErrInterviewNotFound}
	svc := newTestRecordingService(t, repo, &fakeRecordingStore{})
	_, err := svc.Status(context.Background(), uuid.New(), "candidate", uuid.New())
	assertCode(t, err, apperror.CodeInterviewNotFound)
}
