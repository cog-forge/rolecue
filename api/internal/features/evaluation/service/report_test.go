package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

var (
	scoringOtherCandidate = uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	scoringOtherRecruiter = uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff")
)

func ptr[T any](v T) *T { return &v }

// evaluatedRecord marks a record completed by a rubric evaluation with the given overall and outcome.
func evaluatedRecord(record *domain.InterviewRecord, overall *float64, outcome string, reason *string) *domain.InterviewRecord {
	record.EvaluationStatus = domain.EvaluationCompleted
	record.Score = overall
	record.ScoreOutcome = &outcome
	record.ScoreWithheldReason = reason
	record.Feedback = ptr("Solid answers.")
	record.QuestionCount = 3
	return record
}

// storedWorkedExample is what the worked example stores, plus a third question that was not answered. Q01 is
// graded with a private rubric (systemRubric false) and Q02 with a system template.
func storedWorkedExample(systemRubric bool) domain.StoredResults {
	crit := func(q int32, id uuid.UUID, name string, metric domain.Metric, max, points int32, evidence *string, at *int32) domain.StoredCriterion {
		return domain.StoredCriterion{QuestionPosition: q, CriterionID: id, Name: name, MetricID: metric.ID, MetricName: metric.Name,
			MaxPoints: max, Points: points, Evidence: evidence, EvidencePosition: at, Feedback: ptr("Why.")}
	}
	return domain.StoredResults{
		Metrics: []domain.StoredMetric{
			{MetricID: metricAccuracy.ID, Name: metricAccuracy.Name, Score: ptr(70.0), Weight: ptr(60.0), Questions: ptr(int32(2))},
			{MetricID: metricSolving.ID, Name: metricSolving.Name, Score: ptr(50.0), Weight: ptr(40.0), Questions: ptr(int32(2))},
		},
		Questions: []domain.StoredQuestion{
			{Position: 1, Score: ptr(70.0), HasRubric: true, RubricName: ptr("Private R01"), SystemRubric: systemRubric},
			{Position: 2, Score: ptr(50.0), HasRubric: true, RubricName: ptr("System R01"), SystemRubric: true},
			{Position: 3, HasRubric: true, RubricName: ptr("System R01"), SystemRubric: true},
		},
		Criteria: []domain.StoredCriterion{
			crit(1, critC01, "Mechanism", metricAccuracy, 2, 2, ptr("B-tree index"), ptr(int32(1))),
			crit(1, critC02, "Limits", metricAccuracy, 3, 2, ptr("Writes get slower."), ptr(int32(2))),
			crit(1, critC03, "Recommendation", metricSolving, 5, 3, ptr("on the filtered column"), ptr(int32(1))),
			crit(2, critC01, "Mechanism", metricAccuracy, 2, 1, ptr("read-through cache"), ptr(int32(3))),
			crit(2, critC02, "Limits", metricAccuracy, 3, 2, ptr("with a TTL"), ptr(int32(3))),
			crit(2, critC03, "Recommendation", metricSolving, 5, 2, ptr("Use a read-through cache"), ptr(int32(3))),
		},
	}
}

func evaluatedRepository(record *domain.InterviewRecord) *fakeInterviewRepository {
	turns := answeredTurns()
	turns[0].Feedback = ptr("Name the index type.")
	return &fakeInterviewRepository{record: record, turns: turns, results: storedWorkedExample(false)}
}

func reportFor(t *testing.T, repo *fakeInterviewRepository, userID uuid.UUID, role string) domain.Report {
	t.Helper()
	report, err := newScoringForTest(t, repo, pointsGrader(nil)).Report(context.Background(), userID, role, scoringInterviewID)
	if err != nil {
		t.Fatalf("Report(%s) error = %v", role, err)
	}
	return report
}

