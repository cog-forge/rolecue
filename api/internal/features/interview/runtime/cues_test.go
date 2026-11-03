package runtime

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
)

// words spaces one timing per token, 500 ms apart, so an anchored offset names its word index.
func words(n int) []WordTiming {
	out := make([]WordTiming, n)
	for i := range out {
		out[i] = WordTiming{OffsetMs: i * 500, DurationMs: 300, Text: "w"}
	}
	return out
}

func core(position int32, text string) domain.AskedQuestion {
	return domain.AskedQuestion{Position: position, QuestionNumber: position, Kind: domain.TurnCore, Text: text}
}

func TestVocabularyIsBounded(t *testing.T) {
	valid := []domain.PerformanceCue{
		{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionNeutral},
		{Word: 3, Kind: domain.CueEmotion, Name: domain.EmotionThinking},
		{Word: 0, Kind: domain.CueGesture, Name: domain.GestureGreet},
		{Word: 9, Kind: domain.CueGesture, Name: domain.GestureListen},
	}
	for _, cue := range valid {
		if !cue.Valid() {
			t.Errorf("%+v should be valid", cue)
		}
	}
	invalid := []domain.PerformanceCue{
		{Word: 0, Kind: domain.CueEmotion, Name: "happy"},
		{Word: 0, Kind: domain.CueGesture, Name: "wave"},
		{Word: 0, Kind: domain.CueEmotion, Name: domain.GestureNod},
		{Word: 0, Kind: domain.CueGesture, Name: domain.EmotionCurious},
		{Word: 0, Kind: "dance", Name: domain.GestureNod},
		{Word: -1, Kind: domain.CueGesture, Name: domain.GestureNod},
		{Word: 0, Kind: domain.CueEmotion, Name: "Curious"},
	}
	for _, cue := range invalid {
		if cue.Valid() {
			t.Errorf("%+v should be invalid", cue)
		}
	}
}

func TestCoreRulesGreetEncourageAndExplain(t *testing.T) {
	tests := []struct {
		name string
		q    domain.AskedQuestion
		want []Cue
	}{
		{
			name: "first turn greets then asks",
			q:    core(1, "Welcome to the interview. What drew you to backend work?"),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "friendly"},
				{OffsetMs: 0, Kind: "gesture", Name: "greet"},
				{OffsetMs: 2000, Kind: "emotion", Name: "curious"},
			},
		},
		{
			name: "first turn with one sentence stays friendly",
			q:    core(1, "What drew you to backend work?"),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "friendly"},
				{OffsetMs: 0, Kind: "gesture", Name: "greet"},
			},
		},
		{
			name: "later turn encourages, nods, then turns curious at the question",
			q:    core(2, "Thanks for that. How would you shard the table?"),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "encouraging"},
				{OffsetMs: 0, Kind: "gesture", Name: "nod"},
				{OffsetMs: 1500, Kind: "emotion", Name: "curious"},
			},
		},
		{
			name: "later one-sentence question moves on after two words",
			q:    core(3, "How would you shard the table?"),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "encouraging"},
				{OffsetMs: 0, Kind: "gesture", Name: "nod"},
				{OffsetMs: 1000, Kind: "emotion", Name: "curious"},
			},
		},
		{
			name: "statement defaults to neutral",
			q:    core(3, "Describe the last outage you handled."),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "encouraging"},
				{OffsetMs: 0, Kind: "gesture", Name: "nod"},
				{OffsetMs: 1000, Kind: "emotion", Name: "neutral"},
			},
		},
		{
			name: "long question is explained with a hand from its second sentence",
			q: core(4, "Imagine a service that receives ten thousand writes per second from mobile clients. "+
				"How would you design its storage layer so reads stay fast and nothing is lost?"),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "encouraging"},
				{OffsetMs: 0, Kind: "gesture", Name: "nod"},
				{OffsetMs: 6500, Kind: "emotion", Name: "curious"},
				{OffsetMs: 6500, Kind: "gesture", Name: "explain_hand"},
			},
		},
		{
			name: "long single sentence is explained a third of the way in",
			q: core(1, "Walk me through how you would design a globally distributed rate limiter "+
				"that stays fair under bursty traffic and partial outages?"),
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "friendly"},
				{OffsetMs: 0, Kind: "gesture", Name: "greet"},
				{OffsetMs: 3500, Kind: "gesture", Name: "explain_hand"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planCues(tt.q, words(40))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("cues = %+v\nwant   %+v", got, tt.want)
			}
			for _, cue := range got {
				if !(domain.PerformanceCue{Kind: domain.CueKind(cue.Kind), Name: cue.Name}).Valid() {
					t.Fatalf("rule produced a cue outside the vocabulary: %+v", cue)
				}
			}
		})
	}
}

