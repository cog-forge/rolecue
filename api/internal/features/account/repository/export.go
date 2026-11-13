package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/dbnum"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func readFailed(err error) error {
	return apperror.Wrap(apperror.CodeInternal, "could not read the account data", err)
}

// ExportProfile reads the account's own profile row.
func (r *Repository) ExportProfile(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	row, err := r.queries.ExportProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExportProfile{}, apperror.New(apperror.CodeAccountNotFound, "account not found")
	}
	if err != nil {
		return domain.ExportProfile{}, readFailed(err)
	}
	return domain.ExportProfile{
		ID: row.ID, Email: row.Email, FullName: row.Name, Role: row.Role, Image: row.Image,
		CompanyName: row.CompanyName, CompanyWebsite: row.CompanyWebsite, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// ExportRoleProfiles lists the account's own Role Profiles, oldest first.
func (r *Repository) ExportRoleProfiles(ctx context.Context, id uuid.UUID) ([]domain.ExportRoleProfile, error) {
	rows, err := r.queries.ExportRoleProfiles(ctx, id)
	if err != nil {
		return nil, readFailed(err)
	}
	var out []domain.ExportRoleProfile
	for _, row := range rows {
		out = append(out, domain.ExportRoleProfile{ID: row.ID, Title: row.Title, Status: row.Status, CreatedAt: row.CreatedAt.Time})
	}
	if err := r.attachRoadmaps(ctx, id, out); err != nil {
		return nil, err
	}
	return out, nil
}

// attachRoadmaps adds each Role Profile's Learning Roadmap: status, topics in display order and the practice
// answers linked to them (KAN-356). Everything is read for the owner only; a profile without a Roadmap row keeps a
// nil Roadmap.
func (r *Repository) attachRoadmaps(ctx context.Context, owner uuid.UUID, profiles []domain.ExportRoleProfile) error {
	if len(profiles) == 0 {
		return nil
	}
	roadmaps, err := r.queries.ExportRoadmaps(ctx, owner)
	if err != nil {
		return readFailed(err)
	}
	items, err := r.queries.ExportRoadmapItems(ctx, owner)
	if err != nil {
		return readFailed(err)
	}
	results, err := r.queries.ExportRoadmapResults(ctx, owner)
	if err != nil {
		return readFailed(err)
	}
	byRoadmap := make(map[uuid.UUID]*domain.ExportRoadmap, len(roadmaps))
	byProfile := make(map[uuid.UUID]*domain.ExportRoadmap, len(roadmaps))
	for _, row := range roadmaps {
		rm := &domain.ExportRoadmap{GenerationStatus: row.GenerationStatus, Topics: []domain.ExportRoadmapTopic{}, Results: []domain.ExportRoadmapResult{}}
		byRoadmap[row.ID] = rm
		byProfile[row.RoleProfileID] = rm
	}
	for _, row := range items {
		if rm := byRoadmap[row.RoadmapID]; rm != nil {
			rm.Topics = append(rm.Topics, domain.ExportRoadmapTopic{ID: row.ID, Title: row.Title, Description: row.Description, Position: row.Position})
		}
	}
	for _, row := range results {
		if rm := byRoadmap[row.RoadmapID]; rm != nil {
			rm.Results = append(rm.Results, domain.ExportRoadmapResult{
				TopicID: row.TopicID, InterviewID: row.InterviewID, QuestionNumber: row.QuestionPosition,
				Score: dbnum.Float(row.Score), LinkedAt: row.CreatedAt.Time,
			})
		}
	}
	for i := range profiles {
		profiles[i].Roadmap = byProfile[profiles[i].ID]
	}
	return nil
}

// ExportApplications lists the Candidate's own Applications, oldest first.
func (r *Repository) ExportApplications(ctx context.Context, id uuid.UUID) ([]domain.ExportApplication, error) {
	rows, err := r.queries.ExportApplications(ctx, id)
	if err != nil {
		return nil, readFailed(err)
	}
	var out []domain.ExportApplication
	for _, row := range rows {
		out = append(out, domain.ExportApplication{
			ID: row.ID, JobPostingTitle: row.Title, Status: row.Status, RejectReason: row.RejectReason,
			InterviewDeadline: timePtr(row.InterviewDeadline), CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time, CVKey: row.CvPath,
		})
	}
	return out, nil
}

// ExportInterviews lists the Candidate's own interviews with turns and, for practice interviews,
// scores. A recruitment interview carries the questions and the Candidate's answers only. A recording
// key is set only for a recording whose upload finished (record_status ready).
func (r *Repository) ExportInterviews(ctx context.Context, id uuid.UUID) ([]domain.ExportInterview, error) {
	rows, err := r.queries.ExportInterviews(ctx, id)
	if err != nil {
		return nil, readFailed(err)
	}
	var out []domain.ExportInterview
	for _, row := range rows {
		out = append(out, domain.ExportInterview{
			ID: row.ID, Type: row.Type, ApplicationID: row.ApplicationID, StartedAt: row.StartedAt.Time,
			CompletedAt: timePtr(row.CompletedAt), EndReason: row.EndReason, Score: dbnum.Float(row.Score),
			ScoreOutcome: row.ScoreOutcome, Feedback: row.Feedback, RecordKey: recordKey(row.RecordKey),
			Turns: []domain.ExportTurn{},
		})
	}
	for n := range out {
		item := &out[n]
		if item.Type != "practice" {
			// The report view hides score, feedback and breakdown from a recruitment Candidate.
			item.Score, item.ScoreOutcome, item.Feedback = nil, nil, nil
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

// recordKey reads the CASE expression of ExportInterviews, which sqlc types as interface{}: the key of a
// ready recording, or NULL.
func recordKey(value interface{}) *string {
	key, ok := value.(string)
	if !ok {
		return nil
	}
	return &key
}

func (r *Repository) loadTurns(ctx context.Context, item *domain.ExportInterview) error {
	rows, err := r.queries.ExportTurns(ctx, item.ID)
	if err != nil {
		return readFailed(err)
	}
	for _, row := range rows {
		t := domain.ExportTurn{
			Position: row.Position, Kind: row.Kind, Question: row.Question, Answer: row.Answer,
			Feedback: row.Feedback, AskedAt: timePtr(row.AskedAt), AnsweredAt: timePtr(row.AnsweredAt),
		}
		if item.Type != "practice" {
			t.Feedback = nil
		}
		item.Turns = append(item.Turns, t)
	}
	return nil
}

func (r *Repository) loadScores(ctx context.Context, item *domain.ExportInterview) error {
	rows, err := r.queries.ExportScores(ctx, item.ID)
	if err != nil {
		return readFailed(err)
	}
	for _, row := range rows {
		item.Scores = append(item.Scores, domain.ExportScore{Metric: row.Metric, Score: dbnum.Float(row.Score), Weight: row.Weight, Evidence: row.Evidence})
	}
	return r.loadCriteria(ctx, item)
}

// loadCriteria adds the graded rubric criteria of a practice interview. The rubric of a practice question is
// a system template, so its criterion names are not anyone's private text; expected answers and scoring
// guides are never exported.
func (r *Repository) loadCriteria(ctx context.Context, item *domain.ExportInterview) error {
	rows, err := r.queries.ExportCriteria(ctx, item.ID)
	if err != nil {
		return readFailed(err)
	}
	for _, row := range rows {
		item.Criteria = append(item.Criteria, domain.ExportCriterion{
			QuestionPosition: row.QuestionPosition, Criterion: row.Criterion, Metric: row.Metric,
			Points: row.Points, MaxPoints: row.MaxPoints, Evidence: row.Evidence, Feedback: row.Feedback,
		})
	}
	return nil
}

// ExportPostings lists a Recruiter's own Job Postings, oldest first.
func (r *Repository) ExportPostings(ctx context.Context, id uuid.UUID) ([]domain.ExportPosting, error) {
	rows, err := r.queries.ExportPostings(ctx, id)
	if err != nil {
		return nil, readFailed(err)
	}
	var out []domain.ExportPosting
	for _, row := range rows {
		out = append(out, domain.ExportPosting{
			ID: row.ID, Title: row.Title, Status: row.Status, Location: row.Location,
			EmploymentType: row.EmploymentType, SalaryMin: row.SalaryMin, SalaryMax: row.SalaryMax,
			CreatedAt: row.CreatedAt.Time, PublishedAt: timePtr(row.PublishedAt),
		})
	}
	return out, nil
}