func TestReportRecruiterSeesHowTheOverallWasReached(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), ptr(62.0), "partial", nil))
	report := reportFor(t, repo, scoringRecruiterID, recruiterRole)

	if report.OverallScore == nil || *report.OverallScore != 62 || report.Scoring == nil {
		t.Fatalf("report = %+v, want the overall 62 with its explanation", report)
	}
	s := report.Scoring
	if s.Method != domain.ScoringMethodRubric || s.Outcome == nil || *s.Outcome != "partial" || s.AssignedQuestions != 3 ||
		s.GradedQuestions != 2 || s.WeightTotal != 100 || s.Decimals != 2 {
		t.Fatalf("scoring = %+v, want rubric, partial, 2 of 3 graded, weight total 100, 2 decimals", s)
	}
	if len(report.Breakdown) != 2 || *report.Breakdown[0].Score != 70 || *report.Breakdown[0].Weight != 60 || *report.Breakdown[0].QuestionCount != 2 {
		t.Fatalf("breakdown = %+v, want the session metrics with their weights", report.Breakdown)
	}
	if len(report.GradedQuestions) != 3 {
		t.Fatalf("graded questions = %+v, want every assigned question", report.GradedQuestions)
	}
	q1 := report.GradedQuestions[0]
	if q1.Status != domain.QuestionGraded || q1.Total == nil || *q1.Total != 70 || q1.Earned != 7 || q1.Max != 10 ||
		q1.RubricName == nil || *q1.RubricName != "Private R01" || len(q1.Criteria) != 3 || q1.Criteria[0].Name != "Mechanism" {
		t.Fatalf("question 1 = %+v, want 7/10 = 70 with the private rubric visible to its owner", q1)
	}
	// The question metrics are 100 x earned / max of the question's criteria of each metric: 4/5 and 3/5.
	if len(q1.Metrics) != 2 || q1.Metrics[0].Score != 80 || q1.Metrics[0].Earned != 4 || q1.Metrics[0].Max != 5 || q1.Metrics[1].Score != 60 {
		t.Fatalf("question 1 metrics = %+v, want Technical Accuracy 80 (4/5) and Problem-Solving 60 (3/5)", q1.Metrics)
	}
	if c := q1.Criteria[1]; c.Points != 2 || c.MaxPoints != 3 || c.Evidence == nil || c.EvidencePosition == nil || *c.EvidencePosition != 2 {
		t.Fatalf("criterion = %+v, want 2/3 with its quote from the follow-up", c)
	}
	if q3 := report.GradedQuestions[2]; q3.Status != domain.QuestionNotAnswered || q3.Total != nil || len(q3.Criteria) != 0 {
		t.Fatalf("question 3 = %+v, want not answered", q3)
	}
	if report.InjectionFlagged == nil || report.SummarySource != domain.SummarySourceAI {
		t.Fatalf("report = %+v, want the flag and the summary source", report)
	}
}

func TestReportPracticeCandidateSeesTheExplanationButNoPrivateRubricText(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedPracticeRecord(), ptr(62.0), "complete", nil))
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.OverallScore == nil || report.Scoring == nil || len(report.GradedQuestions) != 3 || len(report.Questions) != 3 {
		t.Fatalf("report = %+v, want the full practice report", report)
	}
	q1, q2 := report.GradedQuestions[0], report.GradedQuestions[1]
	if q1.RubricName != nil || q1.Criteria[0].Name != "" {
		t.Fatalf("question 1 = %+v, want no text of a non-system rubric for a Candidate", q1)
	}
	if q2.RubricName == nil || q2.Criteria[0].Name != "Mechanism" {
		t.Fatalf("question 2 = %+v, want the system rubric text visible", q2)
	}
}

func TestReportRecruitmentCandidateSeesOnlyStatus(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), ptr(62.0), "complete", nil))
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.Status != domain.EvaluationCompleted || report.OverallScore != nil || report.Scoring != nil || report.Breakdown != nil ||
		report.GradedQuestions != nil || report.Questions != nil || report.Summary != nil || report.InjectionFlagged != nil {
		t.Fatalf("report = %+v, want the status only", report)
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"overall_score", "injection_flagged", "summary_source", "Mechanism", "B-tree", "62"} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("limited report %s leaks %q", body, leaked)
		}
	}
}

func TestReportWithheldOverallExplainsWhy(t *testing.T) {
	record := evaluatedRecord(completedPracticeRecord(), nil, "withheld", ptr("missing_metric_coverage"))
	repo := evaluatedRepository(record)
	repo.results.Metrics = append(repo.results.Metrics, domain.StoredMetric{MetricID: metricClarity.ID, Name: metricClarity.Name, Weight: ptr(1.0), Questions: ptr(int32(0))})
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.OverallScore != nil || report.Scoring.Outcome == nil || *report.Scoring.Outcome != "withheld" ||
		report.Scoring.WithheldReason == nil || *report.Scoring.WithheldReason != "missing_metric_coverage" {
		t.Fatalf("report = %+v, want a withheld overall with its reason", report)
	}
	if last := report.Breakdown[2]; last.Score != nil || last.Weight == nil {
		t.Fatalf("breakdown = %+v, want the uncovered metric listed without a score", report.Breakdown)
	}
}

