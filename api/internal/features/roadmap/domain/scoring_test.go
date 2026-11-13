package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
)

var baseTime = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// at returns baseTime shifted by the given number of hours.
func at(hours int) time.Time { return baseTime.Add(time.Duration(hours) * time.Hour) }

// uid builds a deterministic UUID whose byte order follows n, so tests can reason about tie-breaks.
func uid(n byte) uuid.UUID {
	var id uuid.UUID
	id[15] = n
	return id
}

// ev builds one evidence row: core question n answered in interview iv at position pos, completed at hour h.
func ev(core byte, score float64, h int, iv byte, pos int32) domain.Evidence {
	return domain.Evidence{CoreQuestionID: uid(core), Score: score, EventTime: at(h), InterviewID: uid(iv), Position: pos}
}

func mustScoreTopic(t *testing.T, rows []domain.Evidence) domain.TopicScore {
	t.Helper()
	got, err := domain.ScoreTopic(rows)
	if err != nil {
		t.Fatalf("ScoreTopic: %v", err)
	}
	return got
}

func wantScore(t *testing.T, got domain.TopicScore, want float64, count int, status domain.TopicStatus) {
	t.Helper()
	if got.Score == nil {
		t.Fatalf("score = nil, want %v", want)
	}
	if math.Abs(*got.Score-want) > 1e-9 {
		t.Errorf("score = %v, want %v", *got.Score, want)
	}
	if got.EvidenceQuestionCount != count {
		t.Errorf("count = %d, want %d", got.EvidenceQuestionCount, count)
	}
	if got.Status != status {
		t.Errorf("status = %s, want %s", got.Status, status)
	}
}

func TestScoreTopicNoEvidence(t *testing.T) {
	for name, rows := range map[string][]domain.Evidence{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			got := mustScoreTopic(t, rows)
			if got.Score != nil || got.EvidenceQuestionCount != 0 || got.Status != domain.TopicNotStarted || got.LastEvaluatedAt != nil {
				t.Fatalf("got %+v, want nil score, 0 questions, not_started, nil time", got)
			}
			if len(got.InCurrentWindow) != 0 {
				t.Fatalf("marks = %v, want none", got.InCurrentWindow)
			}
		})
	}
}

func TestScoreTopicProvisionalOneToFour(t *testing.T) {
	// Spec 4.3: INT-01 SQL Q1=60, Q2=80 -> provisional 70, 2/5.
	got := mustScoreTopic(t, []domain.Evidence{ev(1, 60, 0, 1, 1), ev(2, 80, 0, 1, 2)})
	wantScore(t, got, 70, 2, domain.TopicInProgress)

	// A single very high score is provisional, never achieved.
	got = mustScoreTopic(t, []domain.Evidence{ev(1, 100, 0, 1, 1)})
	wantScore(t, got, 100, 1, domain.TopicInProgress)

	// Four questions averaging above the threshold are still not achieved.
	got = mustScoreTopic(t, []domain.Evidence{ev(1, 100, 0, 1, 1), ev(2, 100, 1, 1, 2), ev(3, 100, 2, 1, 3), ev(4, 100, 3, 1, 4)})
	wantScore(t, got, 100, 4, domain.TopicInProgress)
}

func TestScoreTopicFiveQuestionsThreshold(t *testing.T) {
	tests := []struct {
		name   string
		scores [5]float64
		mean   float64
		status domain.TopicStatus
	}{
		{"spec example 82 is achieved", [5]float64{70, 80, 90, 80, 90}, 82, domain.TopicAchieved},
		{"exactly 80 is achieved", [5]float64{80, 80, 80, 80, 80}, 80, domain.TopicAchieved},
		{"exactly 80 from mixed scores", [5]float64{60, 100, 70, 90, 80}, 80, domain.TopicAchieved},
		{"79.99 is not achieved", [5]float64{79.99, 79.99, 79.99, 79.99, 79.99}, 79.99, domain.TopicInProgress},
		{"mean 79.99 from mixed scores", [5]float64{80, 80, 80, 80, 79.95}, 79.99, domain.TopicInProgress},
		{"mean 80.00 from 2-decimal inputs", [5]float64{79.99, 80.01, 79.98, 80.02, 80}, 80, domain.TopicAchieved},
		{"all zero", [5]float64{0, 0, 0, 0, 0}, 0, domain.TopicInProgress},
		{"all perfect", [5]float64{100, 100, 100, 100, 100}, 100, domain.TopicAchieved},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rows []domain.Evidence
			for i, s := range tc.scores {
				rows = append(rows, ev(byte(i+1), s, i, 1, int32(i+1)))
			}
			wantScore(t, mustScoreTopic(t, rows), tc.mean, 5, tc.status)
		})
	}
}

