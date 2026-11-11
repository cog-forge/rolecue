package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// scriptedDecider returns one proposal or error, or blocks until its context ends (or forever when
// ignoreContext is set, like a provider that does not honour ctx).
type scriptedDecider struct {
	proposal      domain.FollowUpProposal
	err           error
	block         bool
	ignoreContext bool
	panicValue    any
	calls         int
	seen          domain.FollowUpContext
}

func (d *scriptedDecider) Decide(ctx context.Context, in domain.FollowUpContext) (domain.FollowUpProposal, error) {
	d.calls++
	d.seen = in
	switch {
	case d.panicValue != nil:
		panic(d.panicValue)
	case d.block && d.ignoreContext:
		select {}
	case d.block:
		<-ctx.Done()
		return domain.FollowUpProposal{}, ctx.Err()
	}
	return d.proposal, d.err
}

func pendingSlot() domain.FollowUpContext {
	return domain.FollowUpContext{
		Position: 3, QuestionNumber: 2, Ordinal: 1, CoreQuestion: "How would you design a rate limiter?",
		Exchanges:    []domain.Exchange{{Question: "How would you design a rate limiter?", Answer: sentinelAnswer}},
		Requirements: []domain.Requirement{{Name: "Go", Target: "Proficient", Type: "required"}},
	}
}

func followUpService(t *testing.T, decider FollowUpDecider, limit int32) (*Service, *fakeRepository) {
	t.Helper()
	pending := pendingSlot()
	repo := &fakeRepository{
		followUpPending: &pending,
		deliverDelivery: domain.Delivery{Type: domain.TypePractice, QuestionCount: 3, AnsweredCount: 2, ProviderVoiceID: "en-US-AvaNeural"},
	}
	svc := newTestService(t, repo, Settings{FollowUpLimit: limit, FollowUpTimeout: 200 * time.Millisecond})
	if decider != nil {
		svc.SetFollowUpDecider(decider)
	}
	return svc, repo
}

func TestFollowUpIsOffWithLimitZeroOrWithoutADecider(t *testing.T) {
	for name, build := range map[string]func(*fakeRepository) *Service{
		"limit 0": func(repo *fakeRepository) *Service {
			svc := newTestService(t, repo, Settings{FollowUpLimit: 0})
			svc.SetFollowUpDecider(&scriptedDecider{})
			return svc
		},
		"no decider": func(repo *fakeRepository) *Service {
			return newTestService(t, repo, Settings{FollowUpLimit: 1})
		},
	} {
		t.Run(name, func(t *testing.T) {
			pending := pendingSlot()
			repo := &fakeRepository{followUpPending: &pending}
			delivery, err := build(repo).FollowUp(context.Background(), candidateID, "candidate", sessionID)
			if err != nil || delivery.Question != nil || repo.followUpCalls != 0 {
				t.Fatalf("FollowUp = %+v, %v after %d reads", delivery, err, repo.followUpCalls)
			}
		})
	}
}

func TestFollowUpAsksTheValidatedQuestionWithoutStoringIt(t *testing.T) {
	decider := &scriptedDecider{proposal: domain.FollowUpProposal{
		FollowUp: true, Question: "  Which algorithm would you pick for bursts?  ", Reason: "The answer skipped burst handling.",
	}}
	svc, repo := followUpService(t, decider, 1)
	delivery, err := svc.FollowUp(context.Background(), candidateID, "candidate", sessionID)
	if err != nil || delivery.Question == nil {
		t.Fatalf("FollowUp = %+v, %v", delivery, err)
	}
	if q := delivery.Question; q.Kind != domain.TurnFollowUp || q.Position != 3 || q.QuestionNumber != 2 ||
		q.Text != "Which algorithm would you pick for bursts?" || q.AskedAt.IsZero() {
		t.Fatalf("follow-up = %+v", q)
	}
	if delivery.ProviderVoiceID != "en-US-AvaNeural" || delivery.QuestionCount != 3 || repo.followUpLimit != 1 {
		t.Fatalf("delivery = %+v, limit %d", delivery, repo.followUpLimit)
	}
	if decider.calls != 1 || decider.seen.CoreQuestion != pendingSlot().CoreQuestion || len(decider.seen.Requirements) != 1 {
		t.Fatalf("decider saw %+v after %d calls", decider.seen, decider.calls)
	}
	if repo.appendCalls != 0 || repo.deliverCalls != 0 {
		t.Fatalf("FollowUp wrote %d answers and delivered %d core questions", repo.appendCalls, repo.deliverCalls)
	}
}

