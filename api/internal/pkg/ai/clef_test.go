package ai

import (
	"context"
	"encoding/json"
	"errors"
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
	id "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	ip "github.com/cog-forge/rolecue/api/internal/features/interview/provider"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const (
	clefAccount = "0123456789abcdef0123456789abcdef"
	clefToken   = "cf-test-secret-token"
)

// clefReply is the Workers AI REST envelope as the first live call returned it (2026-11-18).
func clefReply(p float64) string {
	return fmt.Sprintf(`{"result":{"model":"clef-flash","answers":{"ask_follow_up":{"type":"noul","noul":%v}},"usage":{"input_tokens":177,"output_tokens":0}},"success":true,"errors":[],"messages":[]}`, p)
}

func clefServer(t *testing.T, status int, body string, seen func(*http.Request, map[string]any)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		if seen != nil {
			seen(r, decoded)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

// stallingHandler never answers in time. It reads the body first so the server notices the client hanging up,
// and gives up on its own so closing the test server never waits for it.
func stallingHandler(_ http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body)
	select {
	case <-r.Context().Done():
	case <-time.After(2 * time.Second):
	}
}

func testClef(base string) *clefClient {
	return newClefClient(base, clefAccount, clefToken, "clef-flash", time.Second)
}

var clefQuestion = ClefYesNo{Instructions: "Should the interviewer ask a follow-up?", Yes: "The answer is vague.", No: "The answer is complete."}

func TestClefRequestShape(t *testing.T) {
	state := map[string]string{"untrusted_answer": secretAnswer}
	srv, _ := clefServer(t, 200, clefReply(0.73), func(r *http.Request, body map[string]any) {
		if r.Method != http.MethodPost || r.URL.Path != "/client/v4/accounts/"+clefAccount+"/ai/run/@cf/cloudflare/clef-flash" {
			t.Errorf("endpoint %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+clefToken || r.Header.Get("Content-Type") != "application/json" {
			t.Error("headers")
		}
		if body["model"] != "clef-flash" {
			t.Errorf("model %v", body["model"])
		}
		q := body["questions"].(map[string]any)[clefQuestionID].(map[string]any)
		if q["type"] != "noul" || q["instructions"] != clefQuestion.Instructions {
			t.Errorf("question %v", q)
		}
		criteria := q["criteria"].(map[string]any)
		if criteria["true"] != clefQuestion.Yes || criteria["false"] != clefQuestion.No {
			t.Errorf("criteria %v", criteria)
		}
		// The untrusted text travels only inside state, never inside the instructions.
		if body["state"].(map[string]any)["untrusted_answer"] != secretAnswer || strings.Contains(fmt.Sprint(q), secretAnswer) {
			t.Error("state placement")
		}
	})
	p, usage, err := testClef(srv.URL).YesProbability(context.Background(), state, clefQuestion)
	if err != nil || p != 0.73 || usage.InputTokens != 177 {
		t.Fatal(p, usage, err)
	}
}

func TestClefFailuresAreClassified(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		reason     string
	}{
		{"auth", `{"success":false,"errors":[{"message":"` + secretAnswer + `"}]}`, 401, "auth_rejected"},
		{"bad request", `{}`, 400, "http_4xx"},
		{"rate limited", `{}`, 429, "rate_limited"},
		{"server", `{}`, 503, "http_5xx"},
		{"not json", `<html>`, 200, "invalid_output"},
		{"not success", `{"result":null,"success":false,"errors":[{"code":1}]}`, 200, "invalid_output"},
		{"missing answer", `{"result":{"answers":{}},"success":true}`, 200, "invalid_output"},
		{"wrong type", `{"result":{"answers":{"ask_follow_up":{"type":"choice","noul":0.5}}},"success":true}`, 200, "invalid_output"},
		{"no probability", `{"result":{"answers":{"ask_follow_up":{"type":"noul"}}},"success":true}`, 200, "invalid_output"},
		{"above one", clefReply(1.2), 200, "invalid_output"},
		{"below zero", clefReply(-0.1), 200, "invalid_output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := clefServer(t, tc.status, tc.body, nil)
			_, _, err := testClef(srv.URL).YesProbability(context.Background(), "state", clefQuestion)
			if err == nil || llmFailureReason(err) != tc.reason {
				t.Fatalf("reason %q, err %v", llmFailureReason(err), err)
			}
			for _, secret := range []string{secretAnswer, clefToken} {
				if strings.Contains(err.Error(), secret) {
					t.Fatal("error leaks provider content or the token")
				}
			}
		})
	}
}

func TestClefHonoursTheContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(stallingHandler))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, _, err := testClef(srv.URL).YesProbability(ctx, "state", clefQuestion)
	if llmFailureReason(err) != "timeout" {
		t.Fatal(err)
	}
}

// wordingLLM answers like the follow-up chat model and records which prompt it was given.
type wordingLLM struct {
	calls   *atomic.Int32
	wording *atomic.Int32
	reply   func(wording bool) string
}

func (m wordingLLM) model() replyModel {
	return replyModel{generate: func(in []*schema.Message) (*schema.Message, error) {
		m.calls.Add(1)
		wording := strings.Contains(in[0].Content, ip.WordingMarker)
		if wording {
			m.wording.Add(1)
		}
		return schema.AssistantMessage(m.reply(wording), nil), nil
	}}
}

func newWordingLLM(reply func(bool) string) wordingLLM {
	return wordingLLM{calls: &atomic.Int32{}, wording: &atomic.Int32{}, reply: reply}
}

const (
	askReply     = `{"follow_up":true,"question":"Which collision strategy did you use and why?","reason":"probe the claim","cues":[]}`
	declineReply = `{"follow_up":false,"question":"","reason":"complete answer","cues":[]}`
)

