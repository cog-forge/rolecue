package domain

import (
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func core(position, question int32) Turn {
	return Turn{Position: position, QuestionNumber: question, Kind: TurnCore, Answer: "a"}
}

func followUpTurn(position, question int32) Turn {
	return Turn{Position: position, QuestionNumber: question, Kind: TurnFollowUp, Answer: "a"}
}

func TestNextCore(t *testing.T) {
	tests := []struct {
		name  string
		turns []Turn
		count int32
		want  Slot
		ok    bool
	}{
		{name: "first core question", count: 3, want: Slot{Position: 1, QuestionNumber: 1}, ok: true},
		{name: "a session without questions has none", count: 0},
		{name: "core after core", turns: []Turn{core(1, 1)}, count: 3, want: Slot{Position: 2, QuestionNumber: 2}, ok: true},
		{name: "core after a follow-up", turns: []Turn{core(1, 1), followUpTurn(2, 1)}, count: 3, want: Slot{Position: 3, QuestionNumber: 2}, ok: true},
		{name: "done after the last core question", turns: []Turn{core(1, 1), core(2, 2)}, count: 2},
		{name: "done after the last core question's follow-up", turns: []Turn{core(1, 1), core(2, 2), followUpTurn(3, 2)}, count: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NextCore(tt.turns, tt.count)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("NextCore = %+v, %v; want %+v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFollowUpSlot(t *testing.T) {
	tests := []struct {
		name  string
		turns []Turn
		limit int32
		want  Slot
		ok    bool
	}{
		{name: "nothing answered yet", limit: 1},
		{name: "limit 0 never opens a slot", turns: []Turn{core(1, 1)}},
		{name: "after a core answer", turns: []Turn{core(1, 1)}, limit: 1, want: Slot{Position: 2, QuestionNumber: 1, Ordinal: 1}, ok: true},
		{name: "limit 1 is used up by one follow-up", turns: []Turn{core(1, 1), followUpTurn(2, 1)}, limit: 1},
		{name: "limit 2 opens a second follow-up", turns: []Turn{core(1, 1), followUpTurn(2, 1)}, limit: 2, want: Slot{Position: 3, QuestionNumber: 1, Ordinal: 2}, ok: true},
		{name: "only the last core question counts", turns: []Turn{core(1, 1), followUpTurn(2, 1), core(3, 2)}, limit: 1, want: Slot{Position: 4, QuestionNumber: 2, Ordinal: 1}, ok: true},
		{name: "the last core question gets its follow-up", turns: []Turn{core(1, 1), core(2, 2)}, limit: 1, want: Slot{Position: 3, QuestionNumber: 2, Ordinal: 1}, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FollowUpSlot(tt.turns, tt.limit)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("FollowUpSlot = %+v, %v; want %+v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// Whatever a decider returns, a core question never gets more follow-ups than the limit and the session
// always ends: here every answer is followed by a follow-up whenever a slot is open.
func TestFollowUpsAreBoundedWhateverTheDecider(t *testing.T) {
	for limit := int32(0); limit <= 3; limit++ {
		var turns []Turn
		for steps := 0; ; steps++ {
			if steps > 100 {
				t.Fatalf("limit %d: the loop did not terminate", limit)
			}
			if slot, ok := FollowUpSlot(turns, limit); ok {
				turns = append(turns, followUpTurn(slot.Position, slot.QuestionNumber))
				continue
			}
			slot, ok := NextCore(turns, 4)
			if !ok {
				break
			}
			turns = append(turns, core(slot.Position, slot.QuestionNumber))
		}
		perQuestion := map[int32]int32{}
		for _, turn := range turns {
			if turn.Kind == TurnFollowUp {
				perQuestion[turn.QuestionNumber]++
			}
		}
		for question, n := range perQuestion {
			if n > limit {
				t.Fatalf("limit %d: question %d got %d follow-ups", limit, question, n)
			}
		}
		if want := 4 * (1 + int(limit)); len(turns) != want {
			t.Fatalf("limit %d: %d turns, want %d", limit, len(turns), want)
		}
	}
}

func TestAcceptAnswerAndFollowUp(t *testing.T) {
	answeredCore := []Turn{core(1, 1)}
	tests := []struct {
		name      string
		followUp  bool
		turns     []Turn
		position  int32
		unchanged bool
		wantCode  apperror.Code
	}{
		{name: "core answer", position: 1},
		{name: "core answer after an unanswered follow-up was dropped", turns: answeredCore, position: 2},
		{name: "retry of a stored core answer", turns: answeredCore, position: 1, unchanged: true},
		{name: "core answer beyond the last question", turns: []Turn{core(1, 1), core(2, 2)}, position: 3, wantCode: apperror.CodeInterviewTurnConflict},
		{name: "follow-up answer", followUp: true, turns: answeredCore, position: 2},
		{name: "retry of a stored follow-up answer", followUp: true, turns: []Turn{core(1, 1), followUpTurn(2, 1)}, position: 2, unchanged: true},
		{name: "follow-up answer stored as a core answer", turns: []Turn{core(1, 1), followUpTurn(2, 1)}, position: 2, wantCode: apperror.CodeInterviewTurnConflict},
		{name: "follow-up over the limit", followUp: true, turns: []Turn{core(1, 1), followUpTurn(2, 1)}, position: 3, wantCode: apperror.CodeInterviewTurnConflict},
		{name: "follow-up before any core answer", followUp: true, position: 1, wantCode: apperror.CodeInterviewTurnConflict},
		{name: "follow-up at a skipped position", followUp: true, turns: answeredCore, position: 3, wantCode: apperror.CodeInterviewTurnConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var slot Slot
			var unchanged bool
			var err error
			if tt.followUp {
				slot, unchanged, err = AcceptFollowUp(tt.turns, 1, tt.position, "a")
			} else {
				slot, unchanged, err = AcceptAnswer(tt.turns, 2, tt.position, "a")
			}
			if tt.wantCode != "" {
				if !apperror.IsCode(err, tt.wantCode) {
					t.Fatalf("error = %v, want %s", err, tt.wantCode)
				}
				return
			}
			if err != nil || unchanged != tt.unchanged {
				t.Fatalf("unchanged = %v, err = %v", unchanged, err)
			}
			if !unchanged && slot.Position != tt.position {
				t.Fatalf("slot = %+v", slot)
			}
		})
	}
}
