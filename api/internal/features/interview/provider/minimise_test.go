package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
)

func TestDecideStripsIdentifiersFromTheAnswer(t *testing.T) {
	chat := replying(`{"follow_up":false,"question":"","reason":"done"}`)
	in := slot()
	in.Exchanges = []domain.Exchange{{
		Question: "How would you design a rate limiter?",
		Answer:   "Mail me at an.nguyen@example.com or +84 912 345 678, my site is https://an.example.dev/cv.",
	}}
	if _, err := decider(t, chat).Decide(context.Background(), in); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	user := chat.messages[1].Content
	for _, leaked := range []string{"an.nguyen@example.com", "912 345 678", "an.example.dev"} {
		if strings.Contains(user, leaked) {
			t.Fatalf("the prompt payload still contains %q: %s", leaked, user)
		}
	}
}
