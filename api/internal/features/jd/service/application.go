package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Repository is the persistence contract consumed by JD application operations. Every method is
// scoped to the Candidate; a JD owned by someone else is reported as not found.
type Repository interface {
	Create(context.Context, uuid.UUID, domain.Record) (domain.Record, error)
	List(context.Context, uuid.UUID, int32, int32) ([]domain.ListItem, int64, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.Record, error)
	Update(context.Context, uuid.UUID, uuid.UUID, domain.Record) (domain.Record, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

// TextStore keeps the original JD text. job_descriptions has no text column, only content_path,
// which holds the key this store returns.
type TextStore interface {
	Upload(ctx context.Context, body io.ReadSeeker, contentType string) (string, error)
	Download(ctx context.Context, key string) (storage.Object, error)
}

const jdTextContentType = "text/plain; charset=utf-8"

// maxStoredJDBytes bounds a download: MaxJDLength runes of at most 4 bytes each.
const maxStoredJDBytes = 4 * MaxJDLength

// Application combines the existing extraction pipeline with reviewed persistence.
type Application struct {
	extraction *Service
	repository Repository
	texts      TextStore
}

func NewApplication(extraction *Service, repository Repository, texts TextStore) (*Application, error) {
	if extraction == nil || repository == nil || texts == nil {
		return nil, fmt.Errorf("JD extraction, repository and text store are required")
	}
	return &Application{extraction: extraction, repository: repository, texts: texts}, nil
}

func (a *Application) Analyze(ctx context.Context, raw string) (domain.StructuredJD, error) {
	return a.extraction.Extract(ctx, raw)
}

// Create stores the original text first and the rows second. If the rows fail, the uploaded
// object stays behind unreferenced: the storage package has no delete operation yet.
func (a *Application) Create(ctx context.Context, user uuid.UUID, input domain.CreateInput) (domain.JD, error) {
	if _, err := NormalizeInput(input.RawText); err != nil {
		return domain.JD{}, err
	}
	// The length check above counts normalized text, but the original is what gets stored, and
	// withText refuses anything over the cap. Without this, a JD padded with whitespace would
	// save and then never be readable again.
	if len(input.RawText) > maxStoredJDBytes {
		return domain.JD{}, apperror.Wrap(apperror.CodeJDTooLong, jdTooLongMessage, fmt.Errorf("raw text is %d bytes, over %d", len(input.RawText), maxStoredJDBytes))
	}
	reviewed, err := ValidateReviewed(input.StructuredJD)
	if err != nil {
		return domain.JD{}, err
	}
	key, err := a.texts.Upload(ctx, strings.NewReader(input.RawText), jdTextContentType)
	if err != nil {
		return domain.JD{}, storageError(err)
	}
	record, err := a.repository.Create(ctx, user, toRecord(reviewed, key))
	if err != nil {
		return domain.JD{}, err
	}
	return toJD(record, input.RawText)
}

func (a *Application) List(ctx context.Context, user uuid.UUID, limit, offset int32) ([]domain.ListItem, int64, error) {
	items, total, err := a.repository.List(ctx, user, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	for _, item := range items {
		if err := checkSeniority(item.SeniorityLevel); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func (a *Application) Get(ctx context.Context, user, id uuid.UUID) (domain.JD, error) {
	record, err := a.repository.Get(ctx, user, id)
	if err != nil {
		return domain.JD{}, err
	}
	return a.withText(ctx, record)
}

func (a *Application) Update(ctx context.Context, user, id uuid.UUID, input domain.UpdateInput) (domain.JD, error) {
	reviewed, err := ValidateReviewed(input.StructuredJD)
	if err != nil {
		return domain.JD{}, err
	}
	// The text never changes on update, so read it before writing: a storage failure then
	// leaves the JD untouched instead of reporting an error for a change that was saved.
	current, err := a.repository.Get(ctx, user, id)
	if err != nil {
		return domain.JD{}, err
	}
	text, err := a.readText(ctx, current.ContentPath)
	if err != nil {
		return domain.JD{}, err
	}
	record, err := a.repository.Update(ctx, user, id, toRecord(reviewed, ""))
	if err != nil {
		return domain.JD{}, err
	}
	return toJD(record, text)
}

// Delete removes the rows. The text object is left in storage until the storage package can
// delete objects.
func (a *Application) Delete(ctx context.Context, user, id uuid.UUID) error {
	return a.repository.Delete(ctx, user, id)
}

func toRecord(jd domain.StructuredJD, contentPath string) domain.Record {
	return domain.Record{ContentPath: contentPath, Title: jd.Title, Seniority: *jd.SeniorityLevel, Requirements: requirementRows(jd)}
}

func (a *Application) withText(ctx context.Context, record domain.Record) (domain.JD, error) {
	text, err := a.readText(ctx, record.ContentPath)
	if err != nil {
		return domain.JD{}, err
	}
	return toJD(record, text)
}

func (a *Application) readText(ctx context.Context, key string) (string, error) {
	object, err := a.texts.Download(ctx, key)
	if err != nil {
		return "", storageError(err)
	}
	defer func() { _ = object.Body.Close() }()
	text, err := io.ReadAll(io.LimitReader(object.Body, maxStoredJDBytes+1))
	if err != nil {
		return "", storageError(err)
	}
	if len(text) > maxStoredJDBytes {
		return "", storageError(fmt.Errorf("stored JD text exceeds %d bytes", maxStoredJDBytes))
	}
	return string(text), nil
}

// toJD is the one place a stored record becomes a response, so Create, Get and Update return
// the same competency order and reject the same unknown levels.
func toJD(record domain.Record, text string) (domain.JD, error) {
	if err := checkSeniority(record.Seniority); err != nil {
		return domain.JD{}, err
	}
	return domain.JD{ID: record.ID, RawText: text, StructuredJD: structuredJD(record), CreatedAt: record.CreatedAt}, nil
}

// checkSeniority guards against a levels row outside the extraction values (for example one an
// admin added by hand). Returning it would hand the client a seniority the API itself rejects.
func checkSeniority(level domain.Seniority) error {
	switch level {
	case domain.Intern, domain.Junior, domain.Mid, domain.Senior, domain.Lead:
		return nil
	default:
		return apperror.Wrap(apperror.CodeInternal, "job description persistence failed", fmt.Errorf("unknown seniority level %q", level))
	}
}

func storageError(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "job description file storage failed", err)
}
