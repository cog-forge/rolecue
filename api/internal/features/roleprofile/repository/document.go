package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *RoleProfileRepository) CreatePDFProfile(ctx context.Context, user uuid.UUID, raw, objectKey string) (domain.RoleProfile, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	// A lost completion response must not turn a retry into a second Role Profile.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,20))`, objectKey); err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT rp.id FROM role_profiles rp JOIN job_descriptions jd ON jd.id=rp.jd_id
 WHERE rp.candidate_id=$1 AND jd.content_path=$2`, user, objectKey).Scan(&existing)
	if err == nil {
		// A retried completion returns the profile the first attempt created.
		logger.FromContext(ctx).Info("document role profile already exists, returning it", logger.Op("roleprofile.create_document"),
			logger.String("profile_id", existing.String()))
		profile, _, err := readProfile(ctx, q, user, existing)
		if err != nil {
			return domain.RoleProfile{}, err
		}
		return profile, rolePersistenceError(tx.Commit(ctx))
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	jd, err := q.CreateDocumentJD(ctx, CreateDocumentJDParams{RawText: &raw, ContentPath: &objectKey})
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	id, err := q.CreateRoleProfile(ctx, CreateRoleProfileParams{JdID: jd, CandidateID: user})
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	profile, _, err := readProfile(ctx, q, user, id)
	if err != nil {
		return domain.RoleProfile{}, err
	}
	err = rolePersistenceError(tx.Commit(ctx))
	if err == nil {
		logger.FromContext(ctx).Info("role profile created from document", logger.Op("roleprofile.create_document"),
			logger.String("profile_id", profile.ID.String()), logger.String("to_status", string(profile.Status)))
	}
	return profile, err
}
