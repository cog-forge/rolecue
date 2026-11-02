package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const questionSentinel = "SENTINEL-QUESTION-7315"

func observedBankContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

func renderBankLogs(logs *observer.ObservedLogs) string {
	var b strings.Builder
	for _, entry := range logs.All() {
		fmt.Fprintf(&b, "%s %v\n", entry.Message, entry.ContextMap())
	}
	return b.String()
}

func TestGenerateLogsRejectedProposalWithRuleAndCounts(t *testing.T) {
	ctx, logs := observedBankContext()
	repo := readyRepo()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(1), nil }}

	_, err := newService(t, repo, gen, 0, 3).Generate(ctx, uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)

	rejected := logs.FilterMessage("question generation attempt rejected").All()
	if len(rejected) != 1 {
		t.Fatalf("rejected entries = %d, logs:\n%s", len(rejected), renderBankLogs(logs))
	}
	fields := rejected[0].ContextMap()
	if fields["attempt"] != int64(1) || fields["will_retry"] != false || fields["count"] != int64(1) {
		t.Fatalf("rejection fields = %v", fields)
	}
	if cause := fmt.Sprint(fields["cause_chain"]); !strings.Contains(cause, "question generation returned too few distinct questions") ||
		!strings.Contains(cause, "1 distinct questions, 3 required") {
		t.Fatalf("cause_chain does not name the rule and counts: %s", cause)
	}

	failed := logs.FilterMessage("step failed").All()
	if len(failed) != 1 || failed[0].ContextMap()["op"] != "questionbank.generate" {
		t.Fatalf("step failure entries = %d, logs:\n%s", len(failed), renderBankLogs(logs))
	}
}

func TestGenerateLogsStepOutcomeWithCount(t *testing.T) {
	ctx, logs := observedBankContext()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return questionsOf(3), nil }}

	if _, err := newService(t, readyRepo(), gen, 0, 3).Generate(ctx, uuid.New(), "recruiter", uuid.New()); err != nil {
		t.Fatal(err)
	}
	ok := logs.FilterMessage("step ok").All()
	if len(ok) != 1 {
		t.Fatalf("step ok entries = %d", len(ok))
	}
	fields := ok[0].ContextMap()
	if fields["count"] != int64(3) || fields["requested"] != int64(3) || fields["op"] != "questionbank.generate" {
		t.Fatalf("step fields = %v", fields)
	}
}

func TestGenerateLogsNeverContainQuestionText(t *testing.T) {
	ctx, logs := observedBankContext()
	gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) {
		return domain.GenerationCandidate{Questions: []string{questionSentinel + " how do you design it?", ""}}, nil
	}}

	_, err := newService(t, readyRepo(), gen, 0, 2).Generate(ctx, uuid.New(), "recruiter", uuid.New())
	assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)

	if rendered := renderBankLogs(logs); strings.Contains(rendered, questionSentinel) {
		t.Fatalf("question text reached the logs:\n%s", rendered)
	}
}
