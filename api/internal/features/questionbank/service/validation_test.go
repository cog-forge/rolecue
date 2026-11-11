package service

import (
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestValidateCandidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   domain.GenerationCandidate
		size int
		ok   bool
		want []string
	}{
		{"exact count", domain.CandidateOf("A?", "B?"), 2, true, []string{"A?", "B?"}},
		{"trims", domain.CandidateOf("  A?  ", "B?"), 2, true, []string{"A?", "B?"}},
		{"nil list", domain.GenerationCandidate{}, 2, false, nil},
		{"too few", domain.CandidateOf("A?"), 2, false, nil},
		{"extra distinct questions keep the first size", domain.CandidateOf("A?", "B?", "C?"), 2, true, []string{"A?", "B?"}},
		{"case-insensitive duplicate drops below size", domain.CandidateOf("What is Go?", "what is go?"), 2, false, nil},
		{"duplicate removed but count still met", domain.CandidateOf("A?", "a?", "B?"), 2, true, []string{"A?", "B?"}},
		{"blank question", domain.CandidateOf("A?", "   "), 2, false, nil},
		{"NUL character", domain.CandidateOf("A?", "B\x00?"), 2, false, nil},
		{"invalid UTF-8", domain.CandidateOf("A?", string([]byte{0xff, 0xfe})), 2, false, nil},
		{"too long", domain.CandidateOf("A?", strings.Repeat("界", domain.MaxQuestionRunes+1)), 2, false, nil},
		{"max length ok", domain.CandidateOf("A?", strings.Repeat("界", domain.MaxQuestionRunes)), 2, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCandidate(domain.GenerationInput{}, tc.in, tc.size)
			if !tc.ok {
				assertCode(t, err, apperror.CodeInvalidQuestionGenerationOutput)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.want != nil && strings.Join(contentsOf(got), "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func contentsOf(questions []domain.GeneratedQuestion) []string {
	out := make([]string, 0, len(questions))
	for _, q := range questions {
		out = append(out, q.Content)
	}
	return out
}
