package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

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
	Update(ctx context.Context, recruiterID, id uuid.UUID, patch domain.Edit, document *domain.NewDocument) error
	Submit(ctx context.Context, recruiterID, id uuid.UUID) error
	Resubmit(ctx context.Context, recruiterID, id uuid.UUID) error
}

// DocumentStore keeps the JD PDF; job_descriptions.content_path holds the key it returns. Postings
// created before PDF upload hold a plain-text object under that key instead.
type DocumentStore interface {
	Upload(ctx context.Context, body io.ReadSeeker, contentType string) (string, error)
	Download(ctx context.Context, key string) (storage.Object, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (storage.SignedRequest, error)
}

// PDFParser reads the text of a JD PDF. The Role Profile feature's Poppler parser implements it.
type PDFParser interface {
	Parse(ctx context.Context, data []byte) (string, error)
}

const (
	pdfContentType = "application/pdf"
	// MaxPDFBytes matches the Role Profile PDF limit.
	MaxPDFBytes     int64 = 5 << 20
	documentLinkTTL       = 5 * time.Minute
)

type Service struct {
	repository Repository
	documents  DocumentStore
	parser     PDFParser
}

func New(repository Repository, documents DocumentStore, parser PDFParser) (*Service, error) {
	if repository == nil || documents == nil || parser == nil {
		return nil, fmt.Errorf("job posting repository, document store and PDF parser are required")
	}
	return &Service{repository: repository, documents: documents, parser: parser}, nil
}

func requireRecruiter(role string) error {
	if role != recruiterRole {
		return apperror.New(apperror.CodeForbidden, "only recruiters can manage job postings")
	}
	return nil
}

// UploadDocument stores a JD PDF and returns its ID with the text read from it, so the Recruiter
// can ask for requirement suggestions before creating the posting. The PDF is read before it is
// stored, so an unreadable file leaves nothing behind.
func (s *Service) UploadDocument(ctx context.Context, role string, data []byte) (domain.Document, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.Document{}, err
	}
	text, err := s.parsePDF(ctx, data)
	if err != nil {
		return domain.Document{}, err
	}
	key, err := s.documents.Upload(ctx, bytes.NewReader(data), pdfContentType)
	if err != nil {
		return domain.Document{}, storageError(err)
	}
	return domain.Document{DocumentID: key, RawText: text}, nil
}

// Create reads the uploaded PDF again on the server, so the stored text always comes from the PDF
// the Admin will see, never from the client.
func (s *Service) Create(ctx context.Context, recruiterID uuid.UUID, role string, input domain.CreateInput) (domain.JobPosting, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.JobPosting{}, err
	}
	input, err := validateCreate(input)
	if err != nil {
		return domain.JobPosting{}, err
	}
	if input.RawText, err = s.loadDocument(ctx, input.DocumentID); err != nil {
		return domain.JobPosting{}, err
	}
	record, err := s.repository.Create(ctx, recruiterID, domain.NewPosting{CreateInput: input, ContentPath: input.DocumentID})
	if err != nil {
		return domain.JobPosting{}, err
	}
	return s.withText(ctx, record)
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
	var document *domain.NewDocument
	if patch.DocumentID.Present {
		current, err := s.repository.Get(ctx, recruiterID, id)
		if err != nil {
			return domain.JobPosting{}, err
		}
		// Resending the PDF the posting already has is not a change.
		if *patch.DocumentID.Value != current.ContentPath {
			// Checked again under the row lock; this only avoids reading the PDF for nothing.
			if current.Status != domain.StatusRejected {
				return domain.JobPosting{}, domain.ErrTextLocked
			}
			text, err := s.loadDocument(ctx, *patch.DocumentID.Value)
			if err != nil {
				return domain.JobPosting{}, err
			}
			document = &domain.NewDocument{ContentPath: *patch.DocumentID.Value, RawText: text}
		}
	}
	if err := s.repository.Update(ctx, recruiterID, id, patch, document); err != nil {
		return domain.JobPosting{}, err
	}
	return s.read(ctx, recruiterID, id)
}

// Submit sends the Recruiter's prepared posting for Admin review. Saving an edit or generating the
// question bank never does this on its own. Repeating it is safe.
func (s *Service) Submit(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.JobPosting, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.JobPosting{}, err
	}
	if err := s.repository.Submit(ctx, recruiterID, id); err != nil {
		return domain.JobPosting{}, err
	}
	return s.read(ctx, recruiterID, id)
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

// Document returns a short-lived link to the posting's JD PDF.
func (s *Service) Document(ctx context.Context, recruiterID uuid.UUID, role string, id uuid.UUID) (domain.DocumentLink, error) {
	if err := requireRecruiter(role); err != nil {
		return domain.DocumentLink{}, err
	}
	record, err := s.repository.Get(ctx, recruiterID, id)
	if err != nil {
		return domain.DocumentLink{}, err
	}
	if record.StoredText == nil {
		return domain.DocumentLink{}, apperror.New(apperror.CodeJDNotFound, "This Job Posting has no PDF.")
	}
	link, err := s.documents.PresignGet(ctx, record.ContentPath, documentLinkTTL)
	if err != nil {
		return domain.DocumentLink{}, storageError(err)
	}
	return domain.DocumentLink{URL: link.URL, ExpiresAt: link.ExpiresAt}, nil
}

func (s *Service) read(ctx context.Context, recruiterID, id uuid.UUID) (domain.JobPosting, error) {
	record, err := s.repository.Get(ctx, recruiterID, id)
	if err != nil {
		return domain.JobPosting{}, err
	}
	return s.withText(ctx, record)
}

// withText fills RawText: from the stored PDF text, or for an older posting from its plain-text
// object.
func (s *Service) withText(ctx context.Context, record domain.Record) (domain.JobPosting, error) {
	if record.StoredText != nil {
		record.RawText, record.HasDocument = *record.StoredText, true
		return record.JobPosting, nil
	}
	text, err := s.readText(ctx, record.ContentPath)
	if err != nil {
		return domain.JobPosting{}, err
	}
	record.RawText = text
	return record.JobPosting, nil
}

// loadDocument reads back a PDF uploaded through UploadDocument and returns its text.
func (s *Service) loadDocument(ctx context.Context, documentID string) (string, error) {
	object, err := s.documents.Download(ctx, documentID)
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrInvalidInput) {
		return "", apperror.New(apperror.CodeInvalidPDF, "The JD PDF was not found. Upload it again.")
	}
	if err != nil {
		return "", storageError(err)
	}
	defer func() { _ = object.Body.Close() }()
	if !strings.EqualFold(object.ContentType, pdfContentType) || object.Size > MaxPDFBytes {
		return "", apperror.New(apperror.CodeInvalidPDF, "The JD must be a PDF up to 5 MB. Upload it again.")
	}
	data, err := io.ReadAll(io.LimitReader(object.Body, MaxPDFBytes+1))
	if err != nil {
		return "", storageError(err)
	}
	return s.parsePDF(ctx, data)
}

// parsePDF reads the PDF text and checks it against the same length limits as typed JD text.
func (s *Service) parsePDF(ctx context.Context, data []byte) (string, error) {
	if int64(len(data)) > MaxPDFBytes {
		return "", apperror.New(apperror.CodePDFTooLarge, "Choose a PDF up to 5 MB.")
	}
	text, err := s.parser.Parse(ctx, data)
	if err != nil {
		return "", err
	}
	if err := checkText(text); err != nil {
		return "", err
	}
	return text, nil
}

func (s *Service) readText(ctx context.Context, key string) (string, error) {
	object, err := s.documents.Download(ctx, key)
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
