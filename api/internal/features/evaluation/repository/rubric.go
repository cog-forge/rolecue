package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Rubric management persistence (KAN-326). Every change runs in one transaction under the rubric's row lock
// (SELECT ... FOR UPDATE), so a stale token, a locked rubric and a referenced delete are decided on the same
// state the write then changes.

const (
	pgForeignKeyViolation = "23503"
	// Foreign keys whose violation means something specific to a rubric request.
	metricForeignKey       = "rubric_criteria_metric_id_fkey"
	questionRubricForeign  = "core_questions_rubric_id_fkey"
	ownerForeignKey        = "rubrics_owner_id_fkey"
	criterionBelongsToItem = "a criterion id does not belong to this rubric"
)

// rubricRow is the shape shared by the generated rows of the list, the read and the by-id read.
type rubricRow struct {
	ID             uuid.UUID
	Name           string
	Description    string
	QuestionType   string
	Scope          string
	OwnerID        pgtype.UUID
	IsActive       bool
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
	CriteriaCount  int32
	MaxPoints      int32
	ReferenceCount int32
}

func (row rubricRow) rubric() domain.Rubric {
	rubric := domain.Rubric{
		ID: row.ID, Name: row.Name, Description: row.Description, QuestionType: row.QuestionType, Scope: row.Scope,
		IsActive: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		CriteriaCount: row.CriteriaCount, MaxPoints: row.MaxPoints, References: row.ReferenceCount,
	}
	if row.OwnerID.Valid {
		owner := uuid.UUID(row.OwnerID.Bytes)
		rubric.OwnerID = &owner
	}
	return rubric
}

func optionalUUID(owner *uuid.UUID) pgtype.UUID {
	if owner == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *owner, Valid: true}
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// ListRubrics returns one page of the rubrics the caller may see, without criteria, and the total.
func (r *Repository) ListRubrics(ctx context.Context, access domain.RubricAccess, query domain.RubricListQuery) ([]domain.Rubric, int64, error) {
	count, err := r.queries.CountVisibleRubrics(ctx, CountVisibleRubricsParams{
		ViewerRole: access.Role, ViewerID: access.UserID, ScopeFilter: optionalText(query.Scope),
		QuestionType: optionalText(query.QuestionType), Status: query.Status,
	})
	if err != nil {
		return nil, 0, rubricPersistenceError(err)
	}
	rows, err := r.queries.ListVisibleRubrics(ctx, ListVisibleRubricsParams{
		ViewerRole: access.Role, ViewerID: access.UserID, ScopeFilter: optionalText(query.Scope),
		QuestionType: optionalText(query.QuestionType), Status: query.Status,
		PageLimit: query.Limit, PageOffset: query.Offset,
	})
	if err != nil {
		return nil, 0, rubricPersistenceError(err)
	}
	out := make([]domain.Rubric, 0, len(rows))
	for _, row := range rows {
		out = append(out, rubricRow(row).rubric())
	}
	return out, count, nil
}

// GetRubric returns a rubric with its criteria, or ErrRubricNotFound when it does not exist or the caller may
// not see it (someone else's private rubric, an archived system rubric for a Recruiter).
func (r *Repository) GetRubric(ctx context.Context, access domain.RubricAccess, id uuid.UUID) (domain.Rubric, error) {
	row, err := r.queries.GetVisibleRubric(ctx, GetVisibleRubricParams{ID: id, ViewerRole: access.Role, ViewerID: access.UserID})
	if err != nil {
		return domain.Rubric{}, rubricPersistenceError(err)
	}
	return withCriteria(ctx, r.queries, rubricRow(row).rubric())
}

func withCriteria(ctx context.Context, q *Queries, rubric domain.Rubric) (domain.Rubric, error) {
	rows, err := q.ListRubricCriteria(ctx, rubric.ID)
	if err != nil {
		return domain.Rubric{}, rubricPersistenceError(err)
	}
	rubric.Criteria = make([]domain.RubricCriterion, 0, len(rows))
	for _, row := range rows {
		rubric.Criteria = append(rubric.Criteria, domain.RubricCriterion{
			ID: row.ID, MetricID: row.MetricID, MetricName: row.MetricName, Name: row.Name,
			MaxPoints: row.MaxPoints, ScoringGuide: row.ScoringGuide, Position: row.Position,
		})
	}
	return rubric, nil
}

// readStored returns a rubric just written in the transaction, criteria included.
func readStored(ctx context.Context, q *Queries, id uuid.UUID) (domain.Rubric, error) {
	row, err := q.GetRubricByID(ctx, id)
	if err != nil {
		return domain.Rubric{}, rubricPersistenceError(err)
	}
	return withCriteria(ctx, q, rubricRow(row).rubric())
}

// CreateRubric stores a rubric and its criteria in one transaction. Criteria are numbered from 1 in the order
// given.
func (r *Repository) CreateRubric(ctx context.Context, write domain.RubricWrite) (domain.Rubric, error) {
	var created domain.Rubric
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		id, err := q.InsertRubric(ctx, InsertRubricParams{
			Name: write.Name, Description: write.Description, QuestionType: write.QuestionType,
			Scope: write.Scope, OwnerID: optionalUUID(write.OwnerID),
		})
		if err != nil {
			return err
		}
		for i, criterion := range write.Criteria {
			if err := q.InsertRubricCriterion(ctx, InsertRubricCriterionParams{
				RubricID: id, MetricID: criterion.MetricID, Name: criterion.Name, MaxPoints: criterion.MaxPoints,
				ScoringGuide: criterion.ScoringGuide, Position: int32(i + 1),
			}); err != nil {
				return err
			}
		}
		created, err = readStored(ctx, q, id)
		return err
	})
	if err != nil {
		return domain.Rubric{}, rubricPersistenceError(err)
	}
	return created, nil
}

