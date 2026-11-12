// Package repository reads and deletes the data an account owns. It runs plain SQL because the
// erasure touches most tables of the schema and an export joins several of them.
package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrActiveWork is returned by Erase when the recheck inside the transaction finds new active work.
var ErrActiveWork = errors.New("account has active work")

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// activePostingStatuses are the Job Posting states that are live for Candidates. A pending, rejected or
// closed posting has no work in flight.
const activePostingStatuses = `('open', 'intake_closed')`

// ownedInterviews selects the interviews that disappear with the account ($1): its own Applications,
// the Applications on its Job Postings and its practice Role Profiles.
const ownedInterviews = `
SELECT i.id FROM public.interviews i
WHERE i.application_id IN (
        SELECT a.id FROM public.applications a
        WHERE a.candidate_id = $1
           OR a.job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = $1))
   OR i.role_profile_id IN (SELECT rp.id FROM public.role_profiles rp WHERE rp.candidate_id = $1)`

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Account returns the identity row, or ErrNotFound.
func (r *Repository) Account(ctx context.Context, id uuid.UUID) (domain.Identity, error) {
	var a domain.Identity
	err := r.pool.QueryRow(ctx, `SELECT id, email, COALESCE(role, 'candidate') FROM public.users WHERE id = $1`, id).Scan(&a.ID, &a.Email, &a.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Identity{}, apperror.New(apperror.CodeAccountNotFound, "account not found")
	}
	if err != nil {
		return domain.Identity{}, apperror.Wrap(apperror.CodeInternal, "could not read the account", err)
	}
	return a, nil
}

func activeWork(ctx context.Context, q queryer, id uuid.UUID) (domain.ErasureBlockers, error) {
	var w domain.ErasureBlockers
	err := q.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM public.job_postings WHERE recruiter_id = $1 AND status IN `+activePostingStatuses+`),
  (SELECT count(*) FROM public.applications a JOIN public.job_postings jp ON jp.id = a.job_posting_id
     WHERE jp.recruiter_id = $1)`, id).Scan(&w.OpenPostings, &w.ApplicationsInProgress)
	return w, err
}

// ActiveWork counts the live Job Postings and every Application on a Recruiter's postings, decided or not: erasing
// the account would delete those Candidates' CVs, recordings and history, so it is refused until a person decides.
// (Historical name: ApplicationsInProgress.) A Candidate has none.
func (r *Repository) ActiveWork(ctx context.Context, id uuid.UUID) (domain.ErasureBlockers, error) {
	w, err := activeWork(ctx, r.pool, id)
	if err != nil {
		return domain.ErasureBlockers{}, apperror.Wrap(apperror.CodeInternal, "could not check the account's open work", err)
	}
	return w, nil
}

// StorageKeys lists every object key the account owns: CVs, recordings, avatar models, JD PDFs and the
// keys of issued upload grants (which also covers uploads never attached to anything). Duplicates and
// empty values are removed.
func (r *Repository) StorageKeys(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
SELECT a.cv_path FROM public.applications a
 WHERE a.cv_path IS NOT NULL
   AND (a.candidate_id = $1 OR a.job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = $1))
UNION
SELECT i.record_path FROM public.interviews i WHERE i.record_path IS NOT NULL AND i.id IN (`+ownedInterviews+`)
UNION
SELECT av.model_path FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = $1
UNION
SELECT jd.content_path FROM public.job_descriptions jd
 WHERE jd.content_path IS NOT NULL
   AND jd.id IN (SELECT rp.jd_id FROM public.role_profiles rp WHERE rp.candidate_id = $1
                 UNION SELECT jp.jd_id FROM public.job_postings jp WHERE jp.recruiter_id = $1)
UNION
SELECT g.key::text FROM public.upload_grants g WHERE g.owner_id = $1`, id)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not list the account's stored files", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "could not list the account's stored files", err)
		}
		if key != "" {
			keys = append(keys, key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "could not list the account's stored files", err)
	}
	return keys, nil
}

func ids(ctx context.Context, tx pgx.Tx, sql string, id uuid.UUID) ([]string, error) {
	rows, err := tx.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v uuid.UUID
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v.String())
	}
	return out, rows.Err()
}

// Erase deletes the account and everything it owns in one transaction. The caller has already removed
// the stored objects. It locks the user row and checks again that no work is active, so a posting
// that went live after the first check is refused (ErrActiveWork). An account that is already gone
// is not an error, which makes a retry after a lost response safe.
func (r *Repository) Erase(ctx context.Context, id uuid.UUID) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not start the erasure", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	var email string
	switch scanErr := tx.QueryRow(ctx, `SELECT email FROM public.users WHERE id = $1 FOR UPDATE`, id).Scan(&email); {
	case errors.Is(scanErr, pgx.ErrNoRows):
		return tx.Commit(ctx)
	case scanErr != nil:
		return apperror.Wrap(apperror.CodeInternal, "could not lock the account", scanErr)
	}
	work, err := activeWork(ctx, tx, id)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not check the account's open work", err)
	}
	if !work.None() {
		return ErrActiveWork
	}
	if err = erase(ctx, tx, id, email); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not erase the account", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "could not erase the account", err)
	}
	return nil
}

