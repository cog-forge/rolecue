package service

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const recruiterRole = "recruiter"

// logRecordingStatus records a recording status change. Only IDs, statuses and sizes are logged.
func logRecordingStatus(ctx context.Context, interviewID, candidateID uuid.UUID, from, to domain.RecordStatus, extra ...zap.Field) {
	fields := append([]zap.Field{
		logger.Op("interview.recording_status"),
		logger.String("interview_id", interviewID.String()),
		logger.String("user_id", candidateID.String()),
		logger.String("from_status", string(from)),
		logger.String("to_status", string(to)),
	}, extra...)
	logger.FromContext(ctx).Info("recording status changed", fields...)
}

// RecordingRepository is the persistence contract of recruitment recordings and evidence reads. The
// repository checks ownership in its queries, so another user's interview or Application is not found.
type RecordingRepository interface {
	GetCandidateRecording(ctx context.Context, candidateID, id uuid.UUID) (domain.RecordingState, error)
	SetRecordingUpload(ctx context.Context, candidateID, id uuid.UUID, key string, maxGrants int32) (bool, error)
	SetRecordingResult(ctx context.Context, candidateID, id uuid.UUID, key string, status domain.RecordStatus, etag string) (bool, error)
	FailChangedRecording(ctx context.Context, id uuid.UUID, key string) (bool, error)
	RecruiterOwnsApplication(ctx context.Context, recruiterID, jobPostingID, applicationID uuid.UUID) (bool, error)
	GetApplicationRecording(ctx context.Context, applicationID uuid.UUID) (domain.RecordingState, error)
	ListTranscript(ctx context.Context, interviewID uuid.UUID) ([]domain.TranscriptTurn, error)
}