func TestScoreTopicDoesNotRoundUpToAchieved(t *testing.T) {
	// Mean is 79.996 which rounds to a displayed 80.00; the comparison must use the unrounded mean.
	rows := []domain.Evidence{ev(1, 80, 0, 1, 1), ev(2, 80, 1, 1, 2), ev(3, 80, 2, 1, 3), ev(4, 80, 3, 1, 4), ev(5, 79.98, 4, 1, 5)}
	got := mustScoreTopic(t, rows)
	if got.Status != domain.TopicInProgress {
		t.Fatalf("status = %s, want in_progress (mean 79.996 < 80)", got.Status)
	}
	if got.Score == nil || *got.Score != 80 {
		t.Fatalf("displayed score = %v, want 80 (rounded half-up to 2 decimals)", got.Score)
	}
}

func TestScoreTopicRepeatedQuestionKeepsLatest(t *testing.T) {
	// Spec 4.3: SQL = 70,80,90,80,90 = 82; the question that scored 70 is answered again and gets 20 -> 72.
	rows := []domain.Evidence{
		ev(1, 70, 0, 1, 1), ev(2, 80, 0, 1, 2), ev(3, 90, 1, 2, 1), ev(4, 80, 1, 2, 2), ev(5, 90, 2, 3, 1),
	}
	wantScore(t, mustScoreTopic(t, rows), 82, 5, domain.TopicAchieved)

	rows = append(rows, ev(1, 20, 3, 4, 1))
	got := mustScoreTopic(t, rows)
	wantScore(t, got, 72, 5, domain.TopicInProgress)

	// The superseded 70 stays in history but is outside the window.
	if got.InCurrentWindow[0] {
		t.Error("superseded result must not be in the current window")
	}
	if !got.InCurrentWindow[5] {
		t.Error("latest result of the repeated question must be in the window")
	}
}

func TestScoreTopicRepeatedQuestionDoesNotAddDistinctCount(t *testing.T) {
	rows := []domain.Evidence{ev(1, 50, 0, 1, 1), ev(1, 60, 1, 2, 1), ev(1, 70, 2, 3, 1), ev(1, 80, 3, 4, 1), ev(1, 90, 4, 5, 1)}
	wantScore(t, mustScoreTopic(t, rows), 90, 1, domain.TopicInProgress)
}

func TestScoreTopicAnswerGetsBetterOrWorseLatestWins(t *testing.T) {
	// The latest answer wins even when it is lower than an earlier one: no best-ever score.
	rows := []domain.Evidence{ev(1, 100, 0, 1, 1), ev(1, 10, 5, 2, 1)}
	wantScore(t, mustScoreTopic(t, rows), 10, 1, domain.TopicInProgress)
}

func TestScoreTopicMoreThanFiveKeepsMostRecentFive(t *testing.T) {
	// Seven distinct questions, oldest first: 10, 20, then 90 x5. Only the five most recent count.
	rows := []domain.Evidence{
		ev(1, 10, 0, 1, 1), ev(2, 20, 1, 1, 2),
		ev(3, 90, 2, 2, 1), ev(4, 90, 3, 2, 2), ev(5, 90, 4, 3, 1), ev(6, 90, 5, 3, 2), ev(7, 90, 6, 4, 1),
	}
	got := mustScoreTopic(t, rows)
	wantScore(t, got, 90, 5, domain.TopicAchieved)
	for i, in := range got.InCurrentWindow {
		if want := i >= 2; in != want {
			t.Errorf("row %d in window = %v, want %v", i, in, want)
		}
	}
}

func TestScoreTopicWindowIgnoresAverageOfAllHistory(t *testing.T) {
	// Older low scores fall out of the window as newer questions arrive: 6th question pushes Q1 out.
	rows := []domain.Evidence{
		ev(1, 0, 0, 1, 1), ev(2, 80, 1, 1, 2), ev(3, 80, 2, 1, 3), ev(4, 80, 3, 1, 4), ev(5, 80, 4, 1, 5),
	}
	wantScore(t, mustScoreTopic(t, rows), 64, 5, domain.TopicInProgress)
	rows = append(rows, ev(6, 80, 5, 2, 1))
	wantScore(t, mustScoreTopic(t, rows), 80, 5, domain.TopicAchieved)
}

