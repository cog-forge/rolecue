package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
)

func TestNearDuplicatePairs(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		// Finding F4 (KAN-357): the Go topic held these twice in different words.
		{"buffered channel vs worker pool", "When would you use a buffered channel instead of a worker pool in Go?",
			"How do you choose between a buffered channel and a worker pool in Go?", true},
		{"context cancellation", "How do you use context.Context to cancel goroutines in Go?",
			"Explain how context cancellation propagates to goroutines in Go.", true},
		{"punctuation and case only", "What is a Goroutine leak, and how do you prevent it?",
			"what is a goroutine leak and how do you prevent it", true},
		{"plural and article", "How does the scheduler run goroutines?", "How do schedulers run a goroutine?", true},
		// Short questions that share a technical term are different questions.
		{"short shared term", "What is context in Go?", "How do you cancel with context in Go?", false},
		{"same term, other angle", "How do you prevent goroutine leaks in Go?",
			"How do you detect goroutine leaks in production Go services?", false},
		{"shared channel only", "How does a buffered channel differ from an unbuffered channel?",
			"How would you implement a worker pool with buffered channels?", false},
		{"two terms only", "What is a mutex?", "When is a mutex better than a channel?", false},
		{"different topic", "How do row-level locks work in PostgreSQL?", "How do you write a multi-stage Dockerfile for a Go service?", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nearDuplicate(questionTokens(tc.a), questionTokens(tc.b), domain.DefaultDuplicateThreshold); got != tc.want {
				t.Fatalf("near duplicate = %v, want %v (similarity %.2f)", got, tc.want, similarity(questionTokens(tc.a), questionTokens(tc.b)))
			}
		})
	}
}

func TestNearDuplicateIsSymmetricAndThresholded(t *testing.T) {
	a := questionTokens("When would you use a buffered channel instead of a worker pool in Go?")
	b := questionTokens("How do you choose between a buffered channel and a worker pool in Go?")
	if nearDuplicate(a, b, 0.7) != nearDuplicate(b, a, 0.7) {
		t.Fatal("not symmetric")
	}
	// A threshold of 1 keeps only identical token sets out; above 1 turns the check off.
	if nearDuplicate(a, b, 1.01) {
		t.Fatal("threshold above 1 must turn the check off")
	}
}

func TestQuestionTokensDropFunctionWords(t *testing.T) {
	got := questionTokens("How would you explain the difference between a Mutex and a Channel?")
	for _, want := range []string{"mutex", "channe"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("%q missing from %v", want, got)
		}
	}
	for _, dropped := range []string{"how", "would", "you", "explai", "the", "differ", "a", "and"} {
		if _, ok := got[dropped]; ok {
			t.Fatalf("%q kept in %v", dropped, got)
		}
	}
}

func TestValidateDropsLaterNearDuplicates(t *testing.T) {
	in := domain.GenerationInput{Existing: []string{"How do you use context.Context to cancel goroutines in Go?"}}
	c := domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{
		{Content: "Explain how context cancellation propagates to goroutines in Go."}, // near Existing
		{Content: "When would you use a buffered channel instead of a worker pool in Go?"},
		{Content: "How do you choose between a buffered channel and a worker pool in Go?"}, // near the one above
		{Content: "What is context in Go?"},
		{Content: "How do row-level locks work in PostgreSQL?"},
	}}
	out, err := validateCandidate(in, c, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"When would you use a buffered channel instead of a worker pool in Go?", "What is context in Go?", "How do row-level locks work in PostgreSQL?"}
	for i, q := range out {
		if q.Content != want[i] {
			t.Fatalf("kept %v", out)
		}
	}
	// Short after dropping is invalid output, like an exact duplicate today: the route retries the batch.
	if _, err := validateCandidate(in, c, 4); err == nil {
		t.Fatal("too few distinct questions accepted")
	}
	// The final pass over an accepted bank drops nothing.
	again := domain.GenerationCandidate{}
	for _, q := range out {
		again.Questions = append(again.Questions, domain.GeneratedQuestion{Content: q.Content})
	}
	if out2, err := validateCandidate(domain.GenerationInput{}, again, 3); err != nil || len(out2) != 3 {
		t.Fatal("validation is not idempotent", err)
	}
}

func TestDuplicateThresholdComesFromTheInput(t *testing.T) {
	c := domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{
		{Content: "When would you use a buffered channel instead of a worker pool in Go?"},
		{Content: "How do you choose between a buffered channel and a worker pool in Go?"},
	}}
	// Above 1 only exact duplicates are dropped, as before KAN-384.
	if out, err := validateCandidate(domain.GenerationInput{DuplicateThreshold: 1.01}, c, 2); err != nil || len(out) != 2 {
		t.Fatal("threshold ignored", err)
	}
	if _, err := validateCandidate(domain.GenerationInput{}, c, 2); err == nil {
		t.Fatal("default threshold must drop the near duplicate")
	}
}

func TestServiceStampsItsDuplicateThreshold(t *testing.T) {
	pair := domain.GenerationCandidate{Questions: []domain.GeneratedQuestion{
		{Content: "How do you use context.Context to cancel goroutines in Go?"},
		{Content: "Explain how context cancellation propagates to goroutines in Go."}, // similarity 0.8
	}}
	for _, tc := range []struct {
		threshold float64
		accepted  bool
	}{{0, false}, {0.9, true}} {
		gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) { return pair, nil }}
		svc, err := New(&fakeRepository{}, gen, 0, 2)
		if err != nil {
			t.Fatal(err)
		}
		svc.SetDuplicateThreshold(tc.threshold)
		_, err = svc.generateWithRetry(context.Background(), domain.GenerationInput{}, false)
		if (err == nil) != tc.accepted || gen.gotIn.DuplicateThreshold != tc.threshold {
			t.Fatalf("threshold %v: err=%v, generator saw %v", tc.threshold, err, gen.gotIn.DuplicateThreshold)
		}
	}
}
