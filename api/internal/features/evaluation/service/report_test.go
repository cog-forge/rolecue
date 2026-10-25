package service

import (
	"context"
	"encoding/json"
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

func evaluatedRecord(record *domain.InterviewRecord, score int32, summary string) *domain.InterviewRecord {
	record.EvaluationStatus = domain.EvaluationCompleted
	record.Score = &score
	record.Feedback = &summary
	return record
}

func evaluatedRepository(record *domain.InterviewRecord) *fakeInterviewRepository {
	first := "I would index it."
	feedback := "Name the index type."
	return &fakeInterviewRepository{
		record: record,
		turns:  []domain.TurnRecord{{Position: 1, Question: "How would you index a table?", Answer: &first, Feedback: &feedback}},
		scores: []domain.ReportBreakdown{{MetricID: metricAccuracy.ID, Name: metricAccuracy.Name, Score: 80}},
	}
}

func reportFor(t *testing.T, repo *fakeInterviewRepository, userID uuid.UUID, role string) domain.Report {
	t.Helper()
	report, err := newScoringForTest(t, repo, &fakeScorer{}).Report(context.Background(), userID, role, scoringInterviewID)
	if err != nil {
		t.Fatalf("Report(%s) error = %v", role, err)
	}
	return report
}

func TestReportPracticeCandidateSeesEverything(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedPracticeRecord(), 80, "Solid answers."))
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.Status != domain.EvaluationCompleted || report.OverallScore == nil || *report.OverallScore != 80 {
		t.Fatalf("status = %q, score = %v; want completed with 80", report.Status, report.OverallScore)
	}
	if report.Summary == nil || *report.Summary != "Solid answers." {
		t.Fatalf("Summary = %v, want the summary", report.Summary)
	}
	if len(report.Breakdown) != 1 || report.Breakdown[0].Score != 80 {
		t.Fatalf("Breakdown = %+v, want the metric scores", report.Breakdown)
	}
	if len(report.Questions) != 1 || report.Questions[0].Feedback == nil {
		t.Fatalf("Questions = %+v, want the question with its feedback", report.Questions)
	}
}

func TestReportRecruitmentCandidateSeesOnlyScoreAndStatus(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), 80, "Solid answers."))
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.Status != domain.EvaluationCompleted || report.OverallScore == nil || *report.OverallScore != 80 {
		t.Fatalf("status = %q, score = %v; want completed with 80", report.Status, report.OverallScore)
	}
	if report.Summary != nil || report.Breakdown != nil || report.Questions != nil {
		t.Fatalf("limited view leaked evidence: summary=%v breakdown=%+v questions=%+v", report.Summary, report.Breakdown, report.Questions)
	}

	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"summary":null`, `"breakdown":null`, `"questions":null`} {
		if !strings.Contains(string(body), key) {
			t.Fatalf("JSON %s lacks %s", body, key)
		}
	}
}

func TestReportRecruitmentRecruiterOfThePostingSeesEverything(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), 80, "Solid answers."))
	report := reportFor(t, repo, scoringRecruiterID, recruiterRole)

	if report.Summary == nil || len(report.Breakdown) != 1 || len(report.Questions) != 1 {
		t.Fatalf("recruiter view = %+v, want the full report", report)
	}
}

func TestReportInProgressHasNoEvaluation(t *testing.T) {
	record := completedPracticeRecord()
	record.CompletedAt = nil
	repo := &fakeInterviewRepository{record: record, turns: []domain.TurnRecord{{Position: 1, Question: "Q", Answer: strPtr("A")}}}
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.Status != "in_progress" || report.CompletedAt != nil || report.OverallScore != nil {
		t.Fatalf("report = %+v, want in_progress without a score or completion time", report)
	}
	if report.Summary != nil || report.Breakdown != nil {
		t.Fatalf("summary and breakdown must be null before the evaluation completes, got %v and %+v", report.Summary, report.Breakdown)
	}
	if len(report.Questions) != 1 || report.Questions[0].Feedback != nil {
		t.Fatalf("Questions = %+v, want the answered question without feedback", report.Questions)
	}
}

func TestReportPendingEvaluationKeepsStatusHonest(t *testing.T) {
	repo := &fakeInterviewRepository{record: completedPracticeRecord()}
	report := reportFor(t, repo, scoringCandidateID, candidateRole)

	if report.Status != domain.EvaluationPending {
		t.Fatalf("Status = %q, want pending", report.Status)
	}
	if report.Breakdown != nil || report.Summary != nil {
		t.Fatalf("pending report has evaluation fields: summary=%v breakdown=%+v", report.Summary, report.Breakdown)
	}
	if report.Questions == nil {
		t.Fatal("Questions = nil, want an empty list for the full view")
	}
}

func TestReportHidesTheInterviewFromEveryOtherViewer(t *testing.T) {
	tests := []struct {
		name   string
		record func() *domain.InterviewRecord
		user   uuid.UUID
		role   string
	}{
		{name: "other candidate", record: completedPracticeRecord, user: scoringOtherCandidate, role: candidateRole},
		{name: "other candidate on a recruitment", record: completedRecruitmentRecord, user: scoringOtherCandidate, role: candidateRole},
		{name: "other recruiter", record: completedRecruitmentRecord, user: scoringOtherRecruiter, role: recruiterRole},
		{name: "recruiter of a practice interview", record: completedPracticeRecord, user: scoringRecruiterID, role: recruiterRole},
		{name: "owner with the wrong role", record: completedPracticeRecord, user: scoringCandidateID, role: recruiterRole},
		{name: "admin", record: completedRecruitmentRecord, user: scoringRecruiterID, role: "admin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := evaluatedRepository(evaluatedRecord(tt.record(), 80, "Solid answers."))
			_, err := newScoringForTest(t, repo, &fakeScorer{}).Report(context.Background(), tt.user, tt.role, scoringInterviewID)
			if !apperror.IsCode(err, apperror.CodeInterviewNotFound) {
				t.Fatalf("Report() error = %v, want INTERVIEW_NOT_FOUND", err)
			}
		})
	}
}

func TestReportMissingInterviewIsNotFound(t *testing.T) {
	_, err := newScoringForTest(t, &fakeInterviewRepository{}, &fakeScorer{}).Report(context.Background(), scoringCandidateID, candidateRole, scoringInterviewID)
	if !apperror.IsCode(err, apperror.CodeInterviewNotFound) {
		t.Fatalf("Report() error = %v, want INTERVIEW_NOT_FOUND", err)
	}
}

func TestReportCompletedWithoutScoresHasEmptyBreakdown(t *testing.T) {
	repo := &fakeInterviewRepository{record: evaluatedRecord(completedPracticeRecord(), 0, "Nothing scored.")}
	report := reportFor(t, repo, scoringCandidateID, candidateRole)
	if report.Breakdown == nil || len(report.Breakdown) != 0 {
		t.Fatalf("Breakdown = %#v, want an empty list for a completed evaluation", report.Breakdown)
	}
}

func strPtr(s string) *string { return &s }