func TestReportMarksAnAnsweredQuestionWithoutRubricNotGradable(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedPracticeRecord(), ptr(72.0), "partial", nil))
	repo.results.Criteria = repo.results.Criteria[:3]
	repo.results.Questions[1] = domain.StoredQuestion{Position: 2}
	report := reportFor(t, repo, scoringCandidateID, candidateRole)
	if q2 := report.GradedQuestions[1]; q2.Status != domain.QuestionNotGradable || q2.RubricName != nil {
		t.Fatalf("question 2 = %+v, want not gradable", q2)
	}
}

func TestReportLegacyEvaluationHasNoCriterionDetail(t *testing.T) {
	record := completedPracticeRecord()
	record.EvaluationStatus = domain.EvaluationCompleted
	record.Score = ptr(80.0)
	repo := &fakeInterviewRepository{record: record, turns: answeredTurns(), results: domain.StoredResults{
		Metrics: []domain.StoredMetric{{MetricID: metricAccuracy.ID, Name: metricAccuracy.Name, Score: ptr(80.0), Evidence: ptr("I would add")}},
	}}
	report := reportFor(t, repo, scoringCandidateID, candidateRole)
	if report.Scoring.Method != domain.ScoringMethodLegacy || report.Scoring.Outcome != nil || len(report.GradedQuestions) != 0 ||
		*report.OverallScore != 80 || report.Breakdown[0].Weight != nil {
		t.Fatalf("report = %+v, want a legacy report without criterion results", report)
	}
}

func TestReportBeforeCompletionHasNoScores(t *testing.T) {
	for _, status := range []string{domain.EvaluationPending, domain.EvaluationFailed} {
		record := completedPracticeRecord()
		record.EvaluationStatus = status
		repo := evaluatedRepository(record)
		report := reportFor(t, repo, scoringCandidateID, candidateRole)
		if report.Status != status || report.OverallScore != nil || report.Scoring != nil || report.Breakdown != nil || report.GradedQuestions != nil {
			t.Fatalf("%s report = %+v, want no scores", status, report)
		}
		if len(report.Questions) != 3 {
			t.Fatalf("%s report questions = %+v, want the answers", status, report.Questions)
		}
	}
	record := completedPracticeRecord()
	record.CompletedAt = nil
	if report := reportFor(t, evaluatedRepository(record), scoringCandidateID, candidateRole); report.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", report.Status)
	}
}

func TestReportHidesTheInterviewFromEveryOtherViewer(t *testing.T) {
	practice := evaluatedRepository(evaluatedRecord(completedPracticeRecord(), ptr(62.0), "complete", nil))
	recruitment := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), ptr(62.0), "complete", nil))
	tests := []struct {
		name   string
		repo   *fakeInterviewRepository
		userID uuid.UUID
		role   string
	}{
		{"another candidate", practice, scoringOtherCandidate, candidateRole},
		{"a recruiter on practice", practice, scoringRecruiterID, recruiterRole},
		{"another recruiter", recruitment, scoringOtherRecruiter, recruiterRole},
		{"the candidate id with the recruiter role", recruitment, scoringCandidateID, recruiterRole},
		{"an admin", recruitment, scoringRecruiterID, "admin"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newScoringForTest(t, tc.repo, pointsGrader(nil)).Report(context.Background(), tc.userID, tc.role, scoringInterviewID)
			var appErr *apperror.AppError
			if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInterviewNotFound {
				t.Fatalf("error = %v, want INTERVIEW_NOT_FOUND", err)
			}
		})
	}
}

func TestReportNeverCarriesTheExpectedAnswer(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), ptr(62.0), "complete", nil))
	repo.questions = gradedQuestions(3)
	report := reportFor(t, repo, scoringRecruiterID, recruiterRole)
	body, _ := json.Marshal(report)
	if strings.Contains(string(body), "EXPECTED-SECRET-REFERENCE") || strings.Contains(string(body), "expected_answer") || strings.Contains(string(body), "scoring_guide") {
		t.Fatalf("report %s carries grading-only data", body)
	}
}
