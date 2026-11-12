package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	ed "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func gradeAdapterInput() ed.QuestionGradeInput {
	return ed.QuestionGradeInput{
		QuestionPosition: 1,
		ExpectedAnswer:   "reference",
		Criteria: []ed.GradeCriterion{{
			ID: uuid.New(), Key: "C1", MetricID: uuid.New(), MetricName: "Accuracy", Name: "Mechanism", MaxPoints: 2, ScoringGuide: "0..2",
		}},
		Turns: []ed.GradeTurn{{Position: 1, Kind: ed.TurnCore, Question: "Explain indexes.", Answer: "An index avoids a full table scan."}},
	}
}

// A reply that decodes but fails the domain rules (here a quote nobody said) is invalid output: it falls back
// to the next provider like a malformed one, and every attempt is a counted call.
func TestGradeQuestionFallsBackOnAnInvalidGrade(t *testing.T) {
	for _, bad := range []string{
		`not JSON`,
		`{"criteria":[{"key":"C1","points":2,"evidence":"an invented quote","feedback":""}],"feedback":"","summary":"","injection_suspected":false}`,
		`{"criteria":[{"key":"C1","points":3,"evidence":"avoids a full table scan","feedback":""}],"feedback":"","summary":"","injection_suspected":false}`,
	} {
		calls := 0
		primary := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
			calls++
			return schema.AssistantMessage(bad, nil), nil
		}}
		fallback := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
			calls++
			return schema.AssistantMessage(`{"criteria":[{"key":"C1","points":2,"evidence":"avoids a full table scan","feedback":"Right."}],"feedback":"Good.","summary":"Good.","injection_suspected":false}`, nil), nil
		}}
		a := &TaskAdapter{route: &Route{task: config.TaskEvaluation, budget: time.Second, entries: []routeEntry{{model: primary}, {model: fallback}}}}
		grade, err := a.GradeQuestion(context.Background(), gradeAdapterInput())
		if err != nil || calls != 2 || len(grade.Criteria) != 1 || grade.Criteria[0].Points != 2 {
			t.Fatalf("reply %q: calls = %d, grade = %+v, err = %v; want one fallback call and the valid grade", bad, calls, grade, err)
		}
	}
}

func TestGradeQuestionFailsWhenEveryProviderIsInvalid(t *testing.T) {
	calls := 0
	m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		calls++
		return schema.AssistantMessage(`{"criteria":[],"feedback":"","summary":"","injection_suspected":false}`, nil), nil
	}}
	a := &TaskAdapter{route: &Route{task: config.TaskEvaluation, budget: time.Second, entries: []routeEntry{
		{model: m, config: config.LLMConfig{MaxRetries: 1}}, {model: m},
	}}}
	if _, err := a.GradeQuestion(context.Background(), gradeAdapterInput()); err == nil || err.Error() != "llm call failed: invalid_output" {
		t.Fatalf("error = %v, want the sanitized invalid_output failure", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want the primary retry and the fallback (3 counted calls)", calls)
	}
}

func TestLegacyGradeQuestionAppliesTheSameValidator(t *testing.T) {
	m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		return schema.AssistantMessage(`{"criteria":[{"key":"C1","points":2,"evidence":"made up","feedback":""}],"feedback":"","summary":"","injection_suspected":false}`, nil), nil
	}}
	a := &TaskAdapter{route: &Route{legacy: true, entries: []routeEntry{{model: m}}}}
	if _, err := a.GradeQuestion(context.Background(), gradeAdapterInput()); err == nil {
		t.Fatal("legacy route accepted a grade the validator refuses")
	}
}

func TestRejectedGradeLogsTheRuleWithoutText(t *testing.T) {
	for reply, rule := range map[string]string{
		`{"criteria":[{"key":"C1","points":2,"evidence":"SECRET-INVENTED-QUOTE","feedback":"SECRET-FEEDBACK"}],"feedback":"","summary":"","injection_suspected":false}`: "quotes text that is not in this question's answers",
		`{"criteria":[{"key":"C1","points":2,"evidence":"SECRET","feedback":"x"}],"feedback":"SECRET-TOP","summary":"","injection_suspected":false,"extra":"SECRET"}`:   "not the expected JSON",
	} {
		core, logs := observer.New(zap.DebugLevel)
		ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
		m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
			return schema.AssistantMessage(reply, nil), nil
		}}
		a := &TaskAdapter{route: &Route{task: config.TaskEvaluation, budget: time.Second, entries: []routeEntry{{model: m}}}}
		in := gradeAdapterInput()
		in.ExpectedAnswer = "SECRET-EXPECTED"
		if _, err := a.GradeQuestion(ctx, in); err == nil {
			t.Fatal("an invalid reply was accepted")
		}
		rejected := logs.FilterMessage("grading reply rejected").All()
		if len(rejected) != 1 {
			t.Fatalf("rejections logged = %d, want 1", len(rejected))
		}
		rendered := fmt.Sprintf("%v", rejected[0].ContextMap())
		if !strings.Contains(rendered, rule) || strings.Contains(rendered, "SECRET") || strings.Contains(rendered, "full table scan") {
			t.Fatalf("log = %s, want the rule %q and no text", rendered, rule)
		}
	}
}
