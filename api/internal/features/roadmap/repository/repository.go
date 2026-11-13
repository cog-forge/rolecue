package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotClaimable means the Roadmap is not in a state the caller may start a generation from (or it does
	// not exist); the caller reads the Roadmap to tell which.
	ErrNotClaimable = errors.New("roadmap is not claimable for generation")
	// ErrFenceLost means another generation or a newer state replaced the one the caller holds: its result must be dropped.
	ErrFenceLost = errors.New("roadmap generation fence lost")
)

// Repository stores Roadmaps and their topics. Queries are exported so a feature that must publish a topic
// set together with its own rows (the question bank) can compose them in one transaction with Publish.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func notFound() error { return apperror.New(apperror.CodeJDNotFound, "Role Profile not found.") }

func persistence(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound()
	}
	return apperror.Wrap(apperror.CodeInternal, "Could not save or load the Roadmap.", err)
}

func toRoadmap(id, profile uuid.UUID, status string, created, updated pgtype.Timestamptz) domain.Roadmap {
	return domain.Roadmap{ID: id, RoleProfileID: profile, GenerationStatus: domain.GenerationStatus(status),
		CreatedAt: created.Time, UpdatedAt: updated.Time}
}

func toItem(row RoadmapItem) domain.Item {
	item := domain.Item{ID: row.ID, RoadmapID: row.RoadmapID, Title: row.Title, Position: int(row.Position),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
	if row.Description != nil {
		item.Description = *row.Description
	}
	return item
}

// GetByProfile reads the Roadmap of a profile the candidate owns. A foreign or missing profile is not found.
func (r *Repository) GetByProfile(ctx context.Context, candidate, profile uuid.UUID) (domain.Roadmap, error) {
	row, err := r.queries.GetOwnedRoadmapByProfile(ctx, GetOwnedRoadmapByProfileParams{RoleProfileID: profile, CandidateID: candidate})
	if err != nil {
		return domain.Roadmap{}, persistence(err)
	}
	return toRoadmap(row.ID, row.RoleProfileID, row.GenerationStatus, row.CreatedAt, row.UpdatedAt), nil
}

// EnsureForProfile returns the Roadmap of a profile the candidate owns, creating the pending row if it is
// somehow missing. A foreign or missing profile is not found.
func (r *Repository) EnsureForProfile(ctx context.Context, candidate, profile uuid.UUID) (domain.Roadmap, error) {
	row, err := r.queries.EnsureOwnedRoadmap(ctx, EnsureOwnedRoadmapParams{RoleProfileID: profile, CandidateID: candidate})
	if err != nil {
		return domain.Roadmap{}, persistence(err)
	}
	return toRoadmap(row.ID, row.RoleProfileID, row.GenerationStatus, row.CreatedAt, row.UpdatedAt), nil
}

// ListItems reads the topics of the candidate's profile in display order.
func (r *Repository) ListItems(ctx context.Context, candidate, profile uuid.UUID) ([]domain.Item, error) {
	rows, err := r.queries.ListOwnedRoadmapItemsByProfile(ctx, ListOwnedRoadmapItemsByProfileParams{RoleProfileID: profile, CandidateID: candidate})
	if err != nil {
		return nil, persistence(err)
	}
	items := make([]domain.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, toItem(RoadmapItem(row)))
	}
	return items, nil
}

// GetItem reads one topic, only when it belongs to the profile and the profile to the candidate.
func (r *Repository) GetItem(ctx context.Context, candidate, profile, item uuid.UUID) (domain.Item, error) {
	row, err := r.queries.GetOwnedRoadmapItem(ctx, GetOwnedRoadmapItemParams{ID: item, RoleProfileID: profile, CandidateID: candidate})
	if err != nil {
		return domain.Item{}, persistence(err)
	}
	return toItem(RoadmapItem(row)), nil
}

