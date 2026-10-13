package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
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

// QuestionBank generates the hidden core-question bank of a Candidate's own JD (KAN-79). It
// refuses when a bank already exists, when the Candidate doesn't own the JD, and when the
// requirements changed while it was generating.
type QuestionBank interface {
	GenerateForPractice(ctx context.Context, candidateID, jdID uuid.UUID) error
	Enabled() bool
}

// generationTimeout bounds one background generation (model call plus its single retry). It is
// longer than an HTTP request may take, which is why generation doesn't run inside one.
const generationTimeout = 3 * time.Minute

const jdTextContentType = "text/plain; charset=utf-8"

// maxStoredJDBytes bounds a download: MaxJDLength runes of at most 4 bytes each.
const maxStoredJDBytes = 4 * MaxJDLength

// Application combines the existing extraction pipeline with reviewed persistence.
type Application struct {
	extraction *Service
	repository Repository
	texts      TextStore
	bank       QuestionBank
	log        *logger.Logger
	// background runs generation off the request; tests swap it for a synchronous call.
	background func(func())
}

func NewApplication(extraction *Service, repository Repository, texts TextStore, bank QuestionBank, log *logger.Logger) (*Application, error) {
	if extraction == nil || repository == nil || texts == nil || bank == nil {
		return nil, fmt.Errorf("JD extraction, repository, text store and question bank are required")
	}
	if log == nil {
		log = &logger.Logger{Logger: zap.NewNop()}
	}
	return &Application{extraction: extraction, repository: repository, texts: texts, bank: bank, log: log,
		background: func(task func()) { go task() }}, nil
}

func (a *Application) Analyze(ctx context.Context, raw string) (domain.StructuredJD, error) {
	return a.extraction.Extract(ctx, raw)
}

// Create is the Candidate's confirmation. It stores the original text first and the rows second
// (one transaction), then starts question-bank generation in the background and answers with
// QuestionBankReady false. A model call can outlast the HTTP write timeout, and a confirm that
// looked failed would be sent again and create a second JD. The client polls Get until the bank
// is ready; GenerateQuestionBank retries a failed generation. If the rows fail, the uploaded
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
	note, err := ValidateRefinementNote(input.RefinementNote)
	if err != nil {
		return domain.JD{}, err
	}
	key, err := a.texts.Upload(ctx, strings.NewReader(input.RawText), jdTextContentType)
	if err != nil {
		return domain.JD{}, storageError(err)
	}
	record, err := a.repository.Create(ctx, user, toRecord(reviewed, note, key))
	if err != nil {
		return domain.JD{}, err
	}
	if a.bank.Enabled() {
		a.startGeneration(ctx, user, record.ID)
	} else {
		a.log.WithContext(ctx).Warn("question bank generation is not configured; JD confirmed without a bank",
			logger.String("jd_id", record.ID.String()))
	}
	return toJD(record, input.RawText)
}

// GenerateQuestionBank restarts generation for a confirmed JD whose bank is missing. Everything
// that can fail synchronously is checked first (ownership, an existing bank, generation switched
// off, the stored text), so the call never reports an error after it started the work.
func (a *Application) GenerateQuestionBank(ctx context.Context, user, id uuid.UUID) (domain.JD, error) {
	record, err := a.repository.Get(ctx, user, id)
	if err != nil {
		return domain.JD{}, err
	}
	if record.HasQuestionBank {
		return domain.JD{}, apperror.New(apperror.CodeQuestionBankExists, "this job description already has a question bank")
	}
	if !a.bank.Enabled() {
		return domain.JD{}, apperror.New(apperror.CodeQuestionBankUnavailable, "question bank generation is not configured")
	}
	jd, err := a.withText(ctx, record)
	if err != nil {
		return domain.JD{}, err
	}
	a.startGeneration(ctx, user, id)
	return jd, nil
}

// startGeneration runs one generation detached from the request (it must survive the response
// and a client disconnect) but bounded by its own deadline. Every outcome is logged, so a
// missing bank can be traced to its cause.
func (a *Application) startGeneration(ctx context.Context, user, id uuid.UUID) {
	log := a.log.WithContext(ctx)
	detached := context.WithoutCancel(ctx)
	a.background(func() {
		genCtx, cancel := context.WithTimeout(detached, generationTimeout)
		defer cancel()
		if err := a.bank.GenerateForPractice(genCtx, user, id); err != nil {
			log.Warn("question bank generation failed", logger.String("jd_id", id.String()),
				logger.String("code", string(errorCode(err))), logger.Error(err))
			return
		}
		log.Info("question bank generated", logger.String("jd_id", id.String()))
	})
}

func errorCode(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return apperror.CodeInternal
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
	note, err := ValidateRefinementNote(input.RefinementNote)
	if err != nil {
		return domain.JD{}, err
	}
	// The text never changes on update, so read it before writing: a storage failure then
	// leaves the JD untouched instead of reporting an error for a change that was saved.
	current, err := a.repository.Get(ctx, user, id)
	if err != nil {
		return domain.JD{}, err
	}
	// Checked again under the row lock in the repository; this saves a storage read.
	if current.HasQuestionBank {
		return domain.JD{}, domain.ErrRequirementsLocked
	}
	text, err := a.readText(ctx, current.ContentPath)
	if err != nil {
		return domain.JD{}, err
	}
	record, err := a.repository.Update(ctx, user, id, toRecord(reviewed, note, ""))
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

func toRecord(jd domain.StructuredJD, note *string, contentPath string) domain.Record {
	return domain.Record{ContentPath: contentPath, Title: jd.Title, Seniority: *jd.SeniorityLevel, RefinementNote: note, Requirements: requirementRows(jd)}
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
	return domain.JD{ID: record.ID, RawText: text, StructuredJD: structuredJD(record), RefinementNote: record.RefinementNote,
		QuestionBankReady: record.HasQuestionBank, CreatedAt: record.CreatedAt}, nil
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