func TestScoreTopicScoreDropMovesBackToInProgress(t *testing.T) {
	rows := []domain.Evidence{ev(1, 80, 0, 1, 1), ev(2, 80, 0, 1, 2), ev(3, 80, 1, 2, 1), ev(4, 80, 1, 2, 2), ev(5, 80, 2, 3, 1)}
	wantScore(t, mustScoreTopic(t, rows), 80, 5, domain.TopicAchieved)
	rows = append(rows, ev(6, 40, 3, 4, 1))
	wantScore(t, mustScoreTopic(t, rows), 72, 5, domain.TopicInProgress)
}

func TestScoreTopicEventTimeTieBreakIsStable(t *testing.T) {
	// Six distinct questions in one interview completed at the same instant: the tie is broken by position
	// (higher = later), so Q1 at position 1 falls out of the window.
	var rows []domain.Evidence
	for i := 1; i <= 6; i++ {
		rows = append(rows, ev(byte(i), float64(i*10), 0, 1, int32(i)))
	}
	got := mustScoreTopic(t, rows)
	wantScore(t, got, 40, 5, domain.TopicInProgress) // (20+30+40+50+60)/5
	if got.InCurrentWindow[0] {
		t.Error("lowest position at the same event time must be dropped first")
	}

	// Same event time across interviews: the higher interview id wins the tie.
	rows = []domain.Evidence{
		ev(1, 10, 0, 1, 1), ev(2, 20, 0, 2, 1), ev(3, 30, 0, 3, 1), ev(4, 40, 0, 4, 1), ev(5, 50, 0, 5, 1), ev(6, 60, 0, 6, 1),
	}
	got = mustScoreTopic(t, rows)
	wantScore(t, got, 40, 5, domain.TopicInProgress)
	if got.InCurrentWindow[0] {
		t.Error("lowest interview id at the same event time must be dropped first")
	}
}

func TestScoreTopicTieBreakOnRepeatedQuestion(t *testing.T) {
	// The same core answered in two interviews that completed at the same instant: higher interview id is latest.
	rows := []domain.Evidence{ev(1, 30, 0, 2, 1), ev(1, 90, 0, 1, 1)}
	wantScore(t, mustScoreTopic(t, rows), 30, 1, domain.TopicInProgress)
}

func TestScoreTopicIsOrderIndependent(t *testing.T) {
	rows := []domain.Evidence{
		ev(1, 10, 0, 1, 1), ev(2, 20, 1, 1, 2), ev(3, 90, 2, 2, 1), ev(1, 95, 3, 3, 1),
		ev(4, 85, 3, 3, 2), ev(5, 85, 3, 3, 3), ev(6, 70, 4, 4, 1), ev(7, 88, 4, 4, 2),
	}
	want := mustScoreTopic(t, rows)
	for _, perm := range [][]int{{7, 6, 5, 4, 3, 2, 1, 0}, {3, 0, 6, 2, 5, 1, 7, 4}, {4, 5, 6, 7, 0, 1, 2, 3}} {
		shuffled := make([]domain.Evidence, len(rows))
		for i, j := range perm {
			shuffled[i] = rows[j]
		}
		got := mustScoreTopic(t, shuffled)
		if got.Status != want.Status || got.EvidenceQuestionCount != want.EvidenceQuestionCount || *got.Score != *want.Score || !got.LastEvaluatedAt.Equal(*want.LastEvaluatedAt) {
			t.Fatalf("permutation %v: got %+v, want %+v", perm, got, want)
		}
		for i, j := range perm {
			if got.InCurrentWindow[i] != want.InCurrentWindow[j] {
				t.Fatalf("permutation %v: window mark of row %d differs", perm, j)
			}
		}
	}
}

func TestScoreTopicLateCatchUpDoesNotChangeCurrentScore(t *testing.T) {
	// Evidence is keyed by interview completion time, never by when it was linked. Feeding an old session
	// after a newer one (any slice order) leaves the current result for the question unchanged.
	oldSession := ev(1, 20, 0, 1, 1)
	newSession := ev(1, 90, 5, 2, 1)
	a := mustScoreTopic(t, []domain.Evidence{oldSession, newSession})
	b := mustScoreTopic(t, []domain.Evidence{newSession, oldSession})
	wantScore(t, a, 90, 1, domain.TopicInProgress)
	wantScore(t, b, 90, 1, domain.TopicInProgress)
}

