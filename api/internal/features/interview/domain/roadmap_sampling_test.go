package domain

import (
	"testing"

	"github.com/google/uuid"
)

func roadmapBank(topic uuid.UUID, assessed ...bool) []RoadmapBankQuestion {
	out := make([]RoadmapBankQuestion, len(assessed))
	q := ids(0xd0, len(assessed))
	for i, done := range assessed {
		id := topic
		out[i] = RoadmapBankQuestion{SamplingQuestion: question(q[i]), TopicID: &id, Assessed: done}
	}
	return out
}

func TestGeneralRanksPutUnassessedQuestionsOfOpenTopicsFirst(t *testing.T) {
	open, done := uuid.New(), uuid.New()
	bank := append(roadmapBank(open, false, true), roadmapBank(done, false, true)...)
	bank = append(bank, RoadmapBankQuestion{SamplingQuestion: question(uuid.New()), Assessed: false}) // legacy, no topic
	got := GeneralPracticeRanks(bank, map[uuid.UUID]bool{done: true})
	want := []int{0, 1, 2, 3, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ranks = %v, want %v", got, want)
		}
	}
}

func TestFocusedBankTakesOnlyTheTopicAndNeverFallsBack(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	bank := append(roadmapBank(a, false, false, true), roadmapBank(b, false, false, false, false, false)...)
	bank = append(bank, RoadmapBankQuestion{SamplingQuestion: question(uuid.New())})
	got := FocusedBank(bank, a)
	if len(got) != 3 {
		t.Fatalf("focused bank = %d questions, want only the 3 of the topic", len(got))
	}
	for _, question := range got {
		if question.TopicID == nil || *question.TopicID != a {
			t.Fatal("a question of another topic got into a focused bank")
		}
	}
	if len(FocusedBank(bank, uuid.New())) != 0 {
		t.Fatal("an unknown topic must give an empty bank")
	}
}

func TestFocusedRanksPreferUnassessedQuestions(t *testing.T) {
	topic := uuid.New()
	ranks := FocusedRanks(roadmapBank(topic, true, false, true, false))
	want := []int{1, 0, 1, 0}
	for i := range want {
		if ranks[i] != want[i] {
			t.Fatalf("ranks = %v, want %v", ranks, want)
		}
	}
}

func TestSamplingQuestionsKeepsTheOrderAndIDs(t *testing.T) {
	topic := uuid.New()
	bank := roadmapBank(topic, false, true, false)
	got := SamplingQuestions(bank)
	for i := range bank {
		if got[i].ID != bank[i].ID {
			t.Fatal("order or ids changed")
		}
	}
}
