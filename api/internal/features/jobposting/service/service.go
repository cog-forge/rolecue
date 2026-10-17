package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const recruiterRole = "recruiter"

// Repository is the persistence contract consumed by Job Posting operations. Every method is
// scoped to the Recruiter; a posting owned by someone else is reported as not found.
type Repository interface {
	Create(ctx context.Context, recruiterID uuid.UUID, input domain.NewPosting) (domain.Record, error)
	List(ctx context.Context, recruiterID uuid.UUID, filter domain.ListFilter) ([]domain.Summary, int64, error)
	Get(ctx context.Context, recruiterID, id uuid.UUID) (domain.Record, error)
	Update(ctx context.Context, recruiterID, id uuid.UUID, patch domain.Edit, contentPath *string) error
	Resubmit(ctx context.Context, recruiterID, id uuid.UUID) error
}

// TextStore keeps the original JD text. job_descriptions has no text column, only content_path,
// which holds the key this store returns.
type TextStore interface {
	Upload(ctx context.Context, body io.ReadSeeker, contentType string) (string, error)
	Download(ctx context.Context, key string) (storage.Object, error)
}

const jdTextContentType = "text/plain; charset=utf-8"

type Service struct {
	repository Repository
	texts      TextStore
}

func New(repository Repository, texts TextStore) (*Service, error) {
	if repository == nil || texts == nil {
		return nil, fmt.Errorf("job posting repository and text store are required")
	}
	return &Service{repository: repository, texts: texts}, nil
}

func requireRecruiter(role string) error {
	if role != recruiterRole {
		return apperror.New(apperror.CodeForbidden, "only recruiters can manage job postings")
	}
	return nil
}

// Create stores the JD text first and the rows second. If the rows fail, the uploaded object stays
// behind unreferenced: the storage package has no delete operation yet.
func (s *Service) Create(ctx context.Context, recruiterID uuid.UUID, role string, input domain.CreateInput) (domain.JobPosting, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.JobPosting{}, err
	}
	input, err := validateCreate(input)
	if err != nil {
		return domain.JobPosting{}, err
	}
	key, err := s.texts.Upload(ctx, strings.NewReader(input.RawText), jdTextContentType)
	if err != nil {
		return domain.JobPosting{}, storageError(err)
	}
	record, err := s.repository.Create(ctx, recruiterID, domain.NewPosting{CreateInput: input, ContentPath: key})
	if err != nil {
		return domain.JobPosting{}, err
	}
	// The text was just uploaded, so return it without a round trip to storage.
	record.RawText = input.RawText
	return record.JobPosting, nil
}

func (s *Service) List(ctx context.Context, recruiterID uuid.UUID, role string, filter domain.ListFilter) ([]domain.Summary, int64, error) {
	if err := requireRecruiter(role); err != nil {
		return nil, 0, err
	}
	filter, err := validateFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	return s.repository.List(ctx, recruiterID, filter)
}

func (s *Service) Get(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.JobPosting, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.JobPosting{}, err
	}
	return s.read(ctx, recruiterID, id)
}

func (s *Service) Update(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID, patch domain.Edit) (domain.JobPosting, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.JobPosting{}, err
	}
	patch, err := validatePatch(patch)
	if err != nil {
		return domain.JobPosting{}, err
	}
	// Read the stored text before writing: a storage failure then leaves the posting untouched
	// instead of reporting an error for a change that was saved, and resent text can be compared.
	current, err := s.repository.Get(ctx, recruiterID, id)
	if err != nil {
		return domain.JobPosting{}, err
	}
	text, err := s.readText(ctx, current.ContentPath)
	if err != nil {
		return domain.JobPosting{}, err
	}
	var contentPath *string
	if patch.RawText.Present {
		if *patch.RawText.Value == text {
			patch.RawText = domain.StringChange{}
		} else {
			// Checked again under the row lock; this only avoids a pointless upload.
			if current.Status != domain.StatusRejected {
				return domain.JobPosting{}, domain.ErrTextLocked
			}
			// Like Create: if the rows then fail, the new object stays behind unreferenced.
			key, err := s.texts.Upload(ctx, strings.NewReader(*patch.RawText.Value), jdTextContentType)
			if err != nil {
				return domain.JobPosting{}, storageError(err)
			}
			contentPath, text = &key, *patch.RawText.Value
		}
	}
	if err := s.repository.Update(ctx, recruiterID, id, patch, contentPath); err != nil {
		return domain.JobPosting{}, err
	}
	record, err := s.repository.Get(ctx, recruiterID, id)
	if err != nil {
		return domain.JobPosting{}, err
	}
	record.RawText = text
	return record.JobPosting, nil
}

// Resubmit sends the Recruiter's rejected posting back for Admin review, clearing the old
// rejection reason. Saving an edit never does this on its own. Repeating it is safe.
func (s *Service) Resubmit(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.JobPosting, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.JobPosting{}, err
	}
	if err := s.repository.Resubmit(ctx, recruiterID, id); err != nil {
		return domain.JobPosting{}, err
	}
	return s.read(ctx, recruiterID, id)
}

func (s *Service) read(ctx context.Context, recruiterID, id uuid.UUID) (domain.JobPosting, error) {
	record, err := s.repository.Get(ctx, recruiterID, id)
	if err != nil {
		return domain.JobPosting{}, err
	}
	text, err := s.readText(ctx, record.ContentPath)
	if err != nil {
		return domain.JobPosting{}, err
	}
	record.RawText = text
	return record.JobPosting, nil
}

func (s *Service) readText(ctx context.Context, key string) (string, error) {
	object, err := s.texts.Download(ctx, key)
	if err != nil {
		return "", storageError(err)
	}
	defer func() { _ = object.Body.Close() }()
	text, err := io.ReadAll(io.LimitReader(object.Body, maxStoredTextBytes+1))
	if err != nil {
		return "", storageError(err)
	}
	if len(text) > maxStoredTextBytes {
		return "", storageError(fmt.Errorf("stored JD text exceeds %d bytes", maxStoredTextBytes))
	}
	return string(text), nil
}

func storageError(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "job posting file storage failed", err)
}