// RecordingStore is the object storage the recording flow uses. storage.Store satisfies it.
type RecordingStore interface {
	PresignPut(ctx context.Context, contentType string, size int64, ttl time.Duration) (storage.UploadGrant, error)
	PresignGetWithOverrides(ctx context.Context, key string, ttl time.Duration, overrides storage.GetOverrides) (storage.SignedRequest, error)
	Head(ctx context.Context, key string) (storage.ObjectInfo, error)
	ReadPrefix(ctx context.Context, key string, n int64) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// RecordingService grants and finalizes the Candidate's recruitment recording and gives the owning
// Recruiter read access to the recording and the transcript. The recording is evidence for the Recruiter,
// so practice sessions never have one.
//
// Retention (decided 2026-11-03): a recording is kept for 90 days after the Application's final decision,
// then deleted, on the normal storage provider, by the System Handler (RecordingUpkeep, KAN-272), which
// also fails recordings that never arrived. Admin access to recordings is still an open product gate.
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
// stored before the grant is returned, so finalization can verify exactly that object. A recording that is
// processing or ready cannot be replaced, and an interview gets at most domain.RecordingMaxGrants grants.
// The object the new grant replaces is deleted on a best-effort basis.
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
	if state.GrantCount >= domain.RecordingMaxGrants {
		return domain.RecordingUploadGrant{}, domain.ErrRecordingUploadLimit
	}
	grant, err := s.store.PresignPut(ctx, input.ContentType, input.Size, domain.RecordingUploadTTL)
	if err != nil {
		return domain.RecordingUploadGrant{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	updated, err := s.repository.SetRecordingUpload(ctx, candidateID, interviewID, grant.Key, domain.RecordingMaxGrants)
	if err != nil {
		return domain.RecordingUploadGrant{}, err
	}
	if !updated {
		// The recording changed between the read and the write, or the grant cap was reached. The key was
		// never stored, so the grant is refused rather than issued for a key nothing points to.
		return domain.RecordingUploadGrant{}, domain.ErrRecordingNotAllowed
	}
	if state.RecordPath != nil && *state.RecordPath != grant.Key {
		s.deleteReplaced(ctx, interviewID, candidateID, *state.RecordPath)
	}
	logRecordingStatus(ctx, interviewID, candidateID, state.RecordStatus, domain.RecordUploading, zap.Int64("bytes", input.Size))
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

// deleteReplaced removes the object a new grant replaced. The new key is already stored, so a failed delete
// only leaves an unreferenced object; it is logged without the key.
func (s *RecordingService) deleteReplaced(ctx context.Context, interviewID, candidateID uuid.UUID, key string) {
	if err := s.store.Delete(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("replaced recording object was not deleted",
			logger.Op("interview.recording_cleanup"),
			logger.String("interview_id", interviewID.String()),
			logger.String("user_id", candidateID.String()),
			logger.String("reason", "delete_failed"),
		)
	}
}

// CompleteUpload verifies the uploaded object and sets the recording ready. The object must exist, be
// within the size limit, and start with the magic bytes of the media type it was uploaded as. The ETag
// of the verified object is stored, so a later overwrite is detected at read time. A missing or invalid
// object marks the recording failed. Completing a ready recording returns ready again.
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
	fail := func(reason string, cause error, extra ...zap.Field) error {
		return s.failUpload(ctx, candidateID, interviewID, state.RecordStatus, key, reason, cause, extra...)
	}
	info, err := s.store.Head(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return domain.RecordingStatus{}, fail("object_missing", domain.ErrRecordingNotFound)
	}
	if err != nil {
		return domain.RecordingStatus{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	// The grant signs the declared content type and length into the upload URL (storage.PresignPut), so
	// the store refuses any other body; these checks are the second line, on what actually arrived.
	if info.Size <= 0 || info.Size > domain.RecordingMaxBytes {
		return domain.RecordingStatus{}, fail("size_out_of_range",
			apperror.New(apperror.CodeValidation, "the uploaded recording is empty or too large"), zap.Int64("bytes", info.Size))
	}
	if !domain.ValidRecordingContentType(info.ContentType) {
		return domain.RecordingStatus{}, fail("content_type_unknown",
			apperror.New(apperror.CodeValidation, "the uploaded recording has an unsupported media type"))
	}
	head, err := s.store.ReadPrefix(ctx, key, domain.RecordingSniffBytes)
	if errors.Is(err, storage.ErrNotFound) {
		return domain.RecordingStatus{}, fail("object_missing", domain.ErrRecordingNotFound)
	}
	if err != nil {
		return domain.RecordingStatus{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	if !domain.RecordingMatchesMagic(info.ContentType, head) {
		return domain.RecordingStatus{}, fail("content_mismatch",
			apperror.New(apperror.CodeValidation, "the uploaded file is not the media type it declares"))
	}
	if info.ETag == "" {
		return domain.RecordingStatus{}, apperror.New(apperror.CodeInternal, "recording storage returned no ETag")
	}
	updated, err := s.repository.SetRecordingResult(ctx, candidateID, interviewID, key, domain.RecordReady, info.ETag)
	if err != nil {
		return domain.RecordingStatus{}, err
	}
	if !updated {
		return domain.RecordingStatus{}, domain.ErrRecordingNotAllowed
	}
	logRecordingStatus(ctx, interviewID, candidateID, state.RecordStatus, domain.RecordReady, zap.Int64("bytes", info.Size))
	return domain.RecordingStatus{RecordStatus: domain.RecordReady}, nil
}

// failUpload marks the upload at key failed, logs the reason and returns the error the Candidate sees.
func (s *RecordingService) failUpload(ctx context.Context, candidateID, interviewID uuid.UUID, from domain.RecordStatus, key, reason string, cause error, extra ...zap.Field) error {
	if _, err := s.repository.SetRecordingResult(ctx, candidateID, interviewID, key, domain.RecordFailed, ""); err != nil {
		return err
	}
	fields := append([]zap.Field{logger.String("reason", reason)}, extra...)
	logRecordingStatus(ctx, interviewID, candidateID, from, domain.RecordFailed, fields...)
	return cause
}

// Status returns the Candidate's view of the recording of their recruitment interview: only the state,
// never the object or a link. The room reads it after a reload to show the real upload state.
func (s *RecordingService) Status(ctx context.Context, candidateID uuid.UUID, role string, interviewID uuid.UUID) (domain.RecordingStatus, error) {
	if role != candidateRole {
		return domain.RecordingStatus{}, apperror.New(apperror.CodeForbidden, "only candidates can read their recording status")
	}
	state, err := s.repository.GetCandidateRecording(ctx, candidateID, interviewID)
	if err != nil {
		return domain.RecordingStatus{}, err
	}
	if state.Type != domain.TypeRecruitment {
		return domain.RecordingStatus{}, domain.ErrRecordingNotAllowed
	}
	return domain.RecordingStatus{RecordStatus: state.RecordStatus}, nil
}

// Recording returns the Recruiter's view of the recording of an Application. The read link is issued
// only when the recording is ready, the stored object still has the ETag verified at completion, and only to
// the owning Recruiter. A ready recording whose object changed or vanished is marked failed, so the
// Candidate must upload it again; no link is issued for it.
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
	key := *state.RecordPath
	info, err := s.store.Head(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return s.refuseChangedRecording(ctx, recruiterID, state, key, "object_missing", domain.ErrRecordingNotFound)
	}
	if err != nil {
		return domain.RecruiterRecording{}, apperror.Wrap(apperror.CodeInternal, "recording storage is unavailable", err)
	}
	// Rows finalized before record_etag existed carry no ETag to compare;
	// they stay readable rather than being failed for a missing baseline.
	if state.RecordETag != nil && *state.RecordETag != info.ETag {
		return s.refuseChangedRecording(ctx, recruiterID, state, key, "etag_mismatch", domain.ErrRecordingIntegrity)
	}
	if !domain.ValidRecordingContentType(info.ContentType) {
		return s.refuseChangedRecording(ctx, recruiterID, state, key, "content_type_unknown", domain.ErrRecordingIntegrity)
	}
	request, err := s.store.PresignGetWithOverrides(ctx, key, domain.RecordingReadTTL, storage.GetOverrides{
		ContentType:        info.ContentType,
		ContentDisposition: recordingDisposition(info.ContentType),
	})
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

// refuseChangedRecording marks a ready recording failed because its object no longer matches what was
// verified, then returns the refusal. The row is changed only if it still holds the same key.
func (s *RecordingService) refuseChangedRecording(ctx context.Context, actorID uuid.UUID, state domain.RecordingState, key, reason string, cause error) (domain.RecruiterRecording, error) {
	updated, err := s.repository.FailChangedRecording(ctx, state.InterviewID, key)
	if err != nil {
		return domain.RecruiterRecording{}, err
	}
	if updated {
		logRecordingStatus(ctx, state.InterviewID, actorID, domain.RecordReady, domain.RecordFailed, logger.String("reason", reason))
	}
	return domain.RecruiterRecording{}, cause
}

// recordingDisposition makes a read link download the recording as a file, so the browser never renders it
// inline. The extension follows the verified media type.
func recordingDisposition(contentType string) string {
	ext := "webm"
	if contentType == "video/mp4" {
		ext = "mp4"
	}
	return `attachment; filename="interview-recording.` + ext + `"`
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