func TestScoreTopicLastEvaluatedAt(t *testing.T) {
	// The latest event time of the current window (which is also the latest event time overall).
	rows := []domain.Evidence{ev(1, 60, 3, 1, 1), ev(2, 80, 9, 2, 1), ev(3, 70, 5, 1, 2)}
	got := mustScoreTopic(t, rows)
	if got.LastEvaluatedAt == nil || !got.LastEvaluatedAt.Equal(at(9)) {
		t.Fatalf("last evaluated = %v, want %v", got.LastEvaluatedAt, at(9))
	}
}

func TestScoreTopicRejectsInvalidScores(t *testing.T) {
	for _, s := range []float64{-0.01, 100.01, math.NaN(), math.Inf(1)} {
		if _, err := domain.ScoreTopic([]domain.Evidence{ev(1, s, 0, 1, 1)}); err == nil {
			t.Errorf("score %v accepted, want error", s)
		}
	}
}

func TestScoreTopicRejectsZeroEventTime(t *testing.T) {
	row := ev(1, 50, 0, 1, 1)
	row.EventTime = time.Time{}
	if _, err := domain.ScoreTopic([]domain.Evidence{row}); err == nil {
		t.Fatal("zero event time accepted, want error")
	}
}

func TestScoreTopicDoesNotMutateInput(t *testing.T) {
	rows := []domain.Evidence{ev(2, 20, 1, 1, 2), ev(1, 10, 0, 1, 1)}
	before := append([]domain.Evidence(nil), rows...)
	mustScoreTopic(t, rows)
	for i := range rows {
		if rows[i] != before[i] {
			t.Fatalf("input row %d mutated", i)
		}
	}
}

func TestScoreTopicEventTimeIsCompared(t *testing.T) {
	// Equal instants in different zones are a tie, not an ordering.
	loc := time.FixedZone("x", 7*3600)
	a := ev(1, 10, 0, 1, 1)
	b := ev(1, 90, 0, 2, 1)
	b.EventTime = a.EventTime.In(loc)
	wantScore(t, mustScoreTopic(t, []domain.Evidence{a, b}), 90, 1, domain.TopicInProgress)
}

// --- Roadmap ---

func topic(count int, achieved bool) domain.TopicScore {
	ts := domain.TopicScore{EvidenceQuestionCount: count, Status: domain.TopicNotStarted}
	switch {
	case achieved:
		ts.Status = domain.TopicAchieved
	case count > 0:
		ts.Status = domain.TopicInProgress
	}
	return ts
}

func TestScoreRoadmap(t *testing.T) {
	tests := []struct {
		name         string
		ready        bool
		topics       []domain.TopicScore
		wantStatus   domain.LearningStatus
		wantAchieved int
		wantPercent  *float64
	}{
		{"not ready, no topics", false, nil, domain.LearningNotStarted, 0, nil},
		{"ready but empty is never 100 percent", true, nil, domain.LearningNotStarted, 0, nil},
		{"ready, nothing evaluated", true, []domain.TopicScore{topic(0, false), topic(0, false)}, domain.LearningNotStarted, 0, ptr(0)},
		{"one of three achieved is in progress", true, []domain.TopicScore{topic(5, true), topic(5, false), topic(0, false)}, domain.LearningInProgress, 1, ptr(33.33)},
		{"provisional result only", true, []domain.TopicScore{topic(2, false), topic(0, false)}, domain.LearningInProgress, 0, ptr(0)},
		{"all achieved is completed", true, []domain.TopicScore{topic(5, true), topic(5, true), topic(5, true)}, domain.LearningCompleted, 3, ptr(100)},
		{"single topic achieved is completed", true, []domain.TopicScore{topic(5, true)}, domain.LearningCompleted, 1, ptr(100)},
		{"two of three", true, []domain.TopicScore{topic(5, true), topic(5, true), topic(1, false)}, domain.LearningInProgress, 2, ptr(66.67)},
		{"not ready hides progress and completion", false, []domain.TopicScore{topic(5, true)}, domain.LearningInProgress, 1, nil},
		{"not ready, no results", false, []domain.TopicScore{topic(0, false)}, domain.LearningNotStarted, 0, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.ScoreRoadmap(tc.ready, tc.topics)
			if got.Status != tc.wantStatus {
				t.Errorf("status = %s, want %s", got.Status, tc.wantStatus)
			}
			if got.AchievedItems != tc.wantAchieved || got.TotalItems != len(tc.topics) {
				t.Errorf("achieved/total = %d/%d, want %d/%d", got.AchievedItems, got.TotalItems, tc.wantAchieved, len(tc.topics))
			}
			switch {
			case tc.wantPercent == nil && got.ProgressPercent != nil:
				t.Errorf("progress = %v, want nil", *got.ProgressPercent)
			case tc.wantPercent != nil && got.ProgressPercent == nil:
				t.Errorf("progress = nil, want %v", *tc.wantPercent)
			case tc.wantPercent != nil && math.Abs(*got.ProgressPercent-*tc.wantPercent) > 1e-9:
				t.Errorf("progress = %v, want %v", *got.ProgressPercent, *tc.wantPercent)
			}
		})
	}
}

