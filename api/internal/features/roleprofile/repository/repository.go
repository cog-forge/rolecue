package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RoleProfileRepository persists a candidate's Role Profile and its JD data.
type RoleProfileRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRoleProfileRepository(pool *pgxpool.Pool) *RoleProfileRepository {
	return &RoleProfileRepository{pool: pool, queries: New(pool)}
}

func conflict(message string) error { return apperror.New(apperror.CodeRoleConflict, message) }
func rolePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.New(apperror.CodeJDNotFound, "Role Profile not found.")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		return apperror.New(apperror.CodeJDInUse, "This Role Profile is in use and cannot be deleted.")
	}
	return apperror.Wrap(apperror.CodeInternal, "Could not save or load the Role Profile.", err)
}

func readProfile(ctx context.Context, q *Queries, user, id uuid.UUID) (domain.RoleProfile, GetRoleProfileRow, error) {
	row, err := q.GetRoleProfile(ctx, GetRoleProfileParams{ID: id, CandidateID: user})
	if err != nil {
		return domain.RoleProfile{}, row, rolePersistenceError(err)
	}
	items, err := q.ListProfileRequirements(ctx, row.JdID)
	if err != nil {
		return domain.RoleProfile{}, row, rolePersistenceError(err)
	}
	profile := domain.RoleProfile{ID: row.ID, JDID: row.JdID, RawText: row.RawText, ContentPath: row.ContentPath, HasOriginalDocument: row.ContentPath != "",
		RoleReview: domain.RoleReview{Title: row.Title, RefinementNote: row.RefinementNote, Requirements: make([]domain.RoleRequirement, 0, len(items))},
		Status:     domain.ProfileStatus(row.Status), CreatedAt: row.CreatedAt.Time}
	if row.SeniorityLevel != nil {
		value := domain.Seniority(*row.SeniorityLevel)
		profile.SeniorityLevel = &value
	}
	for _, item := range items {
		requirement := domain.RoleRequirement{Name: item.Name, Target: item.Target, Kind: item.Kind, EvidenceText: item.EvidenceText}
		if item.Category != nil {
			value := domain.SkillCategory(*item.Category)
			requirement.Category = &value
		}
		if item.Type != nil {
			value := domain.Requirement(*item.Type)
			requirement.Type = &value
		}
		profile.Requirements = append(profile.Requirements, requirement)
	}
	return profile, row, nil
}

