package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	id "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type replyModel struct {
	fakeModel
	generate func([]*schema.Message) (*schema.Message, error)
}

func (m replyModel) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.generate(in)
}
func TestDecodedAndDomainInvalidFallback(t *testing.T) {
	for _, reply := range []string{`not JSON`, `{"follow_up":true,"question":"wrong","reason":"probe"}`} {
		calls := 0
		primary := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
			calls++
			return schema.AssistantMessage(reply, nil), nil
		}}
		fallback := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
			calls++
			return schema.AssistantMessage(`{"follow_up":true,"question":"How would you test it?","reason":"probe"}`, nil), nil
		}}
		a := &TaskAdapter{route: &Route{task: config.TaskFollowUp, budget: time.Second, entries: []routeEntry{{model: primary}, {model: fallback}}}}
		out, err := a.Decide(context.Background(), id.FollowUpContext{CoreQuestion: "How would you implement it?"})
		if err != nil || calls != 2 || out.Question != "How would you test it?" {
			t.Fatal("invalid output fallback failed", calls, err)
		}
	}
}
func TestBankBatchesAndDeduplicates(t *testing.T) {
	calls := 0
	m := replyModel{generate: func(in []*schema.Message) (*schema.Message, error) {
		var data struct {
			Existing []string `json:"untrusted_existing_questions"`
		}
		if err := json.Unmarshal([]byte(in[1].Content), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Existing) != calls*10 {
			t.Fatal("previous questions not passed as data")
		}
		questions := []string{}
		for i := 0; i < 10; i++ {
			questions = append(questions, fmt.Sprintf("Question %d?", calls*10+i))
		}
		questions = append(questions, questions[0]) // Extra duplicates must never reach storage.
		calls++
		body, _ := json.Marshal(struct {
			Questions []string `json:"questions"`
		}{questions})
		return schema.AssistantMessage(string(body), nil), nil
	}}
	a := &TaskAdapter{route: &Route{task: config.TaskBank, budget: time.Second, entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
	bank, err := a.Generate(context.Background(), qd.GenerationInput{}, 50)
	if err != nil || calls != 5 || len(bank.Questions) != 50 {
		t.Fatal(calls, len(bank.Questions), err)
	}
}
func TestLegacyAppliesTheSameDomainValidators(t *testing.T) {
	m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		return schema.AssistantMessage(`{"questions":["duplicate","duplicate"]}`, nil), nil
	}}
	a := &TaskAdapter{route: &Route{legacy: true, entries: []routeEntry{{model: m}}}}
	if _, err := a.Generate(context.Background(), qd.GenerationInput{}, 50); err == nil {
		t.Fatal("legacy route accepted a bank the validators reject")
	}
}
func TestBankGradedBatchesKeepAllowedRubricsAndRefuseOthers(t *testing.T) {
	allowed, foreign := uuid.New(), uuid.New()
	in := qd.GenerationInput{Grading: true, Rubrics: []qd.RubricOption{{ID: allowed, Name: "r"}}}
	build := func(rubric uuid.UUID, calls *int) *TaskAdapter {
		m := replyModel{generate: func(msgs []*schema.Message) (*schema.Message, error) {
			var data struct {
				Existing []string `json:"untrusted_existing_questions"`
			}
			if err := json.Unmarshal([]byte(msgs[1].Content), &data); err != nil {
				t.Fatal(err)
			}
			type item struct {
				Question       string `json:"question"`
				RubricID       string `json:"rubric_id"`
				ExpectedAnswer string `json:"expected_answer"`
			}
			items := []item{}
			for i := 0; i < 10; i++ {
				items = append(items, item{fmt.Sprintf("Question %d?", *calls*10+i), rubric.String(), "Key points."})
			}
			*calls++
			body, _ := json.Marshal(struct {
				Questions []item `json:"questions"`
			}{items})
			return schema.AssistantMessage(string(body), nil), nil
		}}
		return &TaskAdapter{route: &Route{task: config.TaskBank, budget: time.Second, entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
	}

	calls := 0
	bank, err := build(allowed, &calls).Generate(context.Background(), in, 20)
	if err != nil || calls != 2 || len(bank.Questions) != 20 {
		t.Fatal(calls, len(bank.Questions), err)
	}
	for _, q := range bank.Questions {
		if q.RubricID == nil || *q.RubricID != allowed || q.ExpectedAnswer == nil {
			t.Fatalf("grading data lost in batching: %+v", q)
		}
	}

	calls = 0
	if _, err := build(foreign, &calls).Generate(context.Background(), in, 10); err == nil {
		t.Fatal("a rubric id outside the allowed set was accepted by the routed path")
	}
}

func TestRejectedBankBatchLogsTheRuleWithoutText(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	foreign := uuid.New()
	in := qd.GenerationInput{Grading: true}
	m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		body := fmt.Sprintf(`{"questions":[{"question":"SECRET-QUESTION?","rubric_id":%q,"expected_answer":"SECRET-ANSWER"}]}`, foreign)
		return schema.AssistantMessage(body, nil), nil
	}}
	a := &TaskAdapter{route: &Route{task: config.TaskBank, budget: time.Second, entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
	if _, err := a.Generate(ctx, in, 1); err == nil {
		t.Fatal("a rubric outside the allowed set was accepted")
	}
	rejected := logs.FilterMessage("question bank batch rejected").All()
	if len(rejected) == 0 {
		t.Fatal("the rejected batch was not logged")
	}
	rendered := fmt.Sprintf("%v", rejected[0].ContextMap())
	if !strings.Contains(rendered, "not in the allowed set") {
		t.Fatalf("the failed rule is missing: %s", rendered)
	}
	for _, leaked := range []string{"SECRET-QUESTION", "SECRET-ANSWER", foreign.String()} {
		if strings.Contains(rendered, leaked) {
			t.Fatalf("%q reached the log: %s", leaked, rendered)
		}
	}
}
