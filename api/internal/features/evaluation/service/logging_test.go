package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const (
	sentinelSummary  = "SENTINEL-SUMMARY-7f3a"
	sentinelFeedback = "SENTINEL-FEEDBACK-7f3a"
)

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

func loggedText(logs *observer.ObservedLogs) string {
	var out strings.Builder
	for _, entry := range logs.All() {
		fmt.Fprintf(&out, "%s %v\n", entry.Message, entry.ContextMap())
	}
	return out.String()
}

func TestEvaluateLogsGradingStepAndCompletionWithoutText(t *testing.T) {
	ctx, logs := observedContext()
	repo := workedExampleRepository()
	grader := pointsGrader(workedExamplePoints())
	inner := grader.respond
	grader.respond = func(in domain.QuestionGradeInput) (domain.QuestionGradeCandidate, error) {
		out, err := inner(in)
		out.Summary, out.Feedback = sentinelSummary, sentinelFeedback
		return out, err
	}
	if err := newScoringForTest(t, repo, grader).Evaluate(ctx, scoringInterviewID); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	step := logs.FilterMessage("step ok").All()
	if len(step) != 1 || step[0].ContextMap()["op"] != "evaluation.grade" || step[0].ContextMap()["graded_count"] != int64(2) {
		t.Fatalf("grading step = %v", step)
	}
	fields := logs.FilterMessage("interview evaluation completed").All()
	if len(fields) != 1 || fields[0].ContextMap()["op"] != "evaluation.complete" || fields[0].ContextMap()["score"] != "62.00" ||
		fields[0].ContextMap()["outcome"] != "complete" {
		t.Fatalf("completion = %v", fields)
	}
	text := loggedText(logs)
	for _, forbidden := range []string{sentinelSummary, sentinelFeedback, "EXPECTED-SECRET-REFERENCE", "B-tree index", "How would you index a table?", "0..5"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%q reached the log", forbidden)
		}
	}
}

func TestEvaluateGraderFailureLogsStepAndNoCompletion(t *testing.T) {
	ctx, logs := observedContext()
	repo := workedExampleRepository()
	grader := &fakeGrader{respond: failing(errors.New("model timeout"))}
	if err := newScoringForTest(t, repo, grader).Evaluate(ctx, scoringInterviewID); err == nil {
		t.Fatal("Evaluate must fail when the grader fails")
	}
	step := logs.FilterMessage("step failed").All()
	if len(step) != 1 || step[0].Level != zapcore.WarnLevel || step[0].ContextMap()["op"] != "evaluation.grade" {
		t.Fatalf("failed step = %v", step)
	}
	if n := logs.FilterMessage("interview evaluation completed").Len(); n != 0 {
		t.Fatalf("a failed evaluation logged %d completions", n)
	}
	if strings.Contains(loggedText(logs), "EXPECTED-SECRET-REFERENCE") {
		t.Fatal("the expected answer reached the log")
	}
}

func TestReplaceAndResetWeightsAreAudited(t *testing.T) {
	ctx, logs := observedContext()
	recruiterID, postingID := uuid.New(), uuid.New()
	repo := &fakeRepository{metrics: []domain.Metric{accuracy, depth, clarity}, status: "pending"}
	svc := newService(t, repo, 0)
	if _, err := svc.ReplaceWeights(ctx, recruiterID, "recruiter", postingID, []domain.WeightInput{entry(accuracy, 40)}); err != nil {
		t.Fatalf("ReplaceWeights: %v", err)
	}
	replaced := logs.FilterMessage("evaluation weights replaced").All()
	if len(replaced) != 1 || replaced[0].ContextMap()["op"] != "evaluation.replace_weights" ||
		replaced[0].ContextMap()["posting_id"] != postingID.String() || replaced[0].ContextMap()["count"] != int64(1) {
		t.Fatalf("replace log = %v", replaced)
	}
	if _, err := svc.ResetWeights(ctx, recruiterID, "recruiter", postingID); err != nil {
		t.Fatalf("ResetWeights: %v", err)
	}
	reset := logs.FilterMessage("evaluation weights reset").All()
	if len(reset) != 1 || reset[0].ContextMap()["op"] != "evaluation.reset_weights" || reset[0].ContextMap()["user_id"] != recruiterID.String() {
		t.Fatalf("reset log = %v", reset)
	}
}
