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

func TestReportRecruitmentCandidateSeesOnlyStatus(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), 80, "Solid answers."))
	report := reportFor(t, repo, scoringCandidateID, candidateRole)
	if report.Status != domain.EvaluationCompleted || report.OverallScore != nil {
		t.Fatalf("status = %q, score = %v; want completed without the score", report.Status, report.OverallScore)
		t.Fatalf("status = %q, score = %v; want completed with 80", report.Status, report.OverallScore)
	}
	if report.Summary != nil || report.Breakdown != nil || report.Questions != nil || report.InjectionFlagged != nil || report.SummarySource != "" {
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
	for _, key := range []string{"overall_score", "injection_flagged", "summary_source"} {
		if strings.Contains(string(body), key) {
			t.Fatalf("JSON %s leaks %s to the candidate", body, key)
		}
	}
}

func TestReportRecruitmentRecruiterOfThePostingSeesEverything(t *testing.T) {
	repo := evaluatedRepository(evaluatedRecord(completedRecruitmentRecord(), 80, "Solid answers."))
	report := reportFor(t, repo, scoringRecruiterID, recruiterRole)

	if report.Summary == nil || len(report.Breakdown) != 1 || len(report.Questions) != 1 {
		t.Fatalf("recruiter view = %+v, want the full report", report)
	}
	if report.OverallScore == nil || *report.OverallScore != 80 || report.SummarySource != domain.SummarySourceAI || report.InjectionFlagged == nil || *report.InjectionFlagged {
		t.Fatalf("recruiter view = %+v, want the score, an ai summary source and an unset flag", report)
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

func TestReportShowsFollowUpsWithTheirCoreQuestion(t *testing.T) {
	answer := "Refill tokens at a fixed rate."
	repo := evaluatedRepository(evaluatedRecord(completedPracticeRecord(), 80, "Solid answers."))
	repo.turns = append(repo.turns, domain.TurnRecord{
		Position: 2, QuestionNumber: 1, Kind: domain.TurnFollowUp, Question: "How do bursts refill?", Answer: &answer,
	})
	report := reportFor(t, repo, scoringCandidateID, candidateRole)
	if len(report.Questions) != 2 {
		t.Fatalf("Questions = %+v", report.Questions)
	}
	if got := report.Questions[1]; got.Position != 2 || got.QuestionNumber != 1 || got.Kind != domain.TurnFollowUp || got.Answer == nil {
		t.Fatalf("follow-up = %+v", got)
	}
}

// A session the Candidate left early reports how it ended and how many core questions it had, so the
// report can show the unanswered ones; a recruitment Candidate's limited view carries both too.
func TestReportCarriesTheEndReasonAndQuestionCount(t *testing.T) {
	left := "left"
	practice := evaluatedRecord(completedPracticeRecord(), 60, "Partial answers.")
	practice.EndReason, practice.QuestionCount = &left, 8
	report := reportFor(t, evaluatedRepository(practice), scoringCandidateID, candidateRole)
	if report.EndReason == nil || *report.EndReason != "left" || report.QuestionCount != 8 || len(report.Questions) != 1 {
		t.Fatalf("end_reason = %v, question_count = %d, questions = %d", report.EndReason, report.QuestionCount, len(report.Questions))
	}

	abandoned := "abandoned"
	recruitment := completedRecruitmentRecord()
	recruitment.EndReason, recruitment.QuestionCount = &abandoned, 5
	limited := reportFor(t, &fakeInterviewRepository{record: recruitment}, scoringCandidateID, candidateRole)
	if limited.EndReason == nil || *limited.EndReason != "abandoned" || limited.QuestionCount != 5 || limited.Questions != nil {
		t.Fatalf("limited view = %+v", limited)
	}
}

func TestReportRecruiterSeesTheInjectionFlag(t *testing.T) {
	record := evaluatedRecord(completedRecruitmentRecord(), 80, "Solid answers.")
	record.InjectionFlagged = true
	report := reportFor(t, evaluatedRepository(record), scoringRecruiterID, recruiterRole)

	if report.InjectionFlagged == nil || !*report.InjectionFlagged {
		t.Fatalf("InjectionFlagged = %v, want true", report.InjectionFlagged)
	}
}
