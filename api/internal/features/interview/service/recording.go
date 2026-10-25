package service

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const recruiterRole = "recruiter"

// RecordingRepository is the persistence contract of recruitment recordings and evidence reads. The
// repository checks ownership in its queries, so another user's interview or Application is not found.
type RecordingRepository interface {
	GetCandidateRecording(ctx context.Context, candidateID, id uuid.UUID) (domain.RecordingState, error)
	SetRecordingUpload(ctx context.Context, candidateID, id uuid.UUID, key string) (bool, error)
	SetRecordingResult(ctx context.Context, candidateID, id uuid.UUID, key string, status domain.RecordStatus) (bool, error)
	RecruiterOwnsApplication(ctx context.Context, recruiterID, jobPostingID, applicationID uuid.UUID) (bool, error)
	GetApplicationRecording(ctx context.Context, applicationID uuid.UUID) (domain.RecordingState, error)
	ListTranscript(ctx context.Context, interviewID uuid.UUID) ([]domain.TranscriptTurn, error)
}

// RecordingStore is the object storage the recording flow uses. storage.Store satisfies it.
type RecordingStore interface {
	PresignPut(ctx context.Context, contentType string, size int64, ttl time.Duration) (storage.UploadGrant, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (storage.SignedRequest, error)
	Download(ctx context.Context, key string) (storage.Object, error)
}

// RecordingService grants and finalizes the Candidate's recruitment recording and gives the owning
// Recruiter read access to the recording and the transcript. The recording is evidence for the Recruiter,
// so practice sessions never have one.
//
// Retention, consent and any Admin access to recordings are open product gates. This service enforces
// none of them and does not delete objects.
type RecordingService struct {
	repository RecordingRepository
	store      RecordingStore
}

func NewRecordingService(repository RecordingRepository, store RecordingStore) (*RecordingService, error) {
	if repository == nil {
		return nil, errors.New("interview recording repository is required")
	}
	if store == nil {
		return nil, errors.New("interview recording store is required")
	}
	return &RecordingService{repository: repository, store: store}, nil
}

// GrantUpload issues the Candidate an upload for the recording of a recruitment interview. The key is
// stored before the grant is returned, so finalization can verify exactly that object. A recording that
// is processing or ready cannot be replaced.
func (s *RecordingService) GrantUpload(ctx context.Context, candidateID uuid.UUID, role string, interviewID uuid.UUID, input domain.RecordingUploadInput) (domain.RecordingUploadGrant, error) {
	if role != candidateRole {
		return domain.RecordingUploadGrant{}, apperror.New(apperror.CodeForbidden, "only candidates can upload recordings")
	}
	if err := validateRecordingUpload(input); err != nil {
		return domain.RecordingUploadGrant{}, err
	}
	state, err := s.repository.GetCandidateRecording(ctx, candidateID, interviewID)
	if err != nil {
		return domain.RecordingUploadGrant{}, err
	}
	if state.Type != domain.TypeRecruitment || !canStartUpload(state.RecordStatus) {
		return domain.RecordingUploadGrant{}, domain.ErrRecordingNotAllowed
	}
	grant, err := s.store.PresignPut(ctx, input.ContentType, input.Size, domain.RecordingUploadTTL)
	if err != nil {
		return domain.RecordingUploadGrant{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	updated, err := s.repository.SetRecordingUpload(ctx, candidateID, interviewID, grant.Key)
	if err != nil {
		return domain.RecordingUploadGrant{}, err
	}
	if !updated {
		// The recording changed between the read and the write. The key was never stored, so the grant
		// is refused rather than issued for a key nothing points to.
		return domain.RecordingUploadGrant{}, domain.ErrRecordingNotAllowed
	}
	headers := grant.Request.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return domain.RecordingUploadGrant{
		UploadURL:    grant.Request.URL,
		Method:       grant.Request.Method,
		Headers:      headers,
		ExpiresAt:    grant.Request.ExpiresAt,
		RecordStatus: domain.RecordUploading,
	}, nil
}

// CompleteUpload verifies the uploaded object and sets the recording ready. A missing object marks the
// recording failed and reports RECORDING_NOT_FOUND. Completing a ready recording returns ready again.
func (s *RecordingService) CompleteUpload(ctx context.Context, candidateID uuid.UUID, role string, interviewID uuid.UUID) (domain.RecordingStatus, error) {
	if role != candidateRole {
		return domain.RecordingStatus{}, apperror.New(apperror.CodeForbidden, "only candidates can complete recordings")
	}
	state, err := s.repository.GetCandidateRecording(ctx, candidateID, interviewID)
	if err != nil {
		return domain.RecordingStatus{}, err
	}
	if state.Type != domain.TypeRecruitment {
		return domain.RecordingStatus{}, domain.ErrRecordingNotAllowed
	}
	if state.RecordStatus == domain.RecordReady {
		return domain.RecordingStatus{RecordStatus: domain.RecordReady}, nil
	}
	if state.RecordStatus != domain.RecordUploading || state.RecordPath == nil {
		return domain.RecordingStatus{}, domain.ErrRecordingNotAllowed
	}
	key := *state.RecordPath
	object, err := s.store.Download(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		if _, err := s.repository.SetRecordingResult(ctx, candidateID, interviewID, key, domain.RecordFailed); err != nil {
			return domain.RecordingStatus{}, err
		}
		return domain.RecordingStatus{}, domain.ErrRecordingNotFound
	}
	if err != nil {
		return domain.RecordingStatus{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	if object.Body != nil {
		defer func() { _ = object.Body.Close() }()
	}
	if object.Size <= 0 || object.Size > domain.RecordingMaxBytes {
		if _, err := s.repository.SetRecordingResult(ctx, candidateID, interviewID, key, domain.RecordFailed); err != nil {
			return domain.RecordingStatus{}, err
		}
		return domain.RecordingStatus{}, apperror.New(apperror.CodeValidation, "the uploaded recording is empty or too large")
	}
	updated, err := s.repository.SetRecordingResult(ctx, candidateID, interviewID, key, domain.RecordReady)
	if err != nil {
		return domain.RecordingStatus{}, err
	}
	if !updated {
		return domain.RecordingStatus{}, domain.ErrRecordingNotAllowed
	}
	return domain.RecordingStatus{RecordStatus: domain.RecordReady}, nil
}

// Recording returns the Recruiter's view of the recording of an Application. The read link is issued
// only when the recording is ready, and only to the owning Recruiter.
func (s *RecordingService) Recording(ctx context.Context, recruiterID uuid.UUID, role string, jobPostingID, applicationID uuid.UUID) (domain.RecruiterRecording, error) {
	state, err := s.recruiterInterview(ctx, recruiterID, role, jobPostingID, applicationID)
	if err != nil {
		return domain.RecruiterRecording{}, err
	}
	if state.RecordStatus != domain.RecordReady {
		return domain.RecruiterRecording{RecordStatus: state.RecordStatus}, nil
	}
	if state.RecordPath == nil {
		return domain.RecruiterRecording{}, apperror.New(apperror.CodeInternal, "ready recording has no stored object")
	}
	request, err := s.store.PresignGet(ctx, *state.RecordPath, domain.RecordingReadTTL)
	if err != nil {
		return domain.RecruiterRecording{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	expiresAt := request.ExpiresAt
	return domain.RecruiterRecording{
		RecordStatus: state.RecordStatus,
		URL:          &request.URL,
		ExpiresAt:    &expiresAt,
	}, nil
}

// Transcript returns the turns of a completed recruitment interview to the owning Recruiter.
func (s *RecordingService) Transcript(ctx context.Context, recruiterID uuid.UUID, role string, jobPostingID, applicationID uuid.UUID) (domain.Transcript, error) {
	state, err := s.recruiterInterview(ctx, recruiterID, role, jobPostingID, applicationID)
	if err != nil {
		return domain.Transcript{}, err
	}
	if !state.Completed {
		return domain.Transcript{}, domain.ErrInterviewNotCompleted
	}
	turns, err := s.repository.ListTranscript(ctx, state.InterviewID)
	if err != nil {
		return domain.Transcript{}, err
	}
	if turns == nil {
		turns = []domain.TranscriptTurn{}
	}
	return domain.Transcript{Turns: turns}, nil
}

// recruiterInterview checks that the caller is the Recruiter of the posting and reads the interview of
// the Application. Any other caller, Admin included, gets APPLICATION_NOT_FOUND, so the existence of
// someone else's Application is not revealed.
func (s *RecordingService) recruiterInterview(ctx context.Context, recruiterID uuid.UUID, role string, jobPostingID, applicationID uuid.UUID) (domain.RecordingState, error) {
	if role != recruiterRole {
		return domain.RecordingState{}, domain.ErrApplicationNotFound
	}
	owned, err := s.repository.RecruiterOwnsApplication(ctx, recruiterID, jobPostingID, applicationID)
	if err != nil {
		return domain.RecordingState{}, err
	}
	if !owned {
		return domain.RecordingState{}, domain.ErrApplicationNotFound
	}
	return s.repository.GetApplicationRecording(ctx, applicationID)
}

// canStartUpload reports whether a recording may take a new upload grant. A processing or ready recording
// is kept as it is.
func canStartUpload(status domain.RecordStatus) bool {
	switch status {
	case domain.RecordNone, domain.RecordUploading, domain.RecordFailed:
		return true
	default:
		return false
	}
}

// validateRecordingUpload checks the declared media type and the exact byte size the grant binds.
func validateRecordingUpload(input domain.RecordingUploadInput) error {
	if !domain.ValidRecordingContentType(input.ContentType) {
		return apperror.New(apperror.CodeValidation, "content_type must be video/webm, audio/webm or video/mp4")
	}
	if input.Size < 1 || input.Size > domain.RecordingMaxBytes {
		return apperror.New(apperror.CodeValidation, "size must be between 1 and 524288000 bytes")
	}
	return nil
}
