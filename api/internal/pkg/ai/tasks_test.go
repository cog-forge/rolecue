package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	id "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
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
		body, _ := json.Marshal(qd.GenerationCandidate{Questions: questions})
		return schema.AssistantMessage(string(body), nil), nil
	}}
	a := &TaskAdapter{route: &Route{task: config.TaskBank, budget: time.Second, entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
	bank, err := a.Generate(context.Background(), qd.GenerationInput{}, 50)
	if err != nil || calls != 5 || len(bank.Questions) != 50 {
		t.Fatal(calls, len(bank.Questions), err)
	}
}
func TestLegacyLeavesDomainValidationAndRetriesToService(t *testing.T) {
	m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		return schema.AssistantMessage(`{"questions":["duplicate","duplicate"]}`, nil), nil
	}}
	a := &TaskAdapter{route: &Route{legacy: true, entries: []routeEntry{{model: m}}}}
	bank, err := a.Generate(context.Background(), qd.GenerationInput{}, 50)
	if err != nil || len(bank.Questions) != 2 {
		t.Fatal("legacy proposal changed", err)
	}
}