func (r *RoleProfileRepository) GetProfile(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	profile, _, err := readProfile(ctx, r.queries.WithTx(tx), user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	return profile, rolePersistenceError(tx.Commit(ctx))
}

func (r *RoleProfileRepository) withOwned(ctx context.Context, user, id uuid.UUID, action func(*Queries, GetRoleProfileRow) error) (domain.RoleProfile, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	if _, err = q.LockOwnedProfile(ctx, LockOwnedProfileParams{ID: id, CandidateID: user}); err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	row, err := q.GetRoleProfile(ctx, GetRoleProfileParams{ID: id, CandidateID: user})
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	if err = action(q, row); err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	result, _, err := readProfile(ctx, q, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	return result, rolePersistenceError(tx.Commit(ctx))
}

func (r *RoleProfileRepository) CreateProfile(ctx context.Context, user uuid.UUID, raw string) (domain.RoleProfile, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	jd, err := q.CreateProfileJD(ctx, &raw)
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	id, err := q.CreateRoleProfile(ctx, CreateRoleProfileParams{JdID: jd, CandidateID: user})
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	result, _, err := readProfile(ctx, q, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	err = rolePersistenceError(tx.Commit(ctx))
	if err == nil {
		logger.FromContext(ctx).Info("role profile created", logger.Op("roleprofile.create"), logger.String("profile_id", result.ID.String()),
			logger.String("to_status", string(result.Status)))
	}
	return result, err
}

func (r *RoleProfileRepository) ListProfiles(ctx context.Context, user uuid.UUID, limit, offset int32) ([]domain.RoleProfile, int64, error) {
	rows, err := r.queries.ListRoleProfileSummaries(ctx, ListRoleProfileSummariesParams{CandidateID: user, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, rolePersistenceError(err)
	}
	total, err := r.queries.CountRoleProfiles(ctx, user)
	if err != nil {
		return nil, 0, rolePersistenceError(err)
	}
	result := make([]domain.RoleProfile, 0, len(rows))
	for _, row := range rows {
		profile := domain.RoleProfile{ID: row.ID, RoleReview: domain.RoleReview{Title: row.Title}, Status: domain.ProfileStatus(row.Status), CreatedAt: row.CreatedAt.Time}
		if row.SeniorityLevel != nil {
			value := domain.Seniority(*row.SeniorityLevel)
			profile.SeniorityLevel = &value
		}
		result = append(result, profile)
	}
	return result, total, nil
}

func writeReview(ctx context.Context, q *Queries, row GetRoleProfileRow, review domain.RoleReview) error {
	var level *string
	if review.SeniorityLevel != nil {
		value := string(*review.SeniorityLevel)
		level = &value
	}
	if err := q.SaveProfileJD(ctx, SaveProfileJDParams{ID: row.JdID, RefinementNote: &review.RefinementNote, SeniorityLevel: level}); err != nil {
		return err
	}
	if err := q.DeleteProfileRequirements(ctx, row.JdID); err != nil {
		return err
	}
	for _, item := range review.Requirements {
		params := InsertProfileRequirementParams{JdID: row.JdID, Name: item.Name, Target: item.Target, Kind: item.Kind, EvidenceText: item.EvidenceText}
		if item.Category != nil {
			value := string(*item.Category)
			params.Category = &value
		}
		if item.Type != nil {
			value := RequirementType(*item.Type)
			params.Type = &value
		}
		if err := q.InsertProfileRequirement(ctx, params); err != nil {
			return err
		}
	}
	return q.CompleteProfileReview(ctx, CompleteProfileReviewParams{ID: row.ID, Title: review.Title})
}

// logTransition records a Role Profile status change. Call it only after the transaction committed.
func logTransition(ctx context.Context, op string, id uuid.UUID, from, to string) {
	logger.FromContext(ctx).Info("role profile status changed", logger.Op(op), logger.String("profile_id", id.String()),
		logger.String("from_status", from), logger.String("to_status", to))
}

// logConflict records a rejected transition. The same error is also returned to the client.
func logConflict(ctx context.Context, op string, id uuid.UUID, status string) {
	logger.FromContext(ctx).Warn("role profile transition conflict", logger.Op(op), logger.String("profile_id", id.String()),
		logger.String("status", status))
}

func (r *RoleProfileRepository) SaveProfile(ctx context.Context, user, id uuid.UUID, in domain.SaveRoleProfile) (domain.RoleProfile, error) {
	profile, err := r.withOwned(ctx, user, id, func(q *Queries, row GetRoleProfileRow) error {
		if row.Status != "review" {
			logConflict(ctx, "roleprofile.save", id, row.Status)
			return conflict("Analyze and review this Role Profile before saving.")
		}
		return writeReview(ctx, q, row, in.RoleReview)
	})
	if err == nil {
		logger.FromContext(ctx).Info("role profile review saved", logger.Op("roleprofile.save"), logger.String("profile_id", id.String()),
			logger.String("status", string(profile.Status)), logger.Int("requirements", len(profile.Requirements)))
	}
	return profile, err
}

func (r *RoleProfileRepository) ConfirmProfile(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	var from string
	profile, err := r.withOwned(ctx, user, id, func(q *Queries, row GetRoleProfileRow) error {
		from = row.Status
		if row.Status == "confirmed" {
			return nil
		}
		if row.Status != "review" {
			logConflict(ctx, "roleprofile.confirm", id, row.Status)
			return conflict("Analyze and review this Role Profile before confirming.")
		}
		return q.ConfirmRoleProfile(ctx, id)
	})
	if err == nil && from == "confirmed" {
		logger.FromContext(ctx).Debug("role profile already confirmed", logger.Op("roleprofile.confirm"), logger.String("profile_id", id.String()))
	} else if err == nil {
		logTransition(ctx, "roleprofile.confirm", id, from, string(profile.Status))
	}
	return profile, err
}

func (r *RoleProfileRepository) ClaimAnalysis(ctx context.Context, user, id uuid.UUID) (domain.RoleProfile, error) {
	var from string
	profile, err := r.withOwned(ctx, user, id, func(q *Queries, row GetRoleProfileRow) error {
		from = row.Status
		if row.Status == "review" {
			return nil
		}
		if row.Status != "draft" && row.Status != "analyzing" {
			logConflict(ctx, "roleprofile.claim_analysis", id, row.Status)
			return conflict("This Role Profile is not ready for analysis.")
		}
		return q.ClaimProfileAnalysis(ctx, id)
	})
	if err == nil && from != "review" {
		// From analyzing to analyzing means an earlier claim was taken over.
		logTransition(ctx, "roleprofile.claim_analysis", id, from, string(profile.Status))
	}
	return profile, err
}

func (r *RoleProfileRepository) ReleaseAnalysis(ctx context.Context, id uuid.UUID) error {
	err := rolePersistenceError(r.queries.ReleaseProfileAnalysis(ctx, id))
	if err == nil {
		// The query has no row count, so this logs the intended analyzing-to-draft move even if no row matched.
		logTransition(ctx, "roleprofile.release_analysis", id, "analyzing", "draft")
	}
	return err
}

func (r *RoleProfileRepository) CompleteAnalysis(ctx context.Context, user, id uuid.UUID, review domain.RoleReview) (domain.RoleProfile, error) {
	profile, err := r.withOwned(ctx, user, id, func(q *Queries, row GetRoleProfileRow) error {
		if row.Status != "analyzing" {
			logConflict(ctx, "roleprofile.complete_analysis", id, row.Status)
			return conflict("This Role Profile is not being analyzed.")
		}
		return writeReview(ctx, q, row, review)
	})
	if err == nil {
		logTransition(ctx, "roleprofile.complete_analysis", id, "analyzing", string(profile.Status))
	}
	return profile, err
}

func (r *RoleProfileRepository) DeleteProfile(ctx context.Context, user, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return rolePersistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	if _, err = q.LockOwnedProfile(ctx, LockOwnedProfileParams{ID: id, CandidateID: user}); err != nil {
		return rolePersistenceError(err)
	}
	profile, _, err := readProfile(ctx, q, user, id)
	if err != nil {
		return err
	}
	if profile.Status == domain.Analyzing {
		logConflict(ctx, "roleprofile.delete", id, string(profile.Status))
		return conflict("Wait for the current operation before deleting.")
	}
	used, err := q.ProfileHasInterviews(ctx, id)
	if err != nil {
		return rolePersistenceError(err)
	}
	if used {
		logConflict(ctx, "roleprofile.delete", id, string(profile.Status))
		return apperror.New(apperror.CodeJDInUse, "This Role Profile has interviews and cannot be deleted.")
	}
	for _, remove := range []func(context.Context, uuid.UUID) error{q.DeleteProfileQuestions, q.DeleteProfileRequirements} {
		if err = remove(ctx, profile.JDID); err != nil {
			return rolePersistenceError(err)
		}
	}
	if err = q.DeleteOwnedRoleProfile(ctx, id); err != nil {
		return rolePersistenceError(err)
	}
	if err = q.DeleteProfileJD(ctx, profile.JDID); err != nil {
		return rolePersistenceError(err)
	}
	err = rolePersistenceError(tx.Commit(ctx))
	if err == nil {
		logger.FromContext(ctx).Info("role profile deleted", logger.Op("roleprofile.delete"), logger.String("profile_id", id.String()),
			logger.String("from_status", string(profile.Status)))
	}
	return err
}
