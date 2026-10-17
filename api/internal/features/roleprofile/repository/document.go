package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/google/uuid"
)

func (r *RoleProfileRepository) CreatePDFProfile(ctx context.Context, user uuid.UUID, raw, objectKey string) (domain.RoleProfile, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RoleProfile{}, rolePersistenceError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
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
	return profile, rolePersistenceError(tx.Commit(ctx))
}