// CopyRubric creates an independent rubric from a source the caller may read. The source is read under a
// shared row lock, so it cannot change or disappear while it is copied; the copy has its own rubric row and its
// own criteria rows, and shares nothing with the source afterwards.
func (r *Repository) CopyRubric(ctx context.Context, access domain.RubricAccess, sourceID uuid.UUID, copy domain.RubricCopy) (domain.Rubric, error) {
	var created domain.Rubric
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		source, err := q.ShareVisibleRubric(ctx, ShareVisibleRubricParams{ID: sourceID, ViewerRole: access.Role, ViewerID: access.UserID})
		if err != nil {
			return err
		}
		id, err := q.InsertRubric(ctx, InsertRubricParams{
			Name: copy.Name, Description: source.Description, QuestionType: source.QuestionType,
			Scope: copy.Scope, OwnerID: optionalUUID(copy.OwnerID),
		})
		if err != nil {
			return err
		}
		if err := q.CopyRubricCriteria(ctx, CopyRubricCriteriaParams{NewRubricID: id, SourceRubricID: sourceID}); err != nil {
			return err
		}
		created, err = readStored(ctx, q, id)
		return err
	})
	if err != nil {
		return domain.Rubric{}, rubricPersistenceError(err)
	}
	return created, nil
}

// UpdateRubric applies a validated change under the rubric's row lock:
//  1. the token the client read must still be the stored updated_at (RUBRIC_STALE otherwise);
//  2. a content change is refused while questions reference the rubric (RUBRIC_LOCKED); archiving or restoring
//     is not a content change and is always allowed;
//  3. criteria are replaced as a set: ids that are sent keep their row, the rest are removed or added.
//
// The lock conflicts with the foreign-key lock of a question that is being assigned this rubric, so the
// reference count cannot change under the check.
func (r *Repository) UpdateRubric(ctx context.Context, id uuid.UUID, update domain.RubricUpdate) (domain.Rubric, error) {
	var updated domain.Rubric
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		locked, err := q.LockRubric(ctx, id)
		if err != nil {
			return err
		}
		if !locked.UpdatedAt.Time.Equal(update.ExpectedUpdatedAt) {
			return domain.ErrRubricStale
		}
		if update.ContentChange() {
			references, err := q.CountRubricReferences(ctx, optionalUUID(&id))
			if err != nil {
				return err
			}
			if domain.RubricLocked(references) {
				return domain.ErrRubricLocked
			}
		}
		if err := q.UpdateRubricFields(ctx, UpdateRubricFieldsParams{
			ID: id, Name: update.Name, Description: update.Description, QuestionType: update.QuestionType, IsActive: update.IsActive,
		}); err != nil {
			return err
		}
		if update.Criteria != nil {
			if err := replaceCriteria(ctx, q, id, *update.Criteria); err != nil {
				return err
			}
		}
		updated, err = readStored(ctx, q, id)
		return err
	})
	if err != nil {
		return domain.Rubric{}, rubricPersistenceError(err)
	}
	return updated, nil
}

func replaceCriteria(ctx context.Context, q *Queries, rubricID uuid.UUID, criteria []domain.CriterionWrite) error {
	keep := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		if criterion.ID != nil {
			keep = append(keep, criterion.ID.String())
		}
	}
	if err := q.DeleteRubricCriteriaExcept(ctx, DeleteRubricCriteriaExceptParams{RubricID: rubricID, Keep: keep}); err != nil {
		return err
	}
	for i, criterion := range criteria {
		position := int32(i + 1)
		if criterion.ID == nil {
			if err := q.InsertRubricCriterion(ctx, InsertRubricCriterionParams{
				RubricID: rubricID, MetricID: criterion.MetricID, Name: criterion.Name, MaxPoints: criterion.MaxPoints,
				ScoringGuide: criterion.ScoringGuide, Position: position,
			}); err != nil {
				return err
			}
			continue
		}
		changed, err := q.UpdateRubricCriterion(ctx, UpdateRubricCriterionParams{
			ID: *criterion.ID, RubricID: rubricID, MetricID: criterion.MetricID, Name: criterion.Name,
			MaxPoints: criterion.MaxPoints, ScoringGuide: criterion.ScoringGuide, Position: position,
		})
		if err != nil {
			return err
		}
		if changed == 0 {
			return apperror.New(apperror.CodeValidation, fmt.Sprintf("criteria[%d].id: %s", i, criterionBelongsToItem))
		}
	}
	return nil
}

// DeleteRubric hard-deletes a rubric (its criteria go with it) when no question references it.
func (r *Repository) DeleteRubric(ctx context.Context, id uuid.UUID) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if _, err := q.LockRubric(ctx, id); err != nil {
			return err
		}
		references, err := q.CountRubricReferences(ctx, optionalUUID(&id))
		if err != nil {
			return err
		}
		if references > 0 {
			return domain.ErrRubricInUse
		}
		return q.DeleteRubric(ctx, id)
	})
	return rubricPersistenceError(err)
}

// rubricPersistenceError keeps domain errors, maps a missing row to not found and the foreign keys that carry
// meaning to their stable codes, and turns anything else into an internal error.
func rubricPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrRubricNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		switch pgErr.ConstraintName {
		case metricForeignKey:
			return apperror.Wrap(apperror.CodeEvaluationMetricUnknown, "unknown evaluation metric", err)
		case questionRubricForeign:
			return apperror.Wrap(apperror.CodeRubricInUse, domain.ErrRubricInUse.Message, err)
		case ownerForeignKey:
			return apperror.Wrap(apperror.CodeRubricNotFound, "rubric owner not found", err)
		}
	}
	return apperror.Wrap(apperror.CodeInternal, "rubric persistence failed", err)
}
