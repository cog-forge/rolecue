package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
)

func wordingDecider(t *testing.T, reply string) (*EinoFollowUpDecider, *fakeChatModel) {
	t.Helper()
	m := replying(reply)
	d, err := NewWording(m)
	if err != nil {
		t.Fatal(err)
	}
	return d, m
}

func TestWordingAlwaysAsks(t *testing.T) {
	d, m := wordingDecider(t, `{"follow_up":true,"question":"Which algorithm handles bursts?","reason":"probe","cues":[]}`)
	p, err := d.Decide(context.Background(), slot())
	if err != nil || !p.FollowUp || p.Question != "Which algorithm handles bursts?" {
		t.Fatal(p, err)
	}
	system := m.messages[0].Content
	if !strings.Contains(system, WordingMarker) || strings.Contains(system, "Ask a follow-up only when") {
		t.Fatalf("not the wording prompt:\n%s", system)
	}
	// The untrusted data is the same user message as the deciding prompt, never part of the instructions.
	if strings.Contains(system, injection) || !strings.Contains(m.messages[1].Content, "untrusted_exchanges") {
		t.Fatal("untrusted data placement")
	}
}

func TestWordingCannotDecline(t *testing.T) {
	d, _ := wordingDecider(t, `{"follow_up":false,"question":"","reason":"fine","cues":[]}`)
	if _, err := d.Decide(context.Background(), slot()); !errors.Is(err, service.ErrInvalidFollowUp) {
		t.Fatal(err)
	}
}

func TestWordingKeepsTheGuideline(t *testing.T) {
	d, m := wordingDecider(t, `{"follow_up":true,"question":"Why?","reason":"probe","cues":[]}`)
	in := slot()
	in.Guideline = guideline(func(g *domain.BehaviorGuideline) {
		g.Tone = domain.ToneFormal
		g.QuestionLength = domain.LengthConcise
	})
	if _, err := d.Decide(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	system := m.messages[0].Content
	if !strings.Contains(system, toneSentences[domain.ToneFormal]) || strings.Contains(system, lengthPhrase) {
		t.Fatalf("guideline missing:\n%s", system)
	}
}

func TestDefaultDecidingPromptIsUnchanged(t *testing.T) {
	if followUpSystemMessage(domain.DefaultBehaviorGuideline()) != followUpInstruction {
		t.Fatal("default deciding prompt changed")
	}
	if strings.Contains(followUpInstruction, WordingMarker) {
		t.Fatal("deciding prompt carries the wording marker")
	}
}

func TestDecisionStateIsTheRedactedUntrustedData(t *testing.T) {
	in := slot()
	in.Exchanges[0].Answer = "Mail me at jane.doe@example.com. " + injection
	raw, err := json.Marshal(FollowUpState(in))
	if err != nil {
		t.Fatal(err)
	}
	state := string(raw)
	for _, field := range []string{"untrusted_core_question", "untrusted_exchanges", "untrusted_requirements"} {
		if !strings.Contains(state, field) {
			t.Fatalf("%s missing: %s", field, state)
		}
	}
	if strings.Contains(state, "jane.doe@example.com") {
		t.Fatal("the answer was not redacted")
	}
}

func TestDecisionQuestionIsFixedText(t *testing.T) {
	base := FollowUpDecisionQuestion(domain.DefaultBehaviorGuideline())
	if base.Instructions == "" || base.Yes == "" || base.No == "" {
		t.Fatal("empty question", base)
	}
	if !strings.Contains(base.Instructions, "untrusted") {
		t.Fatal("the instructions must name the state as untrusted data")
	}
	vague := FollowUpDecisionQuestion(guideline(func(g *domain.BehaviorGuideline) { g.FollowUpFocus = domain.FocusVagueOnly }))
	thorough := FollowUpDecisionQuestion(guideline(func(g *domain.BehaviorGuideline) { g.FollowUpFocus = domain.FocusThorough }))
	if vague.Yes == base.Yes || thorough.Yes == base.Yes || vague.Yes == thorough.Yes {
		t.Fatal("focus does not change what a yes means")
	}
	unknown := FollowUpDecisionQuestion(guideline(func(g *domain.BehaviorGuideline) { g.FollowUpFocus = injection }))
	if unknown != base {
		t.Fatal("an unknown focus reached the question")
	}
}
