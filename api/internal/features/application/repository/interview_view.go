package repository

import (
	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// recruiterInterview maps the interview columns of Application Detail. A nil id means the left join
// found no session, so the field is null.
func recruiterInterview(id *uuid.UUID, startedAt, completedAt pgtype.Timestamptz, evaluation *string, score *int32, record *string) *domain.InterviewSummary {
	if id == nil {
		return nil
	}
	completed := timePtr(completedAt)
	return &domain.InterviewSummary{
		ID: *id, StartedAt: startedAt.Time, CompletedAt: completed,
		Status: domain.InterviewStatus(completed, derefString(evaluation)), Score: score,
		RecordStatus: derefString(record),
	}
}

// candidateInterview maps the interview columns of the Candidate's own Application: the score and the
// state only.
func candidateInterview(id *uuid.UUID, completedAt pgtype.Timestamptz, evaluation *string, score *int32) *domain.CandidateInterview {
	if id == nil {
		return nil
	}
	completed := timePtr(completedAt)
	return &domain.CandidateInterview{
		ID: *id, CompletedAt: completed,
		Status: domain.InterviewStatus(completed, derefString(evaluation)), Score: score,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