func erase(ctx context.Context, tx pgx.Tx, id uuid.UUID, email string) error {
	interviews, err := ids(ctx, tx, ownedInterviews, id)
	if err != nil {
		return err
	}
	jds, err := ids(ctx, tx, `SELECT rp.jd_id FROM public.role_profiles rp WHERE rp.candidate_id = $1
UNION SELECT jp.jd_id FROM public.job_postings jp WHERE jp.recruiter_id = $1`, id)
	if err != nil {
		return err
	}
	steps := []struct {
		sql  string
		args []any
	}{
		// An interview the Candidate used stays counted as a used slot of the Recruiter's posting: the
		// number is kept on the posting (no personal data) before the rows that carried it are deleted.
		{`UPDATE public.job_postings jp SET erased_interviews = jp.erased_interviews + used.n
		  FROM (SELECT a.job_posting_id, count(*)::int AS n
		          FROM public.interviews i JOIN public.applications a ON a.id = i.application_id
		         WHERE a.candidate_id = $1 GROUP BY a.job_posting_id) used
		  WHERE jp.id = used.job_posting_id AND jp.recruiter_id <> $1`, []any{id}},
		// The turns are tombstoned before they are deleted so the text is gone from any copy of the
		// rows that outlives the delete (replicas read at the snapshot, long-running readers).
		{`UPDATE public.conversation_turns SET question = $2,
		    answer = CASE WHEN answer IS NULL THEN NULL ELSE $2 END,
		    feedback = CASE WHEN feedback IS NULL THEN NULL ELSE $2 END
		  WHERE interview_id = ANY($1::text[]::uuid[])`, []any{interviews, domain.Tombstone}},
		// Criterion results quote the answers too (KAN-335): tombstoned, then deleted before the interview's
		// questions and turns they reference.
		{`UPDATE public.interview_criterion_scores SET
		    evidence = CASE WHEN evidence IS NULL THEN NULL ELSE $2 END,
		    feedback = CASE WHEN feedback IS NULL THEN NULL ELSE $2 END
		  WHERE interview_id = ANY($1::text[]::uuid[])`, []any{interviews, domain.Tombstone}},
		{`DELETE FROM public.interview_criterion_scores WHERE interview_id = ANY($1::text[]::uuid[])`, []any{interviews}},
		{`DELETE FROM public.score_details WHERE interview_id = ANY($1::text[]::uuid[])`, []any{interviews}},
		{`DELETE FROM public.interview_questions WHERE interview_id = ANY($1::text[]::uuid[])`, []any{interviews}},
		{`DELETE FROM public.conversation_turns WHERE interview_id = ANY($1::text[]::uuid[])`, []any{interviews}},
		{`DELETE FROM public.interviews WHERE id = ANY($1::text[]::uuid[])`, []any{interviews}},
		// Rows of other accounts that point at this account's avatars lose the pointer, not the row.
		{`UPDATE public.interviews SET avatar_id = NULL
		  WHERE avatar_id IN (SELECT av.id FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = $1)`, []any{id}},
		{`UPDATE public.job_postings SET required_avatar_id = NULL
		  WHERE required_avatar_id IN (SELECT av.id FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = $1)`, []any{id}},
		{`DELETE FROM public.applications
		  WHERE candidate_id = $1 OR job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = $1)`, []any{id}},
		{`DELETE FROM public.metrics_percentage WHERE job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = $1)`, []any{id}},
		{`DELETE FROM public.job_postings WHERE recruiter_id = $1`, []any{id}},
		{`DELETE FROM public.role_profiles WHERE candidate_id = $1`, []any{id}},
		{`DELETE FROM public.core_questions WHERE jd_id = ANY($1::text[]::uuid[])`, []any{jds}},
		{`DELETE FROM public.requirements WHERE jd_id = ANY($1::text[]::uuid[])`, []any{jds}},
		{`DELETE FROM public.job_descriptions WHERE id = ANY($1::text[]::uuid[])`, []any{jds}},
		{`DELETE FROM public.avatars WHERE inventory_id IN (SELECT inv.id FROM public.inventories inv WHERE inv.user_id = $1)`, []any{id}},
		{`DELETE FROM public.inventories WHERE user_id = $1`, []any{id}},
		{`DELETE FROM public.wallets WHERE user_id = $1`, []any{id}},
		{`DELETE FROM public.verifications WHERE user_id = $1 OR lower(identifier) = lower($2)`, []any{id, email}},
		// sessions, accounts, two_factors and upload_grants go with the user row (ON DELETE CASCADE).
		{`DELETE FROM public.users WHERE id = $1`, []any{id}},
	}
	for _, step := range steps {
		if _, err := tx.Exec(ctx, step.sql, step.args...); err != nil {
			return err
		}
	}
	return nil
}