func TestFollowUpWithoutAnOpenSlotAsksNothing(t *testing.T) {
	decider := &scriptedDecider{proposal: domain.FollowUpProposal{FollowUp: true, Question: "Why?", Reason: "r"}}
	svc, repo := followUpService(t, decider, 1)
	repo.followUpPending = nil
	delivery, err := svc.FollowUp(context.Background(), candidateID, "candidate", sessionID)
	if err != nil || delivery.Question != nil || decider.calls != 0 {
		t.Fatalf("FollowUp = %+v, %v after %d decisions", delivery, err, decider.calls)
	}
}

func TestFollowUpDeclinedAsksNothing(t *testing.T) {
	ctx, logs := observedContext()
	svc, _ := followUpService(t, &scriptedDecider{proposal: domain.FollowUpProposal{Reason: "The answer was complete."}}, 1)
	delivery, err := svc.FollowUp(ctx, candidateID, "candidate", sessionID)
	if err != nil || delivery.Question != nil {
		t.Fatalf("FollowUp = %+v, %v", delivery, err)
	}
	if fields := onlyEntry(t, logs, "interview follow-up decided").ContextMap(); fields["outcome"] != "declined" {
		t.Fatalf("log fields = %v", fields)
	}
}

func TestFailedDecisionsAreSkippedAndTheInterviewMovesOn(t *testing.T) {
	tests := []struct {
		name    string
		decider *scriptedDecider
		reason  string
	}{
		{name: "provider error", decider: &scriptedDecider{err: errors.New("503 model overloaded")}, reason: followUpProviderError},
		{name: "timeout", decider: &scriptedDecider{block: true}, reason: followUpTimeout},
		{name: "decider ignoring its context", decider: &scriptedDecider{block: true, ignoreContext: true}, reason: followUpTimeout},
		{name: "panic", decider: &scriptedDecider{panicValue: sentinelAnswer}, reason: followUpProviderError},
		{name: "invalid output", decider: &scriptedDecider{proposal: domain.FollowUpProposal{FollowUp: true, Question: "Two? Questions?", Reason: "r"}}, reason: followUpInvalidOutput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, logs := observedContext()
			svc, _ := followUpService(t, tt.decider, 1)
			started := time.Now()
			delivery, err := svc.FollowUp(ctx, candidateID, "candidate", sessionID)
			if err != nil || delivery.Question != nil {
				t.Fatalf("FollowUp = %+v, %v", delivery, err)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("FollowUp took %s with a 200ms decision timeout", elapsed)
			}
			fields := onlyEntry(t, logs, "interview follow-up skipped").ContextMap()
			if fields["reason"] != tt.reason || fields["outcome"] != "skipped" || fields["question_number"] != int64(2) {
				t.Fatalf("log fields = %v", fields)
			}
			if text := loggedText(logs); strings.Contains(text, "SENTINEL") || strings.Contains(text, "rate limiter") {
				t.Fatalf("the answer or the prompt leaked into the logs: %s", text)
			}
		})
	}
}

func TestAppendFollowUpStoresTheAnswerWithItsFollowUp(t *testing.T) {
	ctx, logs := observedContext()
	svc, repo := followUpService(t, &scriptedDecider{}, 1)
	repo.appendSession = practiceSession()
	followUp := domain.AskedQuestion{Position: 3, QuestionNumber: 2, Kind: domain.TurnFollowUp, Text: "Why?", AskedAt: time.Now()}
	if _, err := svc.AppendFollowUp(ctx, candidateID, "candidate", sessionID, followUp, "  "+sentinelAnswer+"  "); err != nil {
		t.Fatalf("AppendFollowUp: %v", err)
	}
	if !reflect.DeepEqual(repo.appendedFollowUp, followUp) || repo.appendAnswer != sentinelAnswer || repo.appendLimit != 1 {
		t.Fatalf("stored %+v / %q with limit %d", repo.appendedFollowUp, repo.appendAnswer, repo.appendLimit)
	}
	if fields := onlyEntry(t, logs, "interview answer recorded").ContextMap(); fields["kind"] != "follow_up" || fields["question_number"] != int64(2) {
		t.Fatalf("log fields = %v", fields)
	}
	if strings.Contains(loggedText(logs), "SENTINEL") {
		t.Fatal("the answer leaked into the logs")
	}

	_, err := svc.AppendFollowUp(ctx, candidateID, "candidate", sessionID, domain.AskedQuestion{Position: 3, Kind: domain.TurnCore, Text: "Why?"}, "a")
	assertCode(t, err, apperror.CodeInternal)
	_, err = svc.AppendFollowUp(ctx, candidateID, "candidate", sessionID, followUp, "   ")
	assertCode(t, err, apperror.CodeValidation)
	_, err = svc.AppendFollowUp(ctx, candidateID, "recruiter", sessionID, followUp, "a")
	assertCode(t, err, apperror.CodeForbidden)
	if repo.appendCalls != 1 {
		t.Fatalf("repository appends = %d, want 1", repo.appendCalls)
	}
}

