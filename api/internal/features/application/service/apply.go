package service

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// cvContentType is the stored content type of a CV, the same as the JD PDF.
const cvContentType = "application/pdf"

// CandidateRepository is the persistence contract of applying and of the Candidate's own
// Applications. The Repository implementation provides it; the service asserts it at call time, so
// the screening fakes keep satisfying Repository alone.
type CandidateRepository interface {
	// CheckIntakeFunded reports INTAKE_NOT_OPEN for an approved posting still without interview slots.
	CheckIntakeFunded(ctx context.Context, postingID uuid.UUID) error
	InsertCandidateApplication(ctx context.Context, candidateID, postingID uuid.UUID, cvKey string) (domain.Submission, error)
	ListCandidateApplications(ctx context.Context, candidateID uuid.UUID, limit, offset int32) ([]domain.CandidateApplication, int64, error)
	GetCandidateApplication(ctx context.Context, candidateID, applicationID uuid.UUID) (domain.CandidateApplication, error)
}

// CVStore stores and removes CV objects. The storage Store satisfies it; the service asserts it at
// call time, as with CandidateRepository.
type CVStore interface {
	Upload(ctx context.Context, body io.ReadSeeker, contentType string) (string, error)
	Delete(ctx context.Context, key string) error
}

// Apply submits the Candidate's CV to an open, funded Job Posting. The CV is validated, and a posting
// whose Recruiter has not funded interview slots is refused, before anything is uploaded. The CV is
// uploaded before the Application is inserted, and the upload is deleted again when the insert fails (KTD6), so a
// rejected submission stores nothing. The Application starts pending and needs no interview.
func (s *Service) Apply(ctx context.Context, candidateID uuid.UUID, role string, postingID uuid.UUID, data []byte) (domain.Submission, error) {
	if role != candidateRole {
		return domain.Submission{}, apperror.New(apperror.CodeForbidden, "only candidates can apply to job postings")
	}
	if err := domain.ValidateCV(data); err != nil {
		return domain.Submission{}, err
	}
	repository, store, err := s.applyDependencies()
	if err != nil {
		return domain.Submission{}, err
	}
	if err := repository.CheckIntakeFunded(ctx, postingID); err != nil {
		return domain.Submission{}, err
	}
	key, err := store.Upload(ctx, bytes.NewReader(data), cvContentType)
	if err != nil {
		return domain.Submission{}, apperror.Wrap(apperror.CodeInternal, "application CV storage failed", err)
	}
	submission, err := repository.InsertCandidateApplication(ctx, candidateID, postingID, key)
	if err != nil {
		// The Application was not stored, so the CV object must not stay behind. The original
		// error is the one the Candidate needs; a failed delete leaves only an orphan object.
		// The request context may already be cancelled (the client dropped), which would cancel this cleanup too.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		_ = store.Delete(cleanupCtx, key)
		cancel()
		return domain.Submission{}, err
	}
	return submission, nil
}

// applyDependencies returns the candidate persistence and CV storage the service was built with.
func (s *Service) applyDependencies() (CandidateRepository, CVStore, error) {
	repository, ok := s.repository.(CandidateRepository)
	if !ok {
		return nil, nil, apperror.New(apperror.CodeInternal, "candidate applications are not configured")
	}
	store, ok := s.cvLinker.(CVStore)
	if !ok {
		return nil, nil, apperror.New(apperror.CodeInternal, "CV storage is not configured")
	}
	return repository, store, nil
}
