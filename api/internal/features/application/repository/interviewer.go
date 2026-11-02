package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CompanyInterviewer(ctx context.Context, candidateID, applicationID, sessionID uuid.UUID) (domain.InterviewerRecord, error) {
	row, err := r.queries.GetCompanyInterviewer(ctx, GetCompanyInterviewerParams{CandidateID: candidateID, ApplicationID: applicationID, SessionID: sessionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InterviewerRecord{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return domain.InterviewerRecord{}, internal(err)
	}
	return domain.InterviewerRecord{Name: row.Name, VoiceName: row.VoiceName, ModelPath: row.ModelPath, Deadline: row.InterviewDeadline.Time}, nil
}
