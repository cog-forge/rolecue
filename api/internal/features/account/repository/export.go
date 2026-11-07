package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func readFailed(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "could not read the account data", err)
}

// ExportProfile reads the account's own profile row.
func (r *Repository) ExportProfile(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	var p domain.ExportProfile
	err := r.pool.QueryRow(ctx, `
SELECT id, email, name, COALESCE(role, 'candidate'), image, company_name, company_website, created_at
FROM public.users WHERE id = $1`, id).Scan(&p.ID, &p.Email, &p.FullName, &p.Role, &p.Image, &p.CompanyName, &p.CompanyWebsite, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExportProfile{}, apperror.New(apperror.CodeAccountNotFound, "account not found")
	}
	if err != nil {
		return domain.ExportProfile{}, readFailed(err)
	}
	return p, nil
}

// ExportRoleProfiles lists the account's own Role Profiles, oldest first.
func (r *Repository) ExportRoleProfiles(ctx context.Context, id uuid.UUID) ([]domain.ExportRoleProfile, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, title, status, created_at FROM public.role_profiles WHERE candidate_id = $1 ORDER BY created_at, id`, id)
	if err != nil {
		return nil, readFailed(err)
	}
	defer rows.Close()
	var out []domain.ExportRoleProfile
	for rows.Next() {
		var p domain.ExportRoleProfile
		if err := rows.Scan(&p.ID, &p.Title, &p.Status, &p.CreatedAt); err != nil {
			return nil, readFailed(err)
		}
		out = append(out, p)
	}
	return out, readFailedIf(rows.Err())
}

func readFailedIf(err error) error {
	if err != nil {
		return readFailed(err)
	}
	return nil
}

// ExportApplications lists the Candidate's own Applications, oldest first.
func (r *Repository) ExportApplications(ctx context.Context, id uuid.UUID) ([]domain.ExportApplication, error) {
	rows, err := r.pool.Query(ctx, `
SELECT a.id, jp.title, a.status::text, a.reject_reason, a.interview_deadline, a.created_at, a.updated_at, a.cv_path
FROM public.applications a JOIN public.job_postings jp ON jp.id = a.job_posting_id
WHERE a.candidate_id = $1 ORDER BY a.created_at, a.id`, id)
	if err != nil {
		return nil, readFailed(err)
	}
	defer rows.Close()
	var out []domain.ExportApplication
	for rows.Next() {
		var a domain.ExportApplication
		if err := rows.Scan(&a.ID, &a.JobPostingTitle, &a.Status, &a.RejectReason, &a.InterviewDeadline, &a.CreatedAt, &a.UpdatedAt, &a.CVKey); err != nil {
			return nil, readFailed(err)
		}
		out = append(out, a)
	}
	return out, readFailedIf(rows.Err())
}

// ExportInterviews lists the Candidate's own interviews with turns and, for practice interviews,
// scores. A recruitment interview carries the questions and the Candidate's answers only. A recording
// key is set only for a recording whose upload finished (record_status ready).
func (r *Repository) ExportInterviews(ctx context.Context, id uuid.UUID) ([]domain.ExportInterview, error) {
	rows, err := r.pool.Query(ctx, `
SELECT i.id, i.type::text, i.application_id, i.started_at, i.completed_at, i.end_reason, i.score, i.feedback,
       CASE WHEN i.record_status = 'ready' THEN i.record_path END
FROM public.interviews i
LEFT JOIN public.applications a ON a.id = i.application_id
LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
WHERE a.candidate_id = $1 OR rp.candidate_id = $1
ORDER BY i.started_at, i.id`, id)
	if err != nil {
		return nil, readFailed(err)
	}
	defer rows.Close()
	var out []domain.ExportInterview
	for rows.Next() {
		var i domain.ExportInterview
		if err := rows.Scan(&i.ID, &i.Type, &i.ApplicationID, &i.StartedAt, &i.CompletedAt, &i.EndReason, &i.Score, &i.Feedback, &i.RecordKey); err != nil {
			return nil, readFailed(err)
		}
		i.Turns = []domain.ExportTurn{}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, readFailed(err)
	}
	rows.Close()
	for n := range out {
		item := &out[n]
		if item.Type != "practice" {
			// The report view hides score, feedback and breakdown from a recruitment Candidate.
			item.Score, item.Feedback = nil, nil
		}
		if err := r.loadTurns(ctx, item); err != nil {
			return nil, err
		}
		if item.Type == "practice" {
			if err := r.loadScores(ctx, item); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (r *Repository) loadTurns(ctx context.Context, item *domain.ExportInterview) error {
	rows, err := r.pool.Query(ctx, `
SELECT position, kind, question, answer, feedback, asked_at, answered_at
FROM public.conversation_turns WHERE interview_id = $1 ORDER BY position`, item.ID)
	if err != nil {
		return readFailed(err)
	}
	defer rows.Close()
	for rows.Next() {
		var t domain.ExportTurn
		if err := rows.Scan(&t.Position, &t.Kind, &t.Question, &t.Answer, &t.Feedback, &t.AskedAt, &t.AnsweredAt); err != nil {
			return readFailed(err)
		}
		if item.Type != "practice" {
			t.Feedback = nil
		}
		item.Turns = append(item.Turns, t)
	}
	return readFailedIf(rows.Err())
}

func (r *Repository) loadScores(ctx context.Context, item *domain.ExportInterview) error {
	rows, err := r.pool.Query(ctx, `
SELECT m.name, s.score, s.evidence FROM public.score_details s JOIN public.metrics m ON m.id = s.metric_id
WHERE s.interview_id = $1 ORDER BY m.name`, item.ID)
	if err != nil {
		return readFailed(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s domain.ExportScore
		if err := rows.Scan(&s.Metric, &s.Score, &s.Evidence); err != nil {
			return readFailed(err)
		}
		item.Scores = append(item.Scores, s)
	}
	return readFailedIf(rows.Err())
}

// ExportPostings lists a Recruiter's own Job Postings, oldest first.
func (r *Repository) ExportPostings(ctx context.Context, id uuid.UUID) ([]domain.ExportPosting, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, title, status::text, location, employment_type, salary_min, salary_max, created_at, published_at
FROM public.job_postings WHERE recruiter_id = $1 ORDER BY created_at, id`, id)
	if err != nil {
		return nil, readFailed(err)
	}
	defer rows.Close()
	var out []domain.ExportPosting
	for rows.Next() {
		var p domain.ExportPosting
		if err := rows.Scan(&p.ID, &p.Title, &p.Status, &p.Location, &p.EmploymentType, &p.SalaryMin, &p.SalaryMax, &p.CreatedAt, &p.PublishedAt); err != nil {
			return nil, readFailed(err)
		}
		out = append(out, p)
	}
	return out, readFailedIf(rows.Err())
}