func TestValidateProposal(t *testing.T) {
	long := strings.Repeat("why ", 80) + "?"
	tests := []struct {
		name     string
		proposal domain.FollowUpProposal
		valid    bool
	}{
		{name: "one question", proposal: domain.FollowUpProposal{FollowUp: true, Question: "What happens under bursty traffic?", Reason: "r"}, valid: true},
		{name: "typographic punctuation", proposal: domain.FollowUpProposal{FollowUp: true, Question: "What’s the trade-off — memory or latency?", Reason: "r"}, valid: true},
		{name: "declined", proposal: domain.FollowUpProposal{Reason: "complete"}, valid: true},
		{name: "declined with a question", proposal: domain.FollowUpProposal{Question: "Why?", Reason: "r"}},
		{name: "no reason", proposal: domain.FollowUpProposal{FollowUp: true, Question: "Why?"}},
		{name: "multi-line reason", proposal: domain.FollowUpProposal{Reason: "a\nb"}},
		{name: "empty question", proposal: domain.FollowUpProposal{FollowUp: true, Reason: "r"}},
		{name: "too long", proposal: domain.FollowUpProposal{FollowUp: true, Question: long, Reason: "r"}},
		{name: "two lines", proposal: domain.FollowUpProposal{FollowUp: true, Question: "First,\nwhy?", Reason: "r"}},
		{name: "two questions", proposal: domain.FollowUpProposal{FollowUp: true, Question: "Why? And how?", Reason: "r"}},
		{name: "not a question", proposal: domain.FollowUpProposal{FollowUp: true, Question: "Tell me more.", Reason: "r"}},
		{name: "not English", proposal: domain.FollowUpProposal{FollowUp: true, Question: "Bạn sẽ xử lý thế nào?", Reason: "r"}},
		{name: "markdown fence", proposal: domain.FollowUpProposal{FollowUp: true, Question: "```Why?```", Reason: "r"}},
		{name: "repeats the core question", proposal: domain.FollowUpProposal{FollowUp: true, Question: "how would you design a rate limiter?", Reason: "r"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal := tt.proposal
			err := validateProposal(&proposal, pendingSlot())
			if tt.valid != (err == nil) {
				t.Fatalf("valid = %v, err = %v", tt.valid, err)
			}
			if err != nil && !errors.Is(err, ErrInvalidFollowUp) {
				t.Fatalf("error %v is not an invalid proposal", err)
			}
		})
	}
}

// A concise guideline lowers the follow-up cap to 150 characters in code, so the effect holds even when the
// model ignores the prompt; the standard cap stays 300 and a guideline never raises it.
func TestValidateProposalHonoursTheGuidelineQuestionLength(t *testing.T) {
	// 200 characters of one-line English ending with its only question mark.
	question := strings.TrimSpace(strings.Repeat("why ", 49)) + " because?"
	if n := len([]rune(question)); n < 160 || n > 250 {
		t.Fatalf("fixture is %d characters, want between the two caps", n)
	}
	cases := []struct {
		name   string
		length string
		valid  bool
	}{
		{"no guideline", "", true},
		{"standard", domain.LengthStandard, true},
		{"concise", domain.LengthConcise, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := pendingSlot()
			in.Guideline.QuestionLength = tc.length
			proposal := domain.FollowUpProposal{FollowUp: true, Question: question, Reason: "r"}
			err := validateProposal(&proposal, in)
			if tc.valid != (err == nil) {
				t.Fatalf("valid = %v, err = %v", tc.valid, err)
			}
			if err != nil && !errors.Is(err, ErrInvalidFollowUp) {
				t.Fatalf("error %v is not an invalid proposal", err)
			}
		})
	}
	// A reply of 150 characters or fewer stays valid under concise.
	in := pendingSlot()
	in.Guideline.QuestionLength = domain.LengthConcise
	short := domain.FollowUpProposal{FollowUp: true, Question: "What happens under bursty traffic?", Reason: "r"}
	if err := validateProposal(&short, in); err != nil {
		t.Fatalf("a short question must stay valid under concise: %v", err)
	}
}

