package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/jd/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository stores a Candidate's JD across job_descriptions, role_profiles and requirements.
// Writes touch several tables, so they run in one transaction.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func (r *Repository) Create(ctx context.Context, candidateID uuid.UUID, record domain.Record) (domain.Record, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		levelID, err := levelID(ctx, q, record.Seniority)
		if err != nil {
			return err
		}
		id, err := q.InsertJobDescription(ctx, InsertJobDescriptionParams{ContentPath: record.ContentPath, LevelID: levelID, RefinementNote: record.RefinementNote})
		if err != nil {
			return err
		}
		createdAt, err := q.InsertRoleProfile(ctx, InsertRoleProfileParams{JdID: id, CandidateID: candidateID, Title: record.Title})
		if err != nil {
			return err
		}
		if err := saveRequirements(ctx, q, id, record.Requirements); err != nil {
			return err
		}
		record.ID, record.CreatedAt = id, createdAt.Time
		return nil
	})
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	return record, nil
}

func (r *Repository) List(ctx context.Context, candidateID uuid.UUID, limit, offset int32) ([]domain.ListItem, int64, error) {
	rows, err := r.queries.ListJDs(ctx, ListJDsParams{CandidateID: candidateID, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountJDs(ctx, candidateID)
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	result := make([]domain.ListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.ListItem{ID: row.JdID, Title: row.Title, SeniorityLevel: seniority(row.LevelName), CreatedAt: row.CreatedAt.Time})
	}
	return result, total, nil
}

func (r *Repository) Get(ctx context.Context, candidateID, id uuid.UUID) (domain.Record, error) {
	row, err := r.queries.GetJD(ctx, GetJDParams{ID: id, CandidateID: candidateID})
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	requirements, err := r.queries.ListRequirements(ctx, id)
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	record := domain.Record{ID: row.ID, ContentPath: row.ContentPath, Title: row.Title, Seniority: seniority(row.LevelName), RefinementNote: row.RefinementNote,
		HasQuestionBank: row.HasQuestionBank, CreatedAt: row.CreatedAt.Time, Requirements: make([]domain.RequirementRow, 0, len(requirements))}
	for _, req := range requirements {
		record.Requirements = append(record.Requirements, domain.RequirementRow{Name: req.Name, Target: req.Target, Type: domain.Requirement(req.Type), EvidenceText: req.EvidenceText})
	}
	return record, nil
}

// Update replaces the reviewed fields. The requirements are rewritten as a whole, the same way
// the Candidate edits them in review. Once a question bank exists they are locked: the bank was
// generated from exactly these requirements and note.
func (r *Repository) Update(ctx context.Context, candidateID, id uuid.UUID, record domain.Record) (domain.Record, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		owned, err := q.LockOwnedJD(ctx, LockOwnedJDParams{ID: id, CandidateID: candidateID})
		if err != nil {
			return err
		}
		if owned.HasQuestionBank {
			return domain.ErrRequirementsLocked
		}
		levelID, err := levelID(ctx, q, record.Seniority)
		if err != nil {
			return err
		}
		if err := q.UpdateJobDescription(ctx, UpdateJobDescriptionParams{ID: id, LevelID: levelID, RefinementNote: record.RefinementNote}); err != nil {
			return err
		}
		createdAt, err := q.UpdateRoleProfileTitle(ctx, UpdateRoleProfileTitleParams{JdID: id, Title: record.Title})
		if err != nil {
			return err
		}
		if err := q.DeleteRequirements(ctx, id); err != nil {
			return err
		}
		if err := saveRequirements(ctx, q, id, record.Requirements); err != nil {
			return err
		}
		record.ID, record.ContentPath, record.CreatedAt = id, owned.ContentPath, createdAt.Time
		return nil
	})
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	return record, nil
}

// Delete removes the JD with its requirements and core questions. Rows that an interview still
// references make PostgreSQL refuse, which becomes JD_IN_USE and rolls everything back.
func (r *Repository) Delete(ctx context.Context, candidateID, id uuid.UUID) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if _, err := q.LockOwnedJD(ctx, LockOwnedJDParams{ID: id, CandidateID: candidateID}); err != nil {
			return err
		}
		for _, step := range []func(context.Context, uuid.UUID) error{q.DeleteRequirements, q.DeleteCoreQuestions, q.DeleteRoleProfile, q.DeleteJobDescription} {
			if err := step(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})
	var pgErr *pgconn.PgError
	// PostgreSQL can report either foreign_key_violation or restrict_violation.
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		return apperror.Wrap(apperror.CodeJDInUse, "job description is in use", err)
	}
	return persistenceError(err)
}

func levelID(ctx context.Context, q *Queries, level domain.Seniority) (uuid.UUID, error) {
	id, err := q.GetLevelIDByName(ctx, string(level))
	if errors.Is(err, pgx.ErrNoRows) {
		// Migration 000007 seeds every extraction level, so a miss is a deployment problem.
		return uuid.Nil, apperror.Wrap(apperror.CodeInternal, "job description persistence failed", errors.New("seniority level is not seeded: "+string(level)))
	}
	return id, err
}

func saveRequirements(ctx context.Context, q *Queries, jdID uuid.UUID, rows []domain.RequirementRow) error {
	if len(rows) == 0 {
		return nil
	}
	params := InsertRequirementsParams{JdID: jdID}
	for _, row := range rows {
		params.Names = append(params.Names, row.Name)
		params.Targets = append(params.Targets, row.Target)
		params.Types = append(params.Types, string(row.Type))
		evidence := ""
		if row.EvidenceText != nil {
			evidence = *row.EvidenceText
		}
		params.EvidenceTexts = append(params.EvidenceTexts, evidence)
	}
	return q.InsertRequirements(ctx, params)
}

func seniority(levelName string) domain.Seniority {
	return domain.Seniority(strings.ToLower(levelName))
}

func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJDNotFound, "job description not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "job description persistence failed", err)
}
