package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGradingIssuesOf(t *testing.T) {
	id := uuid.New()
	ok := RubricState{Active: true, Permitted: true}
	text := func(s string) *string { return &s }
	words := func(n int) *string { return text(strings.TrimSpace(strings.Repeat("w ", n))) }
	for _, tc := range []struct {
		name   string
		rubric *uuid.UUID
		state  RubricState
		answer *string
		want   string
	}{
		{"complete", &id, ok, text("Key points."), ""},
		{"no rubric", nil, ok, text("Key points."), IssueMissingRubric},
		{"archived rubric", &id, RubricState{Active: false, Permitted: true}, text("a"), IssueRubricArchived},
		{"not permitted wins over archived", &id, RubricState{Active: false, Permitted: false}, text("a"), IssueRubricUnavailable},
		{"no answer", &id, ok, nil, IssueMissingExpectedAnswer},
		{"blank answer", &id, ok, text(" \n\t "), IssueMissingExpectedAnswer},
		{"answer at the word limit", &id, ok, words(MaxExpectedAnswerWords), ""},
		{"answer over the word limit", &id, ok, words(MaxExpectedAnswerWords + 1), IssueExpectedAnswerInvalid},
		{"answer at the character limit", &id, ok, text(strings.Repeat("界", MaxExpectedAnswerRunes)), ""},
		{"answer over the character limit", &id, ok, text(strings.Repeat("界", MaxExpectedAnswerRunes+1)), IssueExpectedAnswerInvalid},
		{"both missing", nil, ok, nil, IssueMissingRubric + "," + IssueMissingExpectedAnswer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := GradingIssuesOf(tc.rubric, tc.state, tc.answer)
			if strings.Join(got, ",") != tc.want {
				t.Fatalf("issues = %v, want %q", got, tc.want)
			}
			q := NewBankQuestion(uuid.New(), "Q?", tc.rubric, tc.state, tc.answer)
			if q.GradingIncomplete != (tc.want != "") || q.GradingIssues == nil {
				t.Fatalf("question = %+v", q)
			}
		})
	}
}
