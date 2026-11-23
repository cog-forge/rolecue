package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/config"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// topicClefRequest is what the fake Clef server reads from a topic check request.
type topicClefRequest struct {
	State struct {
		Topics []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"untrusted_topics"`
		Questions []struct {
			ID       string `json:"id"`
			Topic    string `json:"topic"`
			Question string `json:"question"`
		} `json:"untrusted_questions"`
	} `json:"state"`
	Questions map[string]struct {
		Type         string `json:"type"`
		Instructions string `json:"instructions"`
	} `json:"questions"`
}

// topicClef answers every question with a low probability when its text holds "offtopic", else a high one.
func topicClef(t *testing.T, status int, seen func(topicClefRequest)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		var req topicClefRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("request is not JSON: %v", err)
		}
		if seen != nil {
			seen(req)
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		answers := map[string]any{}
		for _, q := range req.State.Questions {
			p := 0.95
			if strings.Contains(q.Question, "offtopic") {
				p = 0.05
			}
			answers[q.ID] = map[string]any{"type": "noul", "noul": p}
		}
		body, _ := json.Marshal(map[string]any{"result": map[string]any{"model": "clef-flash", "answers": answers, "usage": map[string]int{"input_tokens": 400, "output_tokens": 0}}, "success": true})
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

// offTopicBank answers each batch from its quotas; questions listed in offTopic (by call and position) carry
// the word the fake Clef rejects. It records every user message it was sent.
func offTopicBank(t *testing.T, offTopic func(call, item int) bool, prompts *[]string) replyModel {
	t.Helper()
	calls := 0
	return replyModel{generate: func(msgs []*schema.Message) (*schema.Message, error) {
		calls++
		*prompts = append(*prompts, msgs[1].Content)
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
				text := fmt.Sprintf("Question %d on %s?", calls*100+len(items), fixtureTerm(calls*100+len(items)))
				if offTopic(calls, len(items)) {
					text = fmt.Sprintf("Question %d offtopic %s?", calls*100+len(items), fixtureTerm(5000+calls*100+len(items)))
				}
				items = append(items, topicReply{Question: text, TopicID: topic.ID, ExpectedAnswer: "Key points."})
			}
		}
		body, _ := json.Marshal(struct {
			Questions []topicReply `json:"questions"`
		}{items})
		return schema.AssistantMessage(string(body), nil), nil
	}}
}

func topicAdapter(m replyModel, check *clefTopicCheck, legacy bool) *TaskAdapter {
	return &TaskAdapter{topicCheck: check, route: &Route{legacy: legacy, task: config.TaskBank, budget: 20 * time.Second,
		entries: []routeEntry{{model: m, config: config.LLMConfig{Provider: "groq"}}}}}
}

func topicCheckFor(base string) *clefTopicCheck {
	return newClefTopicCheck(newClefClient(base, clefAccount, clefToken, "clef-flash", time.Second), 0.5, time.Second)
}

func countBy(bank qd.GenerationCandidate, topics []qd.RoadmapTopic, t *testing.T, want int) {
	t.Helper()
	counts := qd.CountByTopic(bank.Questions)
	for _, topic := range topics {
		if counts[topic.ID] != want {
			t.Fatalf("topic %s has %d questions, want %d", topic.Title, counts[topic.ID], want)
		}
	}
	for _, q := range bank.Questions {
		if strings.Contains(q.Content, "offtopic") {
			t.Fatal("an off-topic question was kept")
		}
	}
}

func TestTopicCheckDropsOffTopicAndRefills(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(2), 20)
	var prompts []string
	// Batch 1 has two off-topic questions; the refill batch writes clean ones.
	m := offTopicBank(t, func(call, item int) bool { return call == 1 && (item == 0 || item == 7) }, &prompts)
	srv, clefCalls := topicClef(t, 200, func(req topicClefRequest) {
		if len(req.Questions) != len(req.State.Questions) || len(req.Questions) > 10 {
			t.Errorf("one noul per question of the batch: %d questions, %d in state", len(req.Questions), len(req.State.Questions))
		}
		for id, q := range req.Questions {
			// The instructions are fixed text that names an id; question text stays in the state.
			if q.Type != "noul" || !strings.Contains(q.Instructions, id) || strings.Contains(q.Instructions, "Question") {
				t.Errorf("question %s: %+v", id, q)
			}
		}
		if len(req.State.Topics) == 0 || req.State.Topics[0].Title == "" {
			t.Error("topics missing from the state")
		}
	})
	bank, err := topicAdapter(m, topicCheckFor(srv.URL), false).Generate(context.Background(), qd.GenerationInput{Grading: true, Topics: topics}, 20)
	if err != nil || len(bank.Questions) != 20 {
		t.Fatalf("questions=%d err=%v", len(bank.Questions), err)
	}
	countBy(bank, topics, t, 10)
	// Two batches of ten and one refill batch of two; one Clef request per batch.
	if len(prompts) != 3 || clefCalls.Load() != 3 {
		t.Fatalf("model calls=%d clef calls=%d", len(prompts), clefCalls.Load())
	}
	// The refill is told about the rejected questions, so it does not write them again.
	if !strings.Contains(prompts[2], "offtopic") {
		t.Fatal("the rejected questions were not excluded from the refill")
	}
}

func TestTopicCheckFailsOpen(t *testing.T) {
	for _, status := range []int{503, 429, 401} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
			topics := qd.BalancedQuotas(rankedTopics(2), 10)
			var prompts []string
			m := offTopicBank(t, func(call, item int) bool { return item == 0 }, &prompts)
			srv, _ := topicClef(t, status, nil)
			bank, err := topicAdapter(m, topicCheckFor(srv.URL), false).Generate(ctx, qd.GenerationInput{Grading: true, Topics: topics}, 10)
			// Accepted exactly as today: the off-topic question stays and nothing is regenerated.
			if err != nil || len(bank.Questions) != 10 || len(prompts) != 1 {
				t.Fatalf("questions=%d calls=%d err=%v", len(bank.Questions), len(prompts), err)
			}
			skipped := logs.FilterMessage("question bank topic check skipped").All()
			if len(skipped) != 1 {
				t.Fatalf("fail-open not logged: %d", len(skipped))
			}
			for _, e := range logs.All() {
				rendered := e.Message + fmt.Sprint(e.ContextMap())
				if strings.Contains(rendered, "Question") || strings.Contains(rendered, clefToken) || strings.Contains(rendered, "Topic 0") {
					t.Fatalf("text reached the log: %s", rendered)
				}
			}
		})
	}
}

func TestTopicCheckRefillsAreBounded(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(1), 5)
	var prompts []string
	// Every batch writes one off-topic question: after maxTopicRefills drops the rest is accepted unchecked.
	m := offTopicBank(t, func(call, item int) bool { return item == 0 }, &prompts)
	srv, clefCalls := topicClef(t, 200, nil)
	bank, err := topicAdapter(m, topicCheckFor(srv.URL), false).Generate(context.Background(), qd.GenerationInput{Grading: true, Topics: topics}, 5)
	if err != nil || len(bank.Questions) != 5 {
		t.Fatalf("questions=%d err=%v", len(bank.Questions), err)
	}
	if len(prompts) != 1+maxTopicRefills || int(clefCalls.Load()) != maxTopicRefills {
		t.Fatalf("model calls=%d clef calls=%d, want %d and %d", len(prompts), clefCalls.Load(), 1+maxTopicRefills, maxTopicRefills)
	}
}

func TestTopicCheckOnALegacyRoute(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(2), 10)
	var prompts []string
	m := offTopicBank(t, func(call, item int) bool { return call == 1 && item == 3 }, &prompts)
	srv, _ := topicClef(t, 200, nil)
	bank, err := topicAdapter(m, topicCheckFor(srv.URL), true).Generate(context.Background(), qd.GenerationInput{Grading: true, Topics: topics}, 10)
	if err != nil || len(bank.Questions) != 10 || len(prompts) != 2 {
		t.Fatalf("questions=%d calls=%d err=%v", len(bank.Questions), len(prompts), err)
	}
	countBy(bank, topics, t, 5)
}

func TestTopicCheckSkipsBanksWithoutTopics(t *testing.T) {
	srv, clefCalls := topicClef(t, 200, nil)
	m := replyModel{generate: func([]*schema.Message) (*schema.Message, error) {
		return schema.AssistantMessage(`{"questions":["How do you profile a Go service on ordinary load?","Which offtopic thing breaks container startup?"]}`, nil), nil
	}}
	bank, err := topicAdapter(m, topicCheckFor(srv.URL), false).Generate(context.Background(), qd.GenerationInput{}, 2)
	if err != nil || len(bank.Questions) != 2 || clefCalls.Load() != 0 {
		t.Fatalf("questions=%d clef=%d err=%v", len(bank.Questions), clefCalls.Load(), err)
	}
}

func TestTopicCheckIsSkippedWhenTimeIsShort(t *testing.T) {
	topics := qd.BalancedQuotas(rankedTopics(1), 5)
	var prompts []string
	m := offTopicBank(t, func(call, item int) bool { return item == 0 }, &prompts)
	srv, clefCalls := topicClef(t, 200, nil)
	a := topicAdapter(m, topicCheckFor(srv.URL), false)
	a.route.budget = topicCheckMinRemaining / 2
	bank, err := a.Generate(context.Background(), qd.GenerationInput{Grading: true, Topics: topics}, 5)
	if err != nil || len(bank.Questions) != 5 || clefCalls.Load() != 0 {
		t.Fatalf("questions=%d clef=%d err=%v", len(bank.Questions), clefCalls.Load(), err)
	}
}

func TestTopicCheckPacksAtMost64QuestionsPerRequest(t *testing.T) {
	srv, clefCalls := topicClef(t, 200, func(req topicClefRequest) {
		if len(req.Questions) > maxClefQuestions {
			t.Errorf("%d questions in one request", len(req.Questions))
		}
	})
	topics := rankedTopics(1)
	var questions []qd.GeneratedQuestion
	for i := 0; i < 70; i++ {
		id := topics[0].ID
		questions = append(questions, qd.GeneratedQuestion{Content: fmt.Sprintf("Question %d?", i), TopicID: &id})
	}
	keep, _, err := topicCheckFor(srv.URL).check(context.Background(), topics, questions)
	if err != nil || len(keep) != 70 || clefCalls.Load() != 2 {
		t.Fatalf("keep=%d clef=%d err=%v", len(keep), clefCalls.Load(), err)
	}
}

func TestTopicCheckIsOnByDefaultAndNeedsNoCredentials(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	cfg := &config.Config{}
	if newTopicCheckFromConfig(ctx, cfg) != nil {
		t.Fatal("the topic check ran without credentials")
	}
	if len(logs.FilterMessage("question bank topic check is off: Cloudflare credentials are missing").All()) != 1 {
		t.Fatal("missing credentials were not logged")
	}
	cfg.AI.CloudflareAccountID, cfg.AI.CloudflareAPIToken = clefAccount, clefToken
	check := newTopicCheckFromConfig(ctx, cfg)
	if check == nil || check.clef.model != "clef" || check.threshold != 0.5 {
		t.Fatal("the default topic check is not clef at 0.5")
	}
	for _, e := range logs.All() {
		if strings.Contains(e.Message+fmt.Sprint(e.ContextMap()), clefToken) {
			t.Fatal("the token reached the log")
		}
	}
	cfg.AI.BankCheck.Topic.Provider = config.TopicCheckOff
	if newTopicCheckFromConfig(ctx, cfg) != nil {
		t.Fatal("off did not turn the topic check off")
	}
}

// The bank adapter owns the topic check; the adapter of Roadmap topic generation must not build it again, or
// the "check is off" warning is logged twice at startup.
func TestTopicCheckWarningIsLoggedOnceWhenBothAdaptersAreBuilt(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	cfg := &config.Config{}
	cfg.LLM.Provider, cfg.LLM.Timeout = "groq", time.Second
	cfg.AI.GroqAPIKey = "test-key"
	const warning = "question bank topic check is off: Cloudflare credentials are missing"

	bank, err := NewTaskAdapter(ctx, cfg, config.TaskBank)
	if err != nil {
		t.Fatal(err)
	}
	topics, err := NewTopicsAdapter(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(logs.FilterMessage(warning).All()); n != 1 {
		t.Fatalf("the warning was logged %d times, want once", n)
	}
	// Fail-open: without credentials neither adapter has a check, and a batch passes through untouched.
	if bank.topicCheck != nil || topics.topicCheck != nil {
		t.Fatal("a topic check was built without credentials")
	}
	questions := []qd.GeneratedQuestion{{Content: "Q1?"}}
	if kept, dropped := bank.checkTopics(ctx, []qd.RoadmapTopic{{Title: "SQL"}}, questions, 0); len(kept) != 1 || len(dropped) != 0 {
		t.Fatalf("kept %d dropped %d", len(kept), len(dropped))
	}
	// With credentials only the bank adapter has the check.
	cfg.AI.CloudflareAccountID, cfg.AI.CloudflareAPIToken = clefAccount, clefToken
	bank, _ = NewTaskAdapter(ctx, cfg, config.TaskBank)
	topics, _ = NewTopicsAdapter(ctx, cfg)
	if bank.topicCheck == nil || topics.topicCheck != nil {
		t.Fatalf("bank check %v, topics check %v", bank.topicCheck != nil, topics.topicCheck != nil)
	}
}
