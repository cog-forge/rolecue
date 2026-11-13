package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cog-forge/rolecue/api/internal/config"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/google/uuid"
)

type topicReply struct {
	Question       string  `json:"question"`
	TopicID        string  `json:"topic_id"`
	RubricID       *string `json:"rubric_id"`
	ExpectedAnswer string  `json:"expected_answer"`
}

// topicBankAdapter answers each batch from its untrusted_roadmap_topics quotas, or, when ignoreQuota is set,
// writes every question for the first topic of the full list.
func topicBankAdapter(t *testing.T, calls *int, ignoreQuota bool, all []qd.RoadmapTopic) *TaskAdapter {
	t.Helper()
	m := replyModel{generate: func(msgs []*schema.Message) (*schema.Message, error) {
		var data struct {
			Topics []struct {
				ID        string `json:"id"`
				Questions int    `json:"questions"`
			} `json:"untrusted_roadmap_topics"`
		}
		if err := json.Unmarshal([]byte(msgs[1].Content), &data); err != nil {
			t.Fatal(err)
		}
		items := []topicReply{}
		for _, topic := range data.Topics {
			for i := 0; i < topic.Questions; i++ {
				id := topic.ID
				if ignoreQuota {
					id = all[0].ID.String()
				}
				items = append(items, topicReply{Question: fmt.Sprintf("Question %d about %s?", *calls*100+len(items), topic.ID), TopicID: id, ExpectedAnswer: "Key points."})
			}
		}
		*calls++
		body, _ := json.Marshal(struct {
			Questions []topicReply `json:"questions"`
		}{items})
		return schema.AssistantMessage(string(body), nil), nil
	}}
	return &TaskAdapter{route: &Route{task: config.TaskBank, budget: time.Second, entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
}

func rankedTopics(n int) []qd.RoadmapTopic {
	out := make([]qd.RoadmapTopic, n)
	for i := range out {
		out[i] = qd.RoadmapTopic{ID: uuid.New(), Title: fmt.Sprintf("Topic %d", i)}
	}
	return out
}

func TestRoadmapBankBatchesOweTheTopicsStillShortOfTheirQuota(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(3), 30)
	in := qd.GenerationInput{Grading: true, Topics: topics}
	calls := 0
	bank, err := topicBankAdapter(t, &calls, false, topics).Generate(context.Background(), in, 30)
	if err != nil || calls != 3 || len(bank.Questions) != 30 {
		t.Fatalf("calls=%d questions=%d err=%v, want 3 batches of 10", calls, len(bank.Questions), err)
	}
	counts := qd.CountByTopic(bank.Questions)
	for _, topic := range topics {
		if counts[topic.ID] != 10 {
			t.Fatalf("topic %s has %d questions, want its quota of 10", topic.Title, counts[topic.ID])
		}
	}
}

func TestRoadmapBankRefusesAnUncoveredTopicOnTheCompleteBank(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(3), 30)
	in := qd.GenerationInput{Grading: true, Topics: topics}
	calls := 0
	if _, err := topicBankAdapter(t, &calls, true, topics).Generate(context.Background(), in, 30); err == nil {
		t.Fatal("a bank that ignores two topics was accepted")
	}
}

// A batch that leaves a topic short is retried by the route (here: falls back to the second provider) and the
// earlier batches are not asked again.
func TestRoadmapBankRetriesOnlyTheBatchThatBrokeItsQuota(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(2), 20) // two groq batches of 10: five questions per topic in each
	in := qd.GenerationInput{Grading: true, Topics: topics}
	primaryCalls, fallbackCalls := 0, 0
	primary := replyModel{generate: func(msgs []*schema.Message) (*schema.Message, error) {
		primaryCalls++
		// The second batch ignores its quota and writes only the first topic.
		return topicReplyFor(t, msgs, "primary", primaryCalls, primaryCalls == 2, topics), nil
	}}
	fallback := replyModel{generate: func(msgs []*schema.Message) (*schema.Message, error) {
		fallbackCalls++
		return topicReplyFor(t, msgs, "fallback", fallbackCalls, false, topics), nil
	}}
	a := &TaskAdapter{route: &Route{task: config.TaskBank, budget: 2 * time.Second, entries: []routeEntry{
		{model: primary, config: config.LLMConfig{Provider: "groq"}}, {model: fallback, config: config.LLMConfig{Provider: "gemini"}}}}}
	bank, err := a.Generate(context.Background(), in, 20)
	if err != nil || len(bank.Questions) != 20 {
		t.Fatalf("questions=%d err=%v", len(bank.Questions), err)
	}
	counts := qd.CountByTopic(bank.Questions)
	for _, topic := range topics {
		if counts[topic.ID] != 10 {
			t.Fatalf("topic %s has %d questions", topic.Title, counts[topic.ID])
		}
	}
	// Batch 1 came from the primary and was not asked again; only the broken batch 2 was redone by the fallback.
	if primaryCalls != 2 || fallbackCalls != 1 {
		t.Fatalf("primary calls = %d, fallback calls = %d, want 2 and 1", primaryCalls, fallbackCalls)
	}
}

// topicReplyFor answers one batch from the quotas in its prompt; ignoreQuota writes every question for the
// first topic of the list instead.
func topicReplyFor(t *testing.T, msgs []*schema.Message, who string, call int, ignoreQuota bool, all []qd.RoadmapTopic) *schema.Message {
	t.Helper()
	var data struct {
		Topics []struct {
			ID        string `json:"id"`
			Questions int    `json:"questions"`
		} `json:"untrusted_roadmap_topics"`
	}
	if err := json.Unmarshal([]byte(msgs[1].Content), &data); err != nil {
		t.Fatal(err)
	}
	items := []topicReply{}
	for _, topic := range data.Topics {
		for i := 0; i < topic.Questions; i++ {
			id := topic.ID
			if ignoreQuota {
				id = all[0].ID.String()
			}
			items = append(items, topicReply{Question: fmt.Sprintf("Question %s-%d-%d about %s?", who, call, len(items), topic.ID), TopicID: id, ExpectedAnswer: "Key points."})
		}
	}
	body, _ := json.Marshal(struct {
		Questions []topicReply `json:"questions"`
	}{items})
	return schema.AssistantMessage(string(body), nil)
}
