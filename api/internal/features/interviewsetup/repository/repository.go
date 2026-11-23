// Package repository stores the persisted pre-interview setup (KAN-231) and reads the live facts its
// readiness is computed from. Reads never write; a save writes only interview_setups.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Constraint names of the foreign keys a save can violate when the thing it points at disappears between the
// check and the write.
const (
	fkRoleProfile = "interview_setups_role_profile_id_fkey"
	fkApplication = "interview_setups_application_id_fkey"
	fkAvatar      = "interview_setups_avatar_id_fkey"
	fkVoice       = "interview_setups_voice_id_fkey"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func persistence(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "interview setup persistence failed", err)
}

// Practice reads the practice facts of a Role Profile the Candidate owns: one snapshot, no write. A foreign
// or missing profile is JD_NOT_FOUND.
func (r *Repository) Practice(ctx context.Context, candidateID, profileID uuid.UUID) (domain.PracticeFacts, error) {
	row, err := r.queries.GetPracticeSetup(ctx, GetPracticeSetupParams{RoleProfileID: profileID, CandidateID: candidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PracticeFacts{}, domain.ErrRoleProfileNotFound
	}
	if err != nil {
		return domain.PracticeFacts{}, persistence(err)
	}
	return domain.PracticeFacts{
		Title: row.Title, Confirmed: row.Status == domain.RoleProfileConfirmed, BankReady: row.BankReady,
		Avatar: choice(row.AvatarID, row.AvatarName, row.AvatarUsable), Voice: choice(row.VoiceID, row.VoiceName, row.VoiceActive),
		Environment: row.Environment, Saved: row.Saved, UpdatedAt: timePtr(row.UpdatedAt),
	}, nil
}

// Recruitment reads the recruitment facts of an Application the Candidate owns. A foreign or missing
// Application is APPLICATION_NOT_FOUND.
func (r *Repository) Recruitment(ctx context.Context, candidateID, applicationID uuid.UUID) (domain.RecruitmentFacts, error) {
	row, err := r.queries.GetRecruitmentSetup(ctx, GetRecruitmentSetupParams{ApplicationID: applicationID, CandidateID: candidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecruitmentFacts{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.RecruitmentFacts{}, persistence(err)
	}
	return domain.RecruitmentFacts{
		Title: row.Title, Status: row.Status, WithinDeadline: row.WithinDeadline, Deadline: timePtr(row.InterviewDeadline),
		PostingAccepting: row.PostingAccepting, Started: row.Started,
		Avatar: choice(row.AvatarID, row.AvatarName, row.AvatarUsable), Voice: choice(row.VoiceID, row.VoiceName, row.VoiceActive),
		Environment: row.Environment, Saved: row.Saved, UpdatedAt: timePtr(row.UpdatedAt),
	}, nil
}

// SavePractice replaces the practice setup of a Role Profile. In one transaction it locks the chosen avatar
// (the Candidate's own, model not blocked) and voice (active) against deletion, then upserts. Nothing is saved
// when either is refused. The caller has already checked ownership, confirmation and the bank.
func (r *Repository) SavePractice(ctx context.Context, candidateID, profileID uuid.UUID, in domain.PracticeInput) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if in.AvatarID != nil {
			if _, err := q.LockChoosableAvatar(ctx, LockChoosableAvatarParams{ID: *in.AvatarID, UserID: candidateID}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrAvatarNotFound
				}
				return err
			}
		}
		if in.VoiceID != nil {
			if _, err := q.LockActiveVoice(ctx, *in.VoiceID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrVoiceNotFound
				}
				return err
			}
		}
		return q.UpsertPracticeSetup(ctx, UpsertPracticeSetupParams{
			RoleProfileID: &profileID, AvatarID: in.AvatarID, VoiceID: in.VoiceID, Environment: in.Environment,
		})
	})
	return mapSaveError(err)
}

// SaveRecruitment replaces the Candidate's environment of an Application's setup. Avatar and voice are never
// written: the Recruiter's are locked on the posting.
func (r *Repository) SaveRecruitment(ctx context.Context, applicationID uuid.UUID, environment *string) error {
	return mapSaveError(r.queries.UpsertRecruitmentSetup(ctx, UpsertRecruitmentSetupParams{
		ApplicationID: &applicationID, Environment: environment,
	}))
}

// mapSaveError turns a foreign key violation (the target, avatar or voice vanished after the check) into the
// not-found error the caller would have got a moment later, and anything else into an internal error.
func mapSaveError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		switch pgErr.ConstraintName {
		case fkRoleProfile:
			return domain.ErrRoleProfileNotFound
		case fkApplication:
			return domain.ErrApplicationNotFound
		case fkAvatar:
			return domain.ErrAvatarNotFound
		case fkVoice:
			return domain.ErrVoiceNotFound
		}
	}
	return persistence(err)
}

// choice is the avatar or voice a row points at, nil when the row has none. The name is read in the same
// statement, so a choice always has one.
func choice(id *uuid.UUID, name *string, usable bool) *domain.Choice {
	if id == nil {
		return nil
	}
	c := &domain.Choice{ID: *id, Usable: usable}
	if name != nil {
		c.Name = *name
	}
	return c
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
