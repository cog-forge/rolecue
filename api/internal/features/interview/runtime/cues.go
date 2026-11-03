package runtime

import (
	"sort"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
)

// Performance cues (KAN-250). The server decides how a line is performed; the client plays it (KAN-58).
// No model call is made for it: core questions get rule-based cues, and a follow-up may carry the cues
// its decider proposed (validated by the service). Every cue is anchored to a word of the line and
// converted to a millisecond offset with the TTS word boundaries, so it lands on the word as spoken.
//
// The cues are a pure function of what is persisted (the turn's kind, position and frozen text) and of
// the synthesized word timings, which the synthesis cache keeps per voice and text. A reconnect
// therefore replays exactly the same cues without a database column.

const (
	// longQuestionWords is the length from which a question is "explained" with a hand gesture.
	longQuestionWords = 18
	// estimatedWordMs anchors cues when the synthesizer sent no word timings (REST adapter, text-only
	// turn): about 160 words per minute.
	estimatedWordMs = 380
)

// wordCue is a cue before anchoring.
type wordCue = domain.PerformanceCue

// planCues decides the cues of one turn and anchors them to the word timings.
func planCues(q domain.AskedQuestion, words []WordTiming) []Cue {
	tokens := strings.Fields(q.Text)
	if len(tokens) == 0 {
		return []Cue{}
	}
	var plan []wordCue
	if q.Kind == domain.TurnFollowUp {
		plan = followUpCues(q.Cues, tokens)
	} else {
		plan = coreCues(q.Position, tokens)
	}
	return anchorCues(plan, len(tokens), words)
}

// coreCues are the rules for a core question. The first turn greets; a later turn first encourages (it
// follows an answer) and then turns to the question itself; a long question is explained with a hand;
// the default is curious for a question and neutral for a statement.
func coreCues(position int32, tokens []string) []wordCue {
	questionStart := lastSentenceStart(tokens)
	base := domain.EmotionNeutral
	if strings.Contains(strings.Join(tokens, " "), "?") {
		base = domain.EmotionCurious
	}
	var plan []wordCue
	if position <= 1 {
		plan = append(plan,
			wordCue{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionFriendly},
			wordCue{Word: 0, Kind: domain.CueGesture, Name: domain.GestureGreet},
		)
		if questionStart > 0 {
			plan = append(plan, wordCue{Word: questionStart, Kind: domain.CueEmotion, Name: base})
		}
	} else {
		plan = append(plan,
			wordCue{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionEncouraging},
			wordCue{Word: 0, Kind: domain.CueGesture, Name: domain.GestureNod},
		)
		// A one-sentence question still moves from the encouraging transition to the question.
		at := questionStart
		if at == 0 {
			at = min(2, len(tokens)-1)
		}
		if at > 0 {
			plan = append(plan, wordCue{Word: at, Kind: domain.CueEmotion, Name: base})
		}
	}
	if len(tokens) >= longQuestionWords {
		at := secondSentenceStart(tokens)
		if at == 0 {
			at = len(tokens) / 3
		}
		plan = append(plan, wordCue{Word: at, Kind: domain.CueGesture, Name: domain.GestureExplainHand})
	}
	return plan
}

// followUpCues performs a follow-up with the decider's cues when it proposed a usable list, else probes
// with the rule: curious and leaning in. A proposed list that opens without an emotion gets curious first.
func followUpCues(proposed []domain.PerformanceCue, tokens []string) []wordCue {
	if len(proposed) == 0 || !domain.ValidCues(proposed, len(tokens)) {
		return []wordCue{
			{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionCurious},
			{Word: 0, Kind: domain.CueGesture, Name: domain.GestureLeanIn},
		}
	}
	plan := make([]wordCue, 0, len(proposed)+1)
	opensWithEmotion := false
	for _, cue := range proposed {
		if cue.Kind == domain.CueEmotion && cue.Word == 0 {
			opensWithEmotion = true
		}
	}
	if !opensWithEmotion {
		plan = append(plan, wordCue{Word: 0, Kind: domain.CueEmotion, Name: domain.EmotionCurious})
	}
	return append(plan, proposed...)
}

// anchorCues converts word indexes to millisecond offsets. With word timings the cue starts when its
// word does (an index past the timings lands on the last word); without them the offset is estimated
// from the speaking rate. The result is sorted by offset, then emotion before gesture, and stable.
func anchorCues(plan []wordCue, wordCount int, words []WordTiming) []Cue {
	cues := make([]Cue, 0, len(plan))
	for _, cue := range plan {
		word := cue.Word
		if word >= wordCount {
			word = wordCount - 1
		}
		if word < 0 {
			word = 0
		}
		offset := word * estimatedWordMs
		if len(words) > 0 {
			offset = words[min(word, len(words)-1)].OffsetMs
		}
		cues = append(cues, Cue{OffsetMs: offset, Kind: string(cue.Kind), Name: cue.Name})
	}
	sort.SliceStable(cues, func(i, j int) bool {
		if cues[i].OffsetMs != cues[j].OffsetMs {
			return cues[i].OffsetMs < cues[j].OffsetMs
		}
		return cues[i].Kind < cues[j].Kind
	})
	return cues
}

// lastSentenceStart is the index of the first word of the last sentence, 0 when the text is one sentence.
func lastSentenceStart(tokens []string) int {
	start := 0
	for i := 0; i < len(tokens)-1; i++ {
		if endsSentence(tokens[i]) {
			start = i + 1
		}
	}
	return start
}

// secondSentenceStart is the index of the first word of the second sentence, 0 when there is none.
func secondSentenceStart(tokens []string) int {
	for i := 0; i < len(tokens)-1; i++ {
		if endsSentence(tokens[i]) {
			return i + 1
		}
	}
	return 0
}

func endsSentence(token string) bool {
	token = strings.TrimRight(token, `"')]”’`)
	return strings.HasSuffix(token, ".") || strings.HasSuffix(token, "?") || strings.HasSuffix(token, "!")
}