func testDecider(clef *clefClient, llm wordingLLM) *ClefFollowUpDecider {
	adapter := &TaskAdapter{route: &Route{task: config.TaskFollowUp, budget: time.Second, entries: []routeEntry{{model: llm.model(), config: config.LLMConfig{Provider: "groq", Model: "test"}}}}}
	return newClefFollowUpDecider(clef, 0.5, time.Second, adapter)
}

var followUpIn = id.FollowUpContext{
	CoreQuestion: "How does a hash map handle collisions?",
	Exchanges:    []id.Exchange{{Question: "How does a hash map handle collisions?", Answer: secretAnswer}},
	Requirements: []id.Requirement{{Name: "Go", Target: "data structures", Type: "required"}},
}

func TestClefDeclinesWithoutCallingTheLLM(t *testing.T) {
	srv, _ := clefServer(t, 200, clefReply(0.2), nil)
	llm := newWordingLLM(func(bool) string { return askReply })
	p, err := testDecider(testClef(srv.URL), llm).Decide(context.Background(), followUpIn)
	if err != nil || p.FollowUp || p.Question != "" || strings.TrimSpace(p.Reason) == "" {
		t.Fatal(p, err)
	}
	if llm.calls.Load() != 0 {
		t.Fatal("the LLM was called after Clef declined")
	}
}

func TestClefYesMakesTheLLMOnlyWordTheQuestion(t *testing.T) {
	srv, _ := clefServer(t, 200, clefReply(0.5), func(_ *http.Request, body map[string]any) {
		state := fmt.Sprint(body["state"])
		if !strings.Contains(state, "untrusted_exchanges") || !strings.Contains(state, "untrusted_core_question") {
			t.Errorf("state misses the untrusted fields: %s", state)
		}
		if strings.Contains(fmt.Sprint(body["questions"]), secretAnswer) {
			t.Error("the answer reached the instructions")
		}
	})
	llm := newWordingLLM(func(bool) string { return askReply })
	p, err := testDecider(testClef(srv.URL), llm).Decide(context.Background(), followUpIn)
	if err != nil || !p.FollowUp || p.Question == "" {
		t.Fatal(p, err)
	}
	if llm.wording.Load() != 1 || llm.calls.Load() != 1 {
		t.Fatalf("calls=%d wording=%d", llm.calls.Load(), llm.wording.Load())
	}
}

func TestClefYesCannotBeVetoedByTheLLM(t *testing.T) {
	srv, _ := clefServer(t, 200, clefReply(0.9), nil)
	llm := newWordingLLM(func(bool) string { return declineReply })
	p, err := testDecider(testClef(srv.URL), llm).Decide(context.Background(), followUpIn)
	// No valid question exists, so the service skips the follow-up as invalid output; it is never a decline.
	if err == nil || p.FollowUp || llmFailureReason(err) != "invalid_output" {
		t.Fatal(p, err)
	}
}

func TestClefFailureFallsBackToTheLLMDecider(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"down", 503, `{}`},
		{"rate limited", 429, `{}`},
		{"auth", 401, `{}`},
		{"invalid output", 200, `{"result":{},"success":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, clefCalls := clefServer(t, tc.status, tc.body, nil)
			llm := newWordingLLM(func(bool) string { return askReply })
			p, err := testDecider(testClef(srv.URL), llm).Decide(context.Background(), followUpIn)
			if err != nil || !p.FollowUp || clefCalls.Load() != 1 {
				t.Fatal(p, err)
			}
			// The fallback is the full decider: decision and wording in one call, not the wording-only prompt.
			if llm.calls.Load() != 1 || llm.wording.Load() != 0 {
				t.Fatalf("calls=%d wording=%d", llm.calls.Load(), llm.wording.Load())
			}
		})
	}
}

func TestClefTimeoutLeavesTimeForTheFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(stallingHandler))
	defer srv.Close()
	llm := newWordingLLM(func(bool) string { return askReply })
	d := testDecider(testClef(srv.URL), llm)
	d.timeout = 40 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	p, err := d.Decide(ctx, followUpIn)
	if err != nil || !p.FollowUp || time.Since(start) > 500*time.Millisecond {
		t.Fatal(p, err, time.Since(start))
	}
}

func TestClefCancellationIsNotFallenBackFrom(t *testing.T) {
	srv, _ := clefServer(t, 200, clefReply(0.9), nil)
	llm := newWordingLLM(func(bool) string { return askReply })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testDecider(testClef(srv.URL), llm).Decide(ctx, followUpIn); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if llm.calls.Load() != 0 {
		t.Fatal("the LLM was called after cancellation")
	}
}

func TestClefLogsNoTextOrSecrets(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	for _, status := range []int{200, 503} {
		srv, _ := clefServer(t, status, clefReply(0.8), nil)
		llm := newWordingLLM(func(bool) string { return askReply })
		if _, err := testDecider(testClef(srv.URL), llm).Decide(ctx, followUpIn); err != nil {
			t.Fatal(err)
		}
	}
	attempts := 0
	for _, entry := range logs.All() {
		rendered := entry.Message + fmt.Sprint(entry.ContextMap())
		for _, secret := range []string{secretAnswer, clefToken, clefAccount, "hash map", clefQuestion.Instructions} {
			if strings.Contains(rendered, secret) {
				t.Fatalf("%q reached the log: %s", secret, rendered)
			}
		}
		if entry.ContextMap()["provider"] == "clef" {
			attempts++
		}
	}
	if attempts < 2 {
		t.Fatalf("clef attempts not logged: %d", attempts)
	}
}