func ptr(v float64) *float64 { return &v }

func TestRoadmapExampleFromSpec(t *testing.T) {
	// Spec 4.4: SQL 82 (5 questions, achieved), Go 72 (5 questions, in progress), System design not started.
	sql := mustScoreTopic(t, []domain.Evidence{ev(1, 70, 0, 1, 1), ev(2, 80, 0, 1, 2), ev(3, 90, 1, 2, 1), ev(4, 80, 1, 2, 2), ev(5, 90, 2, 3, 1)})
	goT := mustScoreTopic(t, []domain.Evidence{ev(11, 70, 0, 1, 1), ev(12, 70, 1, 2, 1), ev(13, 70, 2, 3, 1), ev(14, 75, 3, 4, 1), ev(15, 75, 4, 5, 1)})
	sd := mustScoreTopic(t, nil)
	wantScore(t, sql, 82, 5, domain.TopicAchieved)
	wantScore(t, goT, 72, 5, domain.TopicInProgress)

	got := domain.ScoreRoadmap(true, []domain.TopicScore{sql, goT, sd})
	if got.Status != domain.LearningInProgress || got.AchievedItems != 1 || got.TotalItems != 3 {
		t.Fatalf("got %+v", got)
	}
	if got.ProgressPercent == nil || *got.ProgressPercent != 33.33 {
		t.Fatalf("progress = %v, want 33.33", got.ProgressPercent)
	}
}

func TestRoadmapExampleSessionsFromSpec(t *testing.T) {
	// Spec 4.3: INT-01 SQL Q1=60, Q2=80; Go Q3=50. INT-02 (Go focus) Q4=70, Q5=90.
	sql := mustScoreTopic(t, []domain.Evidence{ev(1, 60, 0, 1, 1), ev(2, 80, 0, 1, 2)})
	goAfterInt1 := mustScoreTopic(t, []domain.Evidence{ev(3, 50, 0, 1, 3)})
	wantScore(t, sql, 70, 2, domain.TopicInProgress)
	wantScore(t, goAfterInt1, 50, 1, domain.TopicInProgress)

	goAfterInt2 := mustScoreTopic(t, []domain.Evidence{ev(3, 50, 0, 1, 3), ev(4, 70, 5, 2, 1), ev(5, 90, 5, 2, 2)})
	wantScore(t, goAfterInt2, 70, 3, domain.TopicInProgress)
	wantScore(t, mustScoreTopic(t, []domain.Evidence{ev(1, 60, 0, 1, 1), ev(2, 80, 0, 1, 2)}), 70, 2, domain.TopicInProgress)
}

func TestRoadmapCompletedCanFallBack(t *testing.T) {
	achieved := mustScoreTopic(t, []domain.Evidence{ev(1, 80, 0, 1, 1), ev(2, 80, 0, 1, 2), ev(3, 80, 0, 1, 3), ev(4, 80, 0, 1, 4), ev(5, 80, 0, 1, 5)})
	if got := domain.ScoreRoadmap(true, []domain.TopicScore{achieved}); got.Status != domain.LearningCompleted {
		t.Fatalf("status = %s, want completed", got.Status)
	}
	dropped := mustScoreTopic(t, []domain.Evidence{ev(1, 80, 0, 1, 1), ev(2, 80, 0, 1, 2), ev(3, 80, 0, 1, 3), ev(4, 80, 0, 1, 4), ev(5, 80, 0, 1, 5), ev(5, 10, 3, 2, 1)})
	got := domain.ScoreRoadmap(true, []domain.TopicScore{dropped})
	if got.Status != domain.LearningInProgress || got.AchievedItems != 0 {
		t.Fatalf("got %+v, want in_progress with 0 achieved", got)
	}
}
