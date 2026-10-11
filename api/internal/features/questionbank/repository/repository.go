package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository takes the pool, not just DBTX, because generation commits several statements together.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func (r *Repository) Posting(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.Posting, error) {
	row, err := r.queries.GetRecruiterPosting(ctx, GetRecruiterPostingParams{ID: postingID, RecruiterID: recruiterID})
	if err != nil {
		return domain.Posting{}, persistenceError(err)
	}
	return domain.Posting{JDID: row.JdID, Status: row.Status}, nil
}

func (r *Repository) CandidateOwnsJD(ctx context.Context, candidateID, jdID uuid.UUID) error {
	owns, err := r.queries.CandidateOwnsJD(ctx, CandidateOwnsJDParams{JdID: jdID, CandidateID: candidateID})
	if err != nil {
		return persistenceError(err)
	}
	if !owns {
		return apperror.New(apperror.CodeJDNotFound, "job description not found")
	}
	return nil
}

func (r *Repository) List(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	rows, err := r.queries.ListCoreQuestions(ctx, jdID)
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.CoreQuestion, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CoreQuestion{ID: row.ID, Content: row.Content})
	}
	return out, nil
}

func (r *Repository) GenerationInput(ctx context.Context, jdID uuid.UUID) (domain.GenerationInput, error) {
	rows, err := r.queries.ListRequirementsForJD(ctx, jdID)
	if err != nil {
		return domain.GenerationInput{}, persistenceError(err)
	}
	note, err := r.queries.GetJDRefinementNote(ctx, jdID)
	if err != nil {
		return domain.GenerationInput{}, persistenceError(err)
	}
	in := domain.GenerationInput{Requirements: make([]domain.ConfirmedRequirement, 0, len(rows))}
	for _, row := range rows {
		in.Requirements = append(in.Requirements, domain.ConfirmedRequirement{
			Name: row.Name, Target: row.Target, Type: row.Type, EvidenceText: row.EvidenceText})
	}
	if note != nil {
		in.RefinementNote = *note
	}
	return in, nil
}

// Add, Update and Delete are single statements that recheck ownership and the pending status, so
// there is no read-then-write gap in application code. (A status change committed by another
// transaction during that one statement can still slip through, a window of milliseconds; only
// SaveForPosting locks the posting row.) When no row is affected, a cheap follow-up query tells
// "not found" from "locked".
func (r *Repository) Add(ctx context.Context, recruiterID, postingID uuid.UUID, content string) (domain.CoreQuestion, error) {
	row, err := r.queries.AddCoreQuestion(ctx, AddCoreQuestionParams{Content: content, PostingID: postingID, RecruiterID: recruiterID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CoreQuestion{}, r.postingRefusal(ctx, r.queries, recruiterID, postingID, false)
	}
	if err != nil {
		return domain.CoreQuestion{}, persistenceError(err)
	}
	return domain.CoreQuestion{ID: row.ID, Content: row.Content}, nil
}

func (r *Repository) Update(ctx context.Context, recruiterID, postingID, questionID uuid.UUID, content string) (domain.CoreQuestion, error) {
	row, err := r.queries.UpdateCoreQuestion(ctx, UpdateCoreQuestionParams{
		Content: content, ID: questionID, PostingID: postingID, RecruiterID: recruiterID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CoreQuestion{}, r.postingRefusal(ctx, r.queries, recruiterID, postingID, true)
	}
	if err != nil {
		return domain.CoreQuestion{}, persistenceError(err)
	}
	return domain.CoreQuestion{ID: row.ID, Content: row.Content}, nil
}

func (r *Repository) Delete(ctx context.Context, recruiterID, postingID, questionID uuid.UUID) error {
	n, err := r.queries.DeleteCoreQuestion(ctx, DeleteCoreQuestionParams{ID: questionID, PostingID: postingID, RecruiterID: recruiterID})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		return apperror.Wrap(apperror.CodeCoreQuestionInUse, "this question was used in an interview and cannot be deleted", err)
	}
	if err != nil {
		return persistenceError(err)
	}
	if n == 0 {
		return r.postingRefusal(ctx, r.queries, recruiterID, postingID, true)
	}
	return nil
}

// SaveForPosting stores a freshly generated bank. Locking the posting row serializes concurrent
// generate calls, so the second one sees the first one's questions and fails with "exists".
func (r *Repository) SaveForPosting(ctx context.Context, recruiterID, postingID uuid.UUID, contents []string) ([]domain.CoreQuestion, error) {
	var saved []domain.CoreQuestion
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		jdID, err := q.LockPendingPosting(ctx, LockPendingPostingParams{ID: postingID, RecruiterID: recruiterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return r.postingRefusal(ctx, q, recruiterID, postingID, false)
		}
		if err != nil {
			return persistenceError(err)
		}
		saved, err = insertIfEmpty(ctx, q, jdID, contents)
		return err
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// SaveForPractice does the same for a Candidate's own practice JD.
func (r *Repository) SaveForPractice(ctx context.Context, candidateID, jdID uuid.UUID, contents []string) ([]domain.CoreQuestion, error) {
	var saved []domain.CoreQuestion
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		locked, err := q.LockCandidateRoleProfile(ctx, LockCandidateRoleProfileParams{JdID: jdID, CandidateID: candidateID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperror.New(apperror.CodeJDNotFound, "job description not found")
		}
		if err != nil {
			return persistenceError(err)
		}
		saved, err = insertIfEmpty(ctx, q, locked, contents)
		return err
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func insertIfEmpty(ctx context.Context, q *Queries, jdID uuid.UUID, contents []string) ([]domain.CoreQuestion, error) {
	count, err := q.CountCoreQuestions(ctx, jdID)
	if err != nil {
		return nil, persistenceError(err)
	}
	if count > 0 {
		return nil, apperror.New(apperror.CodeQuestionBankExists, "this job description already has a question bank")
	}
	rows, err := q.InsertCoreQuestions(ctx, InsertCoreQuestionsParams{JdID: jdID, Contents: contents})
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.CoreQuestion, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CoreQuestion{ID: row.ID, Content: row.Content})
	}
	return out, nil
}

// postingRefusal explains why a guarded statement affected no row. questionMissing is true for
// per-question statements: a pending posting then means the question itself does not exist.
func (r *Repository) postingRefusal(ctx context.Context, q *Queries, recruiterID, postingID uuid.UUID, questionMissing bool) error {
	row, err := q.GetRecruiterPosting(ctx, GetRecruiterPostingParams{ID: postingID, RecruiterID: recruiterID})
	if err != nil {
		return persistenceError(err)
	}
	if row.Status != domain.PostingPending {
		return apperror.New(apperror.CodeQuestionBankLocked, "the question bank can only be changed while the job posting is pending")
	}
	if questionMissing {
		return apperror.New(apperror.CodeCoreQuestionNotFound, "question not found")
	}
	return apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")
}

func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "question bank persistence failed", err)
}
