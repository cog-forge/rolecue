package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

const (
	candidateRole          = "candidate"
	reportStatusInProgress = "in_progress"
)

// Report returns an interview's report to the viewers who may see it:
//   - the Candidate who owns a practice session sees all of it;
//   - the Candidate who owns a recruitment session sees only its score and status, because the summary,
//     breakdown and questions are the Recruiter's evidence;
//   - the Recruiter of the posting of a recruitment session sees all of it.
//
// Everyone else gets INTERVIEW_NOT_FOUND, so an interview's existence does not leak.
func (s *Scoring) Report(ctx context.Context, userID uuid.UUID, role string, interviewID uuid.UUID) (domain.Report, error) {
	record, err := s.repository.InterviewRecord(ctx, interviewID)
	if err != nil {
		return domain.Report{}, err
	}
	full, err := reportAccess(record, userID, role)
	if err != nil {
		return domain.Report{}, err
	}
	report := domain.Report{
		InterviewID:  record.ID,
		Type:         record.Type,
		Status:       reportStatus(record),
		OverallScore: record.Score,
		StartedAt:    record.StartedAt,
		CompletedAt:  record.CompletedAt,
	}
	if !full {
		return report, nil
	}
	turns, err := s.repository.InterviewTurns(ctx, interviewID)
	if err != nil {
		return domain.Report{}, err
	}
	report.Questions = reportQuestions(turns)
	if record.EvaluationStatus == domain.EvaluationCompleted {
		scores, err := s.repository.InterviewScores(ctx, interviewID)
		if err != nil {
			return domain.Report{}, err
		}
		if scores == nil {
			scores = []domain.ReportBreakdown{}
		}
		report.Summary = record.Feedback
		report.Breakdown = scores
	}
	return report, nil
}

// reportAccess decides whether the viewer gets the full report. An error means the viewer may not see the
// interview at all.
func reportAccess(record domain.InterviewRecord, userID uuid.UUID, role string) (bool, error) {
	switch {
	case role == candidateRole && record.CandidateID == userID:
		return record.Type == domain.InterviewTypePractice, nil
	case role == recruiterRole && record.Type == domain.InterviewTypeRecruitment && record.RecruiterID != nil && *record.RecruiterID == userID:
		return true, nil
	default:
		return false, domain.ErrInterviewNotFound
	}
}

// reportStatus is in_progress until the interview is completed, then the evaluation status.
func reportStatus(record domain.InterviewRecord) string {
	if record.CompletedAt == nil {
		return reportStatusInProgress
	}
	return record.EvaluationStatus
}

// reportQuestions maps the stored questions to the report. Feedback is nil until the evaluation writes it.
func reportQuestions(turns []domain.TurnRecord) []domain.ReportQuestion {
	questions := make([]domain.ReportQuestion, 0, len(turns))
	for _, turn := range turns {
		questions = append(questions, domain.ReportQuestion(turn))
	}
	return questions
}