func TestAnchoringUsesWordTimingsOrAnEstimate(t *testing.T) {
	q := core(2, "Thanks for that. How would you shard the table?")
	timed := []WordTiming{
		{OffsetMs: 120, DurationMs: 300, Text: "Thanks"}, {OffsetMs: 450, DurationMs: 100, Text: "for"},
		{OffsetMs: 600, DurationMs: 250, Text: "that"}, {OffsetMs: 1375, DurationMs: 200, Text: "How"},
	}
	got := planCues(q, timed)
	// The transition lands on "Thanks" as spoken and the question on "How"; both at real offsets.
	want := []Cue{
		{OffsetMs: 120, Kind: "emotion", Name: "encouraging"},
		{OffsetMs: 120, Kind: "gesture", Name: "nod"},
		{OffsetMs: 1375, Kind: "emotion", Name: "curious"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cues = %+v\nwant   %+v", got, want)
	}
	// Fewer timings than words: a later word lands on the last timing instead of past the audio.
	got = planCues(q, timed[:2])
	if got[2].OffsetMs != 450 {
		t.Fatalf("cue past the timings = %+v, want the last timing", got[2])
	}
	// No timings (REST synthesizer): a speaking-rate estimate keeps the order and the word anchoring.
	got = planCues(q, nil)
	if got[0].OffsetMs != 0 || got[2].OffsetMs != 3*estimatedWordMs {
		t.Fatalf("estimated cues = %+v", got)
	}
	if got := planCues(core(1, "   "), nil); len(got) != 0 || got == nil {
		t.Fatalf("blank text cues = %#v, want an empty list", got)
	}
}

func TestFollowUpUsesProposedCuesOrTheProbingRule(t *testing.T) {
	text := "Which algorithm would you pick for bursts?" // 7 words
	followUp := func(cues []domain.PerformanceCue) domain.AskedQuestion {
		return domain.AskedQuestion{Position: 3, QuestionNumber: 2, Kind: domain.TurnFollowUp, Text: text, Cues: cues}
	}
	rule := []Cue{
		{OffsetMs: 0, Kind: "emotion", Name: "curious"},
		{OffsetMs: 0, Kind: "gesture", Name: "lean_in"},
	}
	tests := []struct {
		name string
		cues []domain.PerformanceCue
		want []Cue
	}{
		{name: "no proposal", want: rule},
		{
			name: "proposed cues are anchored and ordered",
			cues: []domain.PerformanceCue{
				{Word: 6, Kind: domain.CueGesture, Name: domain.GestureNod},
				{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionEncouraging},
				{Word: 2, Kind: domain.CueGesture, Name: domain.GestureExplainHand},
			},
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "encouraging"},
				{OffsetMs: 1000, Kind: "gesture", Name: "explain_hand"},
				{OffsetMs: 3000, Kind: "gesture", Name: "nod"},
			},
		},
		{
			name: "a proposal without an opening emotion gets curious first",
			cues: []domain.PerformanceCue{{Word: 3, Kind: domain.CueGesture, Name: domain.GestureLeanIn}},
			want: []Cue{
				{OffsetMs: 0, Kind: "emotion", Name: "curious"},
				{OffsetMs: 1500, Kind: "gesture", Name: "lean_in"},
			},
		},
		// The service drops invalid lists, but the engine checks again: it never performs outside
		// the vocabulary, whatever reached it.
		{name: "invalid name falls back to the rule", cues: []domain.PerformanceCue{{Word: 0, Kind: domain.CueGesture, Name: "backflip"}}, want: rule},
		{name: "word past the line falls back to the rule", cues: []domain.PerformanceCue{{Word: 7, Kind: domain.CueGesture, Name: domain.GestureNod}}, want: rule},
		{
			name: "more than four cues fall back to the rule",
			cues: []domain.PerformanceCue{
				{Word: 0, Kind: domain.CueGesture, Name: domain.GestureNod}, {Word: 1, Kind: domain.CueGesture, Name: domain.GestureNod},
				{Word: 2, Kind: domain.CueGesture, Name: domain.GestureNod}, {Word: 3, Kind: domain.CueGesture, Name: domain.GestureNod},
				{Word: 4, Kind: domain.CueGesture, Name: domain.GestureNod},
			},
			want: rule,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planCues(followUp(tt.cues), words(7))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("cues = %+v\nwant   %+v", got, tt.want)
			}
		})
	}
}

// The cues are derived from the persisted turn and the cached synthesis, so a reconnect that re-sends the
// turn carries exactly the same cues, and a text-only turn carries none.
func TestReconnectReplaysTheSameCuesAndTextOnlyTurnsHaveNone(t *testing.T) {
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Welcome aboard. What drew you to backend work?", "Second?"), withCache(4))
	first, _, turn := h.connect()
	if len(turn.Words) == 0 || len(turn.Cues) != 3 || turn.Cues[1].Name != "greet" || turn.Cues[2].OffsetMs != turn.Words[2].OffsetMs {
		t.Fatalf("first delivery: %d words, cues %+v", len(turn.Words), turn.Cues)
	}
	_ = first.ws.Close()

	_, _, replayed := h.connect()
	if !reflect.DeepEqual(replayed.Cues, turn.Cues) {
		t.Fatalf("replayed cues %+v differ from %+v", replayed.Cues, turn.Cues)
	}

	silent := newHarness(t, newFakeSessions(domain.TypePractice, "Welcome aboard. What drew you to backend work?"))
	silent.sessions.set(func(f *fakeSessions) { f.voice = "" })
	c, _, textOnly := silent.connect()
	if textOnly.AudioStatus != AudioNoVoice || len(textOnly.Cues) != 0 || !bytes.Contains(c.seen[len(c.seen)-1].Data, []byte(`"cues":[]`)) {
		t.Fatalf("text-only turn = %+v", textOnly)
	}
}
