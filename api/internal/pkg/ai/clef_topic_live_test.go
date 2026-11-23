//go:build ai_live

package ai

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/google/uuid"
)

// TestLiveClefTopicCheck makes billable Workers AI calls (KAN-384). The cases are finding F4 of KAN-357: a Go
// topic bank held PostgreSQL locking and Docker questions. It needs CLOUDFLARE_ACCOUNT_ID and
// CLOUDFLARE_AI_API_TOKEN, and logs probabilities and counts only. It compares the packed request (one per batch)
// with one request per question, for both models.
func TestLiveClefTopicCheck(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal("config invalid")
	}
	if !cfg.AI.HasCloudflareCredentials() {
		t.Skip("Cloudflare credentials are missing")
	}
	goTopic := qd.RoadmapTopic{ID: uuid.New(), Title: "Go", Description: "Writing idiomatic Go: goroutines, channels, the standard library, error handling and testing."}
	cases := []struct {
		label, question string
	}{
		{"off", "How do row-level locks and SELECT ... FOR UPDATE work in PostgreSQL?"},
		{"off", "How does PostgreSQL detect and resolve deadlocks between transactions?"},
		{"off", "How do you write a multi-stage Dockerfile that keeps the final image small?"},
		{"off", "What is the difference between a Docker image and a container?"},
		{"on", "When would you use a buffered channel instead of a worker pool in Go?"},
		{"on", "How do you use context.Context to cancel goroutines?"},
		{"on", "How do you handle the error returned by a deferred Close call in Go?"},
		{"on-border", "How do you configure connection pooling with database/sql in a Go service?"},
		{"on-border", "How do you build a statically linked Go binary for a scratch container image?"},
		{"ambiguous", "How do you find and fix slow PostgreSQL queries issued by a Go service?"},
	}
	questions := make([]qd.GeneratedQuestion, len(cases))
	for i, c := range cases {
		id := goTopic.ID
		questions[i] = qd.GeneratedQuestion{Content: c.question, TopicID: &id}
	}
	topics := []qd.RoadmapTopic{goTopic}
	for _, model := range []string{"clef-flash", "clef"} {
		check := newClefTopicCheck(newClefClient(cloudflareAPIBase, cfg.AI.CloudflareAccountID, cfg.AI.CloudflareAPIToken, model, 20*time.Second), 0.5, 20*time.Second)
		for _, perRequest := range []int{maxClefQuestions, 1} {
			start := time.Now()
			scores, usage, err := check.scores(context.Background(), topics, questions, perRequest)
			if err != nil {
				t.Fatalf("model=%s per_request=%d failed (%s)", model, perRequest, llmFailureReason(err))
			}
			t.Logf("model=%s per_request=%d elapsed=%s input_tokens=%d", model, perRequest, time.Since(start).Round(time.Millisecond), usage.InputTokens)
			for i, c := range cases {
				t.Logf("  case=%d label=%s p_on_topic=%.3f", i+1, c.label, scores[i])
			}
		}
	}
}
