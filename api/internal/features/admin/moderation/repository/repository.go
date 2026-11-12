package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	evaluationrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	qbdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository takes the pool, not just DBTX, because a decision runs in one transaction.
type Repository struct {
	pool          *pgxpool.Pool
	queries       *Queries
	questionLimit int
}

// Option configures a Repository.
type Option func(*Repository)

// WithQuestionLimit sets how many core questions one interview session asks, the limit the approval's
// metric-coverage check uses. It must be the value the interview start uses (the default applies when it is
// not set or not positive).
func WithQuestionLimit(limit int) Option {
	return func(r *Repository) { r.questionLimit = evaluationdomain.ResolveQuestionLimit(limit) }
}

func NewRepository(pool *pgxpool.Pool, options ...Option) *Repository {
	r := &Repository{pool: pool, queries: New(pool), questionLimit: evaluationdomain.DefaultQuestionLimit}
	for _, option := range options {
		option(r)
	}
	return r
}

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.PostingSummary, int64, error) {
	status := statusArg(filter.Status)
	from, to := timeArg(filter.From), timeArg(filter.To)
	rows, err := r.queries.ListPostings(ctx, ListPostingsParams{
		Status: status, RecruiterID: filter.RecruiterID, CreatedFrom: from, CreatedTo: to,
		PageLimit: filter.Limit, PageOffset: filter.Offset,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountPostings(ctx, CountPostingsParams{
		Status: status, RecruiterID: filter.RecruiterID, CreatedFrom: from, CreatedTo: to,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.PostingSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PostingSummary{
			ID: row.ID, Title: row.Title, Status: domain.PostingStatus(row.Status), CreatedAt: row.CreatedAt.Time,
			Recruiter: domain.Recruiter{ID: row.RecruiterID, Name: row.RecruiterName, Email: row.RecruiterEmail},
		})
	}
	return out, total, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.ReviewPosting, error) {
	row, err := r.queries.GetPosting(ctx, GetPostingParams{ID: id, MaxRunes: qbdomain.MaxExpectedAnswerRunes, MaxWords: qbdomain.MaxExpectedAnswerWords})
	if err != nil {
		return domain.ReviewPosting{}, persistenceError(err)
	}
	requirements, err := r.queries.ListReviewRequirements(ctx, row.JdID)
	if err != nil {
		return domain.ReviewPosting{}, persistenceError(err)
	}
	confirmed := make([]domain.Requirement, 0, len(requirements))
	for _, requirement := range requirements {
		confirmed = append(confirmed, domain.Requirement{Name: requirement.Name, Target: requirement.Target, Type: requirement.Type})
	}
	coverage, err := evaluationrepository.BankCoverage(ctx, r.pool, row.ID, row.JdID, r.questionLimit)
	if err != nil {
		return domain.ReviewPosting{}, persistenceError(err)
	}
	return domain.ReviewPosting{
		Coverage:    &coverage,
		CompanyName: row.CompanyName, CompanyWebsite: row.CompanyWebsite,
		SeniorityLevel: row.SeniorityLevel, RefinementNote: row.RefinementNote, Requirements: confirmed,
		ID: row.ID, Title: row.Title, Status: domain.PostingStatus(row.Status), RejectReason: row.RejectReason,
		Recruiter: domain.Recruiter{ID: row.RecruiterID, Name: row.RecruiterName, Email: row.RecruiterEmail},
		Location:  row.Location, EmploymentType: row.EmploymentType, SalaryMin: row.SalaryMin, SalaryMax: row.SalaryMax,
		InterviewSlot: row.InterviewSlot, InterviewDuration: row.InterviewDuration,
		RequiredAvatar:         namedRef(row.RequiredAvatarID, row.AvatarName),
		RequiredVoice:          namedRef(row.RequiredVoiceID, row.VoiceName),
		CreatedAt:              row.CreatedAt.Time,
		ContentPath:            row.ContentPath,
		StoredText:             row.RawText,
		CoreQuestionCount:      row.CoreQuestionCount,
		GradingIncompleteCount: row.GradingIncompleteCount,
		ReviewSubmitted:        row.ReviewSubmitted,
		WeightsConfigured:      row.WeightsConfigured,
	}, nil
}

// Decide returns domain.ErrNotPending when the posting is not pending and submitted (the service then reads
// it to tell the cases apart). Approval also needs a non-empty bank (ErrQuestionBankNotReady) in which every
// question has an active permitted rubric and a valid expected answer (ErrGradingIncomplete, KAN-331).
// Rejection needs neither. The checks run after the posting row is locked, in statements of their own, so
// they see any bank change that committed before the lock was granted; every bank change also withdraws
// the submission, which the guard on the locked row then catches.
func (r *Repository) Decide(ctx context.Context, id uuid.UUID, status domain.PostingStatus, rejectReason *string) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		locked, err := q.LockPostingForDecision(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotPending
		}
		if err != nil {
			return err
		}
		if locked.Status != string(domain.PostingStatusPending) || !locked.ReviewSubmitted {
			return domain.ErrNotPending
		}
		if status == domain.PostingStatusOpen {
			count, err := q.CountCoreQuestions(ctx, locked.JdID)
			if err != nil {
				return err
			}
			if count == 0 {
				return domain.ErrQuestionBankNotReady
			}
			incomplete, err := q.CountIncompleteGrading(ctx, CountIncompleteGradingParams{
				JdID: locked.JdID, RecruiterID: locked.RecruiterID, MaxRunes: qbdomain.MaxExpectedAnswerRunes, MaxWords: qbdomain.MaxExpectedAnswerWords})
			if err != nil {
				return err
			}
			if incomplete > 0 {
				return domain.ErrGradingIncomplete
			}
			// Metric coverage (KAN-377): an approved posting must let every Candidate start, and the bank is
			// locked from here on. Read under the posting lock, like the checks above, so a bank, weight or
			// rubric change cannot land between this check and the approval.
			coverage, err := evaluationrepository.BankCoverage(ctx, tx, id, locked.JdID, r.questionLimit)
			if err != nil {
				return err
			}
			if !coverage.Passes {
				return domain.ErrMetricCoverageIncomplete
			}
		}
		n, err := q.DecidePosting(ctx, DecidePostingParams{ID: id, NewStatus: string(status), RejectReason: rejectReason})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotPending
		}
		return nil
	})
	if err != nil && !isDomainError(err) {
		return persistenceError(err)
	}
	return err
}

func isDomainError(err error) bool {
	var appErr *apperror.AppError
	return errors.As(err, &appErr)
}

func statusArg(status *domain.PostingStatus) *string {
	if status == nil {
		return nil
	}
	s := string(*status)
	return &s
}

func timeArg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func namedRef(id *uuid.UUID, name *string) *domain.NamedRef {
	if id == nil {
		return nil
	}
	return &domain.NamedRef{ID: *id, Name: name}
}

func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "job posting moderation persistence failed", err)
}