// Claim moves the Roadmap into generating and returns the fence of this generation. It succeeds only from one of
// the allowed states, or from generating when the previous worker has been silent since staleBefore. Two
// concurrent claims cannot both succeed: the second finds generating and gets ErrNotClaimable.
func (r *Repository) Claim(ctx context.Context, roadmap uuid.UUID, allowedFrom []domain.GenerationStatus, staleBefore time.Time) (domain.Fence, error) {
	from := make([]string, len(allowedFrom))
	for i, status := range allowedFrom {
		from[i] = string(status)
	}
	updated, err := r.queries.ClaimRoadmapGeneration(ctx, ClaimRoadmapGenerationParams{ID: roadmap, AllowedFrom: from,
		StaleBefore: pgtype.Timestamptz{Time: staleBefore, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fence{}, ErrNotClaimable
	}
	if err != nil {
		return domain.Fence{}, persistence(err)
	}
	return domain.Fence{UpdatedAt: updated.UpdatedAt.Time, Previous: domain.GenerationStatus(updated.PreviousStatus)}, nil
}

// Fail marks the generation failed, but only for the holder of the fence; a stale worker gets ErrFenceLost.
func (r *Repository) Fail(ctx context.Context, roadmap uuid.UUID, fence domain.Fence) error {
	return r.finish(ctx, r.queries, roadmap, fence, domain.StatusFailed)
}

// Restore puts the Roadmap back to ready after a regeneration failed: the topics and the bank it replaced are
// still stored and valid. Like Fail it only works for the holder of the fence.
func (r *Repository) Restore(ctx context.Context, roadmap uuid.UUID, fence domain.Fence) error {
	return r.finish(ctx, r.queries, roadmap, fence, domain.StatusReady)
}

// ProfileState reads the generation-relevant facts of a profile the candidate owns; a foreign or missing
// profile is not found.
func (r *Repository) ProfileState(ctx context.Context, candidate, profile uuid.UUID) (domain.ProfileState, error) {
	row, err := r.queries.GetOwnedProfileState(ctx, GetOwnedProfileStateParams{RoleProfileID: profile, CandidateID: candidate})
	if err != nil {
		return domain.ProfileState{}, persistence(err)
	}
	return domain.ProfileState{JDID: row.JdID, Confirmed: row.Confirmed, HasSessions: row.HasSessions}, nil
}

func (r *Repository) finish(ctx context.Context, q *Queries, roadmap uuid.UUID, fence domain.Fence, status domain.GenerationStatus) error {
	rows, err := q.FinishRoadmapGeneration(ctx, FinishRoadmapGenerationParams{ID: roadmap, GenerationStatus: string(status),
		Fence: pgtype.Timestamptz{Time: fence.UpdatedAt, Valid: true}})
	if err != nil {
		return persistence(err)
	}
	if rows == 0 {
		return ErrFenceLost
	}
	return nil
}

// PublishHook adds rows of another feature to the transaction of Publish, after the new topics exist and before
// the Roadmap becomes ready. An error rolls the whole publish back.
type PublishHook func(ctx context.Context, tx pgx.Tx, items []domain.Item) error

// Publish replaces the topics of the Roadmap with the given ones and marks it ready in one transaction. The
// previous topics stay until this commit, and nothing changes when the fence is lost or the hook fails. The hook
// may be nil. Questions that pointed at replaced topics are detached (NULL), never left dangling.
func (r *Repository) Publish(ctx context.Context, roadmap uuid.UUID, fence domain.Fence, items []domain.NewItem, hook PublishHook) ([]domain.Item, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, persistence(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	// The profile's practice start lock comes first, the Roadmap row second: a start (which takes the same lock
	// before it reads the bank) and a publish (which replaces the topics and the bank) never interleave, and
	// both take their locks in this order, so they cannot deadlock.
	owner, err := q.GetRoadmapOwner(ctx, roadmap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFenceLost
	}
	if err != nil {
		return nil, persistence(err)
	}
	if err = q.LockProfilePracticeStart(ctx, PracticeStartLockKey(owner.CandidateID, owner.RoleProfileID)); err != nil {
		return nil, persistence(err)
	}
	current, err := q.GetRoadmapForUpdate(ctx, roadmap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFenceLost
	}
	if err != nil {
		return nil, persistence(err)
	}
	if current.GenerationStatus != string(domain.StatusGenerating) || !current.UpdatedAt.Time.Equal(fence.UpdatedAt) {
		return nil, ErrFenceLost
	}
	if err = q.DeleteRoadmapItems(ctx, roadmap); err != nil {
		return nil, persistence(err)
	}
	stored := make([]domain.Item, 0, len(items))
	for _, item := range items {
		var description *string
		if item.Description != "" {
			description = &item.Description
		}
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		row, err := q.InsertRoadmapItem(ctx, InsertRoadmapItemParams{ID: item.ID, RoadmapID: roadmap, Title: item.Title, Description: description, Position: int32(item.Position)})
		if err != nil {
			return nil, persistence(err)
		}
		stored = append(stored, toItem(row))
	}
	if hook != nil {
		if err = hook(ctx, tx, stored); err != nil {
			return nil, err
		}
	}
	if err = r.finish(ctx, q, roadmap, fence, domain.StatusReady); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, persistence(err)
	}
	return stored, nil
}

// InPracticeLock runs fn in a transaction that holds the profile's practice start lock: nothing that takes the
// same lock (a practice start, a Roadmap publish) runs while it does. The transaction commits when fn
// returns nil and rolls back otherwise. The profile must belong to the candidate.
func (r *Repository) InPracticeLock(ctx context.Context, candidate, profile uuid.UUID, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return persistence(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := r.queries.WithTx(tx)
	if _, err = q.GetOwnedProfileState(ctx, GetOwnedProfileStateParams{RoleProfileID: profile, CandidateID: candidate}); err != nil {
		return persistence(err)
	}
	if err = q.LockProfilePracticeStart(ctx, PracticeStartLockKey(candidate, profile)); err != nil {
		return persistence(err)
	}
	if err = fn(ctx, tx); err != nil {
		return err
	}
	return persistence(tx.Commit(ctx))
}

// PracticeStartLockKey is the advisory lock key a practice start takes for a candidate and a profile. It must
// stay equal to the key of the interview repository's start (candidate id, colon, profile id).
func PracticeStartLockKey(candidate, profile uuid.UUID) string {
	return candidate.String() + ":" + profile.String()
}

// RecordAssessment links an answered core question of a practice session to its topic. It reports whether a
// row was written: false means a retry already recorded it, or the interview, topic, candidate or question
// mapping do not agree (nothing is written then).
func (r *Repository) RecordAssessment(ctx context.Context, candidate, interview, item uuid.UUID, questionPosition int) (bool, error) {
	rows, err := r.queries.InsertRoadmapAssessment(ctx, InsertRoadmapAssessmentParams{RoadmapItemID: item, InterviewID: interview,
		QuestionPosition: int32(questionPosition), CandidateID: candidate})
	if err != nil {
		return false, persistence(err)
	}
	return rows == 1, nil
}
