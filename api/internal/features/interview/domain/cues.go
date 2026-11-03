package domain

// CueKind tells an emotion cue from a gesture cue (KAN-250).
type CueKind string

const (
	CueEmotion CueKind = "emotion"
	CueGesture CueKind = "gesture"
)

// The bounded performance vocabulary. The interviewer never performs anything outside it, whatever a
// model proposes: an unknown name makes the whole proposed cue list fall back to the rules.
const (
	EmotionNeutral     = "neutral"
	EmotionFriendly    = "friendly"
	EmotionCurious     = "curious"
	EmotionEncouraging = "encouraging"
	EmotionThinking    = "thinking"

	GestureNod         = "nod"
	GestureExplainHand = "explain_hand"
	GestureLeanIn      = "lean_in"
	GestureGreet       = "greet"
	GestureListen      = "listen"
)

// MaxProposedCues bounds the cues a follow-up decider may attach to one line.
const MaxProposedCues = 4

var (
	emotionNames = map[string]struct{}{
		EmotionNeutral: {}, EmotionFriendly: {}, EmotionCurious: {}, EmotionEncouraging: {}, EmotionThinking: {},
	}
	gestureNames = map[string]struct{}{
		GestureNod: {}, GestureExplainHand: {}, GestureLeanIn: {}, GestureGreet: {}, GestureListen: {},
	}
)

// PerformanceCue is one emotion or gesture anchored to a word of the spoken line: Word is the 0-based
// index of the word the cue starts on. The live engine converts it to a millisecond offset with the TTS
// word boundaries.
type PerformanceCue struct {
	Word int
	Kind CueKind
	Name string
}

// Valid reports whether the cue uses the bounded vocabulary and a non-negative word index.
func (c PerformanceCue) Valid() bool {
	if c.Word < 0 {
		return false
	}
	switch c.Kind {
	case CueEmotion:
		_, ok := emotionNames[c.Name]
		return ok
	case CueGesture:
		_, ok := gestureNames[c.Name]
		return ok
	}
	return false
}

// ValidCues reports whether a proposed cue list can be performed as it is: at most MaxProposedCues, every
// cue valid, every word index below wordCount and no two cues of the same kind on the same word.
func ValidCues(cues []PerformanceCue, wordCount int) bool {
	if len(cues) > MaxProposedCues {
		return false
	}
	type key struct {
		word int
		kind CueKind
	}
	seen := make(map[key]struct{}, len(cues))
	for _, cue := range cues {
		if !cue.Valid() || cue.Word >= wordCount {
			return false
		}
		k := key{cue.Word, cue.Kind}
		if _, dup := seen[k]; dup {
			return false
		}
		seen[k] = struct{}{}
	}
	return true
}