// Cues never decide a proposal's validity: a usable list is kept, anything else is dropped as a whole so
// the live engine falls back to its rules, and a declined proposal keeps none.
func TestValidateProposalKeepsOnlyAPerformableCueList(t *testing.T) {
	question := "Which algorithm would you pick for bursts?" // 7 words
	good := []domain.PerformanceCue{
		{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionCurious},
		{Word: 6, Kind: domain.CueGesture, Name: domain.GestureLeanIn},
	}
	tests := []struct {
		name     string
		cues     []domain.PerformanceCue
		followUp bool
		kept     bool
	}{
		{name: "valid list", cues: good, followUp: true, kept: true},
		{name: "none proposed", followUp: true},
		{name: "unknown emotion", cues: []domain.PerformanceCue{{Word: 0, Kind: domain.CueEmotion, Name: "smug"}}, followUp: true},
		{name: "unknown gesture", cues: []domain.PerformanceCue{{Word: 0, Kind: domain.CueGesture, Name: "shrug"}}, followUp: true},
		{name: "unknown kind", cues: []domain.PerformanceCue{{Word: 0, Kind: "pose", Name: domain.GestureNod}}, followUp: true},
		{name: "word past the question", cues: []domain.PerformanceCue{{Word: 7, Kind: domain.CueGesture, Name: domain.GestureNod}}, followUp: true},
		{name: "negative word", cues: []domain.PerformanceCue{{Word: -1, Kind: domain.CueGesture, Name: domain.GestureNod}}, followUp: true},
		{name: "one bad cue drops the list", cues: append(append([]domain.PerformanceCue{}, good...), domain.PerformanceCue{Word: 1, Kind: domain.CueEmotion, Name: "x"}), followUp: true},
		{name: "duplicate kind on a word", cues: []domain.PerformanceCue{{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionCurious}, {Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionFriendly}}, followUp: true},
		{name: "too many", cues: []domain.PerformanceCue{
			{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionCurious}, {Word: 1, Kind: domain.CueGesture, Name: domain.GestureNod},
			{Word: 2, Kind: domain.CueGesture, Name: domain.GestureNod}, {Word: 3, Kind: domain.CueGesture, Name: domain.GestureNod},
			{Word: 4, Kind: domain.CueGesture, Name: domain.GestureNod},
		}, followUp: true},
		{name: "declined keeps none", cues: good},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal := domain.FollowUpProposal{FollowUp: tt.followUp, Reason: "r", Cues: tt.cues}
			if tt.followUp {
				proposal.Question = question
			}
			if err := validateProposal(&proposal, pendingSlot()); err != nil {
				t.Fatalf("validateProposal = %v", err)
			}
			if tt.kept != (proposal.Cues != nil) {
				t.Fatalf("cues = %+v, kept %v", proposal.Cues, tt.kept)
			}
		})
	}
}

func TestFollowUpCarriesTheValidatedCues(t *testing.T) {
	cues := []domain.PerformanceCue{{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionCurious}}
	decider := &scriptedDecider{proposal: domain.FollowUpProposal{FollowUp: true, Question: "Why that one?", Reason: "r", Cues: cues}}
	svc, _ := followUpService(t, decider, 1)
	delivery, err := svc.FollowUp(context.Background(), candidateID, "candidate", sessionID)
	if err != nil || delivery.Question == nil {
		t.Fatalf("FollowUp = %+v, %v", delivery, err)
	}
	if got := delivery.Question.Cues; len(got) != 1 || got[0] != cues[0] {
		t.Fatalf("cues = %+v, want %+v", got, cues)
	}
}

func TestNewRefusesANegativeFollowUpLimit(t *testing.T) {
	if _, err := New(&fakeRepository{}, Settings{FollowUpLimit: -1}); err == nil {
		t.Fatal("a negative follow-up limit was accepted")
	}
}
