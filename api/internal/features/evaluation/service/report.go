package service

import (
	"context"
	"math"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

const (
	candidateRole          = "candidate"
	reportStatusInProgress = "in_progress"
)

// Report returns an interview's report to the viewers who may see it (KAN-336):
//   - the Candidate who owns a practice session sees all of it, except rubric text that is not a system
//     template (a practice bank only uses system templates; this is a second guard);
//   - the Candidate who owns a recruitment session sees only its status (not even the score, which the
//     Recruiter announces), because the summary, scores and answers are the Recruiter's evidence;
//   - the Recruiter of the posting of a recruitment session sees all of it.
//
// Everyone else gets INTERVIEW_NOT_FOUND, so an interview's existence does not leak. No reader ever gets an
// expected answer or a scoring guide: the report does not load them.
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
		InterviewID:   record.ID,
		Type:          record.Type,
		Status:        reportStatus(record),
		StartedAt:     record.StartedAt,
		CompletedAt:   record.CompletedAt,
		EndReason:     record.EndReason,
		QuestionCount: record.QuestionCount,
	}
	if !full {
		return report, nil
	}
	flagged := record.InjectionFlagged
	report.InjectionFlagged = &flagged
	turns, err := s.repository.InterviewTurns(ctx, interviewID)
	if err != nil {
		return domain.Report{}, err
	}
	report.Questions = reportQuestions(turns)
	if record.EvaluationStatus != domain.EvaluationCompleted {
		return report, nil
	}
	results, err := s.repository.InterviewResults(ctx, interviewID)
	if err != nil {
		return domain.Report{}, err
	}
	report.OverallScore = record.Score
	report.Summary = record.Feedback
	if record.Feedback != nil {
		report.SummarySource = domain.SummarySourceAI
	}
	showPrivateRubrics := role == recruiterRole
	report.Scoring, report.Breakdown, report.GradedQuestions = s.explain(record, results, turns, showPrivateRubrics)
	return report, nil
}

// explain turns the stored results into the report's explanation. A completed evaluation without an outcome
// predates rubric grading: it keeps its metric scores and overall and has no criterion results.
func (s *Scoring) explain(record domain.InterviewRecord, results domain.StoredResults, turns []domain.TurnRecord, showPrivateRubrics bool) (*domain.ReportScoring, []domain.ReportBreakdown, []domain.ReportGradedQuestion) {
	scoring := &domain.ReportScoring{
		Method: domain.ScoringMethodRubric, Outcome: record.ScoreOutcome, WithheldReason: record.ScoreWithheldReason,
		AssignedQuestions: record.QuestionCount, Decimals: s.policy.MetricDecimals,
	}
	if record.ScoreOutcome == nil {
		scoring.Method = domain.ScoringMethodLegacy
	}
	breakdown := make([]domain.ReportBreakdown, 0, len(results.Metrics))
	for _, m := range results.Metrics {
		breakdown = append(breakdown, domain.ReportBreakdown{
			MetricID: m.MetricID, Name: m.Name, Score: m.Score, Weight: m.Weight, QuestionCount: m.Questions, Evidence: m.Evidence,
		})
		if m.Weight != nil {
			scoring.WeightTotal += *m.Weight
		}
	}
	if scoring.Method == domain.ScoringMethodLegacy {
		return scoring, breakdown, []domain.ReportGradedQuestion{}
	}
	answered := map[int32]bool{}
	for _, turn := range turns {
		if turn.Answer != nil && strings.TrimSpace(*turn.Answer) != "" {
			answered[turn.QuestionNumber] = true
		}
	}
	criteria := map[int32][]domain.StoredCriterion{}
	for _, c := range results.Criteria {
		criteria[c.QuestionPosition] = append(criteria[c.QuestionPosition], c)
	}
	graded := make([]domain.ReportGradedQuestion, 0, len(results.Questions))
	for _, q := range results.Questions {
		visible := showPrivateRubrics || q.SystemRubric
		item := domain.ReportGradedQuestion{
			QuestionNumber: q.Position, Total: q.Score,
			Metrics: []domain.ReportQuestionMetric{}, Criteria: []domain.ReportCriterion{},
		}
		if visible {
			item.RubricName = q.RubricName
		}
		switch {
		case len(criteria[q.Position]) > 0:
			item.Status = domain.QuestionGraded
			scoring.GradedQuestions++
		case answered[q.Position]:
			item.Status = domain.QuestionNotGradable
		default:
			item.Status = domain.QuestionNotAnswered
		}
		for _, c := range criteria[q.Position] {
			name := c.Name
			if !visible {
				name = ""
			}
			item.Criteria = append(item.Criteria, domain.ReportCriterion{
				CriterionID: c.CriterionID, Name: name, MetricID: c.MetricID, MetricName: c.MetricName,
				Points: c.Points, MaxPoints: c.MaxPoints, Evidence: c.Evidence, EvidencePosition: c.EvidencePosition, Feedback: c.Feedback,
			})
		}
		item.Earned, item.Max, item.Metrics = questionMetrics(criteria[q.Position], s.policy.MetricDecimals)
		graded = append(graded, item)
	}
	return scoring, breakdown, graded
}

// questionMetrics recomputes the metric scores inside one question from its stored criterion points, with the
// calculator's own arithmetic. The points are stored and the criteria are locked once used, so this is the
// value the session metric was built from, not a new judgement.
func questionMetrics(criteria []domain.StoredCriterion, decimals int) (earned, max int32, metrics []domain.ReportQuestionMetric) {
	metrics = []domain.ReportQuestionMetric{}
	if len(criteria) == 0 {
		return 0, 0, metrics
	}
	q := checkedQuestion{}
	names := map[uuid.UUID]string{}
	for _, c := range criteria {
		q.criteria = append(q.criteria, domain.CriterionPoints{CriterionID: c.CriterionID, MetricID: c.MetricID, MaxPoints: c.MaxPoints, Points: c.Points})
		names[c.MetricID] = c.MetricName
	}
	scored, _ := scoreQuestion(q, decimals, math.Pow10(decimals))
	for _, m := range scored.Metrics {
		metrics = append(metrics, domain.ReportQuestionMetric{MetricID: m.MetricID, Name: names[m.MetricID], Earned: m.Earned, Max: m.Max, Score: m.Score})
	}
	return scored.Earned, scored.Max, metrics
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
