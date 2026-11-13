package domain

import (
	"testing"

	"github.com/google/uuid"
)

func topicsOf(n int) []RoadmapTopic {
	out := make([]RoadmapTopic, n)
	for i := range out {
		out[i] = RoadmapTopic{ID: uuid.New(), Title: string(rune('A' + i))}
	}
	return out
}

func sum(topics []RoadmapTopic) int {
	total := 0
	for _, topic := range topics {
		total += topic.Quota
	}
	return total
}

func TestPracticeBankSizeNeverFallsBelowTheDistributionRule(t *testing.T) {
	cases := []struct{ configured, topics, want int }{{50, 8, 50}, {20, 4, 20}, {20, 5, 25}, {50, 12, 60}, {0, 4, 20}, {50, 0, 50}}
	for _, c := range cases {
		if got := PracticeBankSize(c.configured, c.topics); got != c.want {
			t.Errorf("PracticeBankSize(%d, %d) = %d, want %d", c.configured, c.topics, got, c.want)
		}
	}
}

func TestBalancedQuotasSumToSizeAndFavourEarlierTopics(t *testing.T) {
	quotas := BalancedQuotas(topicsOf(4), 22)
	want := []int{6, 6, 5, 5}
	for i, topic := range quotas {
		if topic.Quota != want[i] {
			t.Fatalf("quota %d = %d, want %d (%v)", i, topic.Quota, want[i], quotas)
		}
	}
	if sum(quotas) != 22 {
		t.Fatal("quotas must sum to the bank size")
	}
	if len(BalancedQuotas(nil, 10)) != 0 {
		t.Fatal("no topics, no quotas")
	}
}

func TestBatchQuotasFillTheMostMissingTopicsFirst(t *testing.T) {
	topics := BalancedQuotas(topicsOf(3), 15) // 5 each
	have := map[uuid.UUID]int{topics[0].ID: 5, topics[1].ID: 2}
	batch := BatchQuotas(topics, have, 6)
	if sum(batch) != 6 {
		t.Fatalf("batch sum = %d, want 6: %+v", sum(batch), batch)
	}
	for _, topic := range batch {
		if topic.ID == topics[0].ID {
			t.Fatal("a topic that already has its quota must not be asked for more")
		}
	}
	rest := BatchQuotas(topics, map[uuid.UUID]int{topics[0].ID: 5, topics[1].ID: 5, topics[2].ID: 4}, 10)
	if sum(rest) != 1 || len(rest) != 1 || rest[0].ID != topics[2].ID {
		t.Fatalf("last batch = %+v, want one question for the last topic", rest)
	}
}

func TestBatchQuotasOfAFullBankIsEmpty(t *testing.T) {
	topics := BalancedQuotas(topicsOf(2), 10)
	if got := BatchQuotas(topics, map[uuid.UUID]int{topics[0].ID: 5, topics[1].ID: 5}, 10); len(got) != 0 {
		t.Fatalf("batch = %+v, want none", got)
	}
}

func TestCountByTopicIgnoresUnmappedQuestions(t *testing.T) {
	id := uuid.New()
	counts := CountByTopic([]GeneratedQuestion{{TopicID: &id}, {TopicID: &id}, {}})
	if len(counts) != 1 || counts[id] != 2 {
		t.Fatalf("counts = %v", counts)
	}
}
