package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
)

var (
	topicSQL = uid(201)
	topicGo  = uid(202)
	topicSD  = uid(203)
)

func record(status domain.GenerationStatus, started bool, items ...domain.ItemRecord) domain.RoadmapRecord {
	return domain.RoadmapRecord{ID: uid(250), RoleProfileID: uid(251), GenerationStatus: status, PracticeStarted: started, Items: items}
}

func item(id uuid.UUID, title string, position int32) domain.ItemRecord {
	return domain.ItemRecord{ID: id, Title: title, Position: position}
}

// row builds an eligible, linked, answered and graded result row of a practice session.
func row(topic uuid.UUID, core byte, score float64, h int, iv byte, pos int32) domain.ResultRow {
	s := score
	done := at(h)
	return domain.ResultRow{
		ItemID: topic, CoreQuestionID: uid(core), InterviewID: uid(iv), Position: pos,
		InterviewType: "practice", CompletedAt: &done, EvaluationStatus: "completed",
		Delivered: true, Answered: true, QuestionScore: &s, Linked: true,
	}
}

func skipped(topic uuid.UUID, core byte, h int, iv byte, pos int32) domain.ResultRow {
	r := row(topic, core, 0, h, iv, pos)
	r.Answered, r.QuestionScore = false, nil
	return r
}

// viewNow is before every session of these tests, so no pending result is delayed unless a test says so.
var viewNow = at(0).Add(-time.Hour)

func mustView(t *testing.T, rec domain.RoadmapRecord, rows []domain.ResultRow) domain.RoadmapView {
	t.Helper()
	v, err := domain.BuildRoadmapView(rec, rows, viewNow)
	if err != nil {
		t.Fatalf("BuildRoadmapView: %v", err)
	}
	return v
}

func itemByID(t *testing.T, v domain.RoadmapView, id uuid.UUID) domain.RoadmapItemView {
	t.Helper()
	for _, it := range v.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("item %s not in view", id)
	return domain.RoadmapItemView{}
}

func TestRoadmapViewStatesWithoutResults(t *testing.T) {
	tests := []struct {
		name         string
		status       domain.GenerationStatus
		items        []domain.ItemRecord
		wantProgress *float64
	}{
		{"pending has no items and no progress", domain.StatusPending, nil, nil},
		{"generating has no progress", domain.StatusGenerating, nil, nil},
		{"failed has no progress", domain.StatusFailed, nil, nil},
		{"ready but empty is never 100 percent", domain.StatusReady, nil, nil},
		{"ready with topics starts at 0 percent", domain.StatusReady, []domain.ItemRecord{item(topicSQL, "SQL", 1), item(topicGo, "Go", 2)}, ptr(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := mustView(t, record(tc.status, false, tc.items...), nil)
			if v.GenerationStatus != tc.status || v.LearningStatus != domain.LearningNotStarted {
				t.Fatalf("status = %s/%s", v.GenerationStatus, v.LearningStatus)
			}
			if (v.ProgressPercent == nil) != (tc.wantProgress == nil) || (tc.wantProgress != nil && *v.ProgressPercent != *tc.wantProgress) {
				t.Fatalf("progress = %v, want %v", v.ProgressPercent, tc.wantProgress)
			}
			if v.TotalItems != len(tc.items) || v.CompletedItems != 0 || v.UpdateState != domain.UpdateUpToDate || v.PendingResultCount != 0 {
				t.Fatalf("view = %+v", v)
			}
			if v.Items == nil {
				t.Fatal("items must be an empty array, never null")
			}
			for _, it := range v.Items {
				if it.Score != nil || it.EvidenceQuestionCount != 0 || it.LearningStatus != domain.TopicNotStarted || it.LastEvaluatedAt != nil {
					t.Fatalf("item without results = %+v", it)
				}
			}
			if v.WindowSize != 5 || v.AchievedThreshold != 80 {
				t.Fatalf("rules = %d/%v, want 5/80", v.WindowSize, v.AchievedThreshold)
			}
		})
	}
}

func TestRoadmapViewSortsItemsByPositionThenID(t *testing.T) {
	rec := record(domain.StatusReady, false,
		item(uid(9), "C", 2), item(uid(5), "B2", 1), item(uid(3), "B1", 1), item(uid(1), "A", 0))
	v := mustView(t, rec, nil)
	var got []string
	for _, it := range v.Items {
		got = append(got, it.Title)
	}
	want := []string{"A", "B1", "B2", "C"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestRoadmapViewSpecExample(t *testing.T) {
	// Spec 4.4: SQL 82 achieved, Go 72 in progress, System design not started -> 1/3, 33.33 percent.
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1), item(topicGo, "Go", 2), item(topicSD, "System design", 3))
	rows := []domain.ResultRow{
		row(topicSQL, 1, 70, 0, 1, 1), row(topicSQL, 2, 80, 0, 1, 2), row(topicSQL, 3, 90, 1, 2, 1), row(topicSQL, 4, 80, 1, 2, 2), row(topicSQL, 5, 90, 2, 3, 1),
		row(topicGo, 11, 70, 0, 4, 1), row(topicGo, 12, 70, 1, 4, 2), row(topicGo, 13, 70, 2, 5, 1), row(topicGo, 14, 75, 3, 5, 2), row(topicGo, 15, 75, 4, 6, 1),
	}
	v := mustView(t, rec, rows)
	if v.LearningStatus != domain.LearningInProgress || v.CompletedItems != 1 || v.TotalItems != 3 {
		t.Fatalf("view = %+v", v)
	}
	if v.ProgressPercent == nil || *v.ProgressPercent != 33.33 {
		t.Fatalf("progress = %v", v.ProgressPercent)
	}
	sql := itemByID(t, v, topicSQL)
	if sql.Score == nil || *sql.Score != 82 || sql.EvidenceQuestionCount != 5 || sql.LearningStatus != domain.TopicAchieved || sql.LastEvaluatedAt == nil || !sql.LastEvaluatedAt.Equal(at(2)) {
		t.Fatalf("sql = %+v", sql)
	}
	if g := itemByID(t, v, topicGo); g.Score == nil || *g.Score != 72 || g.LearningStatus != domain.TopicInProgress {
		t.Fatalf("go = %+v", g)
	}
	if sd := itemByID(t, v, topicSD); sd.Score != nil || sd.LearningStatus != domain.TopicNotStarted {
		t.Fatalf("system design = %+v", sd)
	}
	if !v.PracticeStarted {
		t.Fatal("practice_started must carry over")
	}
}

func TestRoadmapViewCompletedWhenEveryTopicAchieved(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	rows := []domain.ResultRow{row(topicSQL, 1, 80, 0, 1, 1), row(topicSQL, 2, 80, 0, 1, 2), row(topicSQL, 3, 80, 0, 1, 3), row(topicSQL, 4, 80, 0, 1, 4), row(topicSQL, 5, 80, 0, 1, 5)}
	v := mustView(t, rec, rows)
	if v.LearningStatus != domain.LearningCompleted || v.ProgressPercent == nil || *v.ProgressPercent != 100 {
		t.Fatalf("view = %+v", v)
	}
}

func TestRoadmapViewSkippedCoreCountsZero(t *testing.T) {
	// 4 answered at 100 and a delivered-but-unanswered 5th: 80 percent... 400/5 = 80 achieved. With 90 instead: 360/5 = 72.
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	rows := []domain.ResultRow{row(topicSQL, 1, 90, 0, 1, 1), row(topicSQL, 2, 90, 0, 1, 2), row(topicSQL, 3, 90, 0, 1, 3), row(topicSQL, 4, 90, 0, 1, 4), skipped(topicSQL, 5, 0, 1, 5)}
	it := itemByID(t, mustView(t, rec, rows), topicSQL)
	if it.Score == nil || *it.Score != 72 || it.EvidenceQuestionCount != 5 || it.LearningStatus != domain.TopicInProgress {
		t.Fatalf("item = %+v, want 72 over 5 questions, in progress", it)
	}
}

func TestRoadmapViewIgnoresRowsThatAreNotEvidence(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	notAsked := skipped(topicSQL, 2, 0, 1, 2)
	notAsked.Delivered = false
	ungraded := row(topicSQL, 3, 0, 0, 1, 3)
	ungraded.QuestionScore = nil
	recruitment := row(topicSQL, 4, 99, 0, 2, 1)
	recruitment.InterviewType = "recruitment"
	pending := row(topicSQL, 5, 99, 0, 3, 1)
	pending.EvaluationStatus = "pending"
	failed := row(topicSQL, 6, 99, 0, 4, 1)
	failed.EvaluationStatus = "failed"
	open := row(topicSQL, 7, 99, 0, 5, 1)
	open.CompletedAt = nil
	flagged := row(topicSQL, 8, 99, 0, 6, 1)
	flagged.InjectionFlagged = true
	rows := []domain.ResultRow{row(topicSQL, 1, 60, 0, 1, 1), notAsked, ungraded, recruitment, pending, failed, open, flagged}

	v := mustView(t, rec, rows)
	it := itemByID(t, v, topicSQL)
	if it.Score == nil || *it.Score != 60 || it.EvidenceQuestionCount != 1 {
		t.Fatalf("item = %+v, want only the answered practice question", it)
	}
	if v.UpdateState != domain.UpdateUpToDate || v.PendingResultCount != 0 {
		t.Fatalf("rows that are not evidence must not make the Roadmap look unfinished: %+v", v)
	}
}

func TestRoadmapViewUpdateStateComesFromUnlinkedResults(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	linked := row(topicSQL, 1, 60, 0, 1, 1)
	unlinked := row(topicSQL, 2, 100, 1, 2, 1)
	unlinked.Linked = false
	unlinkedSkip := skipped(topicSQL, 3, 1, 2, 2)
	unlinkedSkip.Linked = false

	v := mustView(t, rec, []domain.ResultRow{linked, unlinked, unlinkedSkip})
	if v.UpdateState != domain.UpdateUpdating || v.PendingResultCount != 2 {
		t.Fatalf("update state = %s/%d, want updating/2", v.UpdateState, v.PendingResultCount)
	}
	// An unlinked result is not scored yet and is never shown as a 0.
	it := itemByID(t, v, topicSQL)
	if it.Score == nil || *it.Score != 60 || it.EvidenceQuestionCount != 1 {
		t.Fatalf("item = %+v, want only the linked result", it)
	}

	// Nothing linked yet: the topic stays not started, with a pending update, not a 0.
	v = mustView(t, rec, []domain.ResultRow{unlinked})
	it = itemByID(t, v, topicSQL)
	if it.Score != nil || it.LearningStatus != domain.TopicNotStarted || v.UpdateState != domain.UpdateUpdating {
		t.Fatalf("view = %+v item = %+v", v, it)
	}
}

func TestRoadmapViewIgnoresRowsOfOtherTopics(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	v := mustView(t, rec, []domain.ResultRow{row(topicGo, 1, 100, 0, 1, 1)})
	if it := itemByID(t, v, topicSQL); it.Score != nil {
		t.Fatalf("a result of a topic outside this Roadmap leaked in: %+v", it)
	}
	if v.UpdateState != domain.UpdateUpToDate {
		t.Fatal("a row of a topic outside this Roadmap must not count as pending")
	}
}

func TestRoadmapViewRejectsBadScore(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	if _, err := domain.BuildRoadmapView(rec, []domain.ResultRow{row(topicSQL, 1, 150, 0, 1, 1)}, viewNow); err == nil {
		t.Fatal("score 150 accepted")
	}
}

// --- History ---

func TestItemHistoryFlagsTheCurrentWindowAndKeepsEverything(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1), item(topicGo, "Go", 2))
	// Spec 4.3: question 1 answered at 70, again at 20 later; six distinct questions overall so one more falls out.
	rows := []domain.ResultRow{
		row(topicSQL, 1, 70, 0, 1, 1),
		row(topicSQL, 2, 80, 1, 2, 1),
		row(topicSQL, 3, 90, 2, 3, 1),
		row(topicSQL, 4, 80, 3, 4, 1),
		row(topicSQL, 5, 90, 4, 5, 1),
		row(topicSQL, 6, 50, 5, 6, 1),
		row(topicSQL, 1, 20, 6, 7, 1),
		row(topicGo, 11, 100, 7, 8, 1),
	}
	h, err := domain.BuildItemHistory(rec, topicSQL, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 7 {
		t.Fatalf("history has %d rows, want all 7 of the topic", len(h))
	}
	// Newest first.
	for i := 1; i < len(h); i++ {
		if h[i].EvaluatedAt.After(h[i-1].EvaluatedAt) {
			t.Fatalf("history is not newest first at %d", i)
		}
	}
	window := map[uuid.UUID]bool{}
	for _, e := range h {
		if e.InCurrentWindow {
			window[e.InterviewID] = true
		}
	}
	// Window = latest result of each distinct question, five most recent: Q1(20 @6), Q6(@5), Q5(@4), Q4(@3), Q3(@2).
	for _, iv := range []byte{7, 6, 5, 4, 3} {
		if !window[uid(iv)] {
			t.Errorf("interview %d should be in the current window", iv)
		}
	}
	for _, iv := range []byte{1, 2} {
		if window[uid(iv)] {
			t.Errorf("interview %d must not be in the current window (superseded or too old)", iv)
		}
	}
	for _, e := range h {
		if e.Result != domain.CoreScored || e.Score == nil {
			t.Fatalf("entry = %+v", e)
		}
	}
}

func TestItemHistoryShowsSkippedAsZeroAndHidesUnlinkedAndNonEvidence(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	unlinked := row(topicSQL, 3, 99, 2, 3, 1)
	unlinked.Linked = false
	recruitment := row(topicSQL, 4, 99, 3, 4, 1)
	recruitment.InterviewType = "recruitment"
	ungraded := row(topicSQL, 5, 0, 4, 5, 1)
	ungraded.QuestionScore = nil
	rows := []domain.ResultRow{row(topicSQL, 1, 60, 0, 1, 1), skipped(topicSQL, 2, 1, 2, 3), unlinked, recruitment, ungraded}

	h, err := domain.BuildItemHistory(rec, topicSQL, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 2 {
		t.Fatalf("history = %+v, want the answered and the skipped result only", h)
	}
	skip := h[0]
	if skip.Result != domain.CoreSkipped || skip.Score == nil || *skip.Score != 0 || skip.QuestionNumber != 3 || skip.QuestionText != nil {
		t.Fatalf("skipped entry = %+v", skip)
	}
}

func TestItemHistoryUnknownItem(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	if _, err := domain.BuildItemHistory(rec, topicGo, nil); err == nil {
		t.Fatal("an item that is not in this Roadmap must be an error")
	}
}

func TestItemHistoryEmptyIsAnEmptySlice(t *testing.T) {
	rec := record(domain.StatusReady, false, item(topicSQL, "SQL", 1))
	h, err := domain.BuildItemHistory(rec, topicSQL, nil)
	if err != nil || h == nil || len(h) != 0 {
		t.Fatalf("history = %#v, err = %v, want an empty non-nil slice", h, err)
	}
}

func TestItemHistoryPaging(t *testing.T) {
	var all []domain.HistoryEntry
	for i := 0; i < 7; i++ {
		all = append(all, domain.HistoryEntry{QuestionNumber: int32(i)})
	}
	tests := []struct {
		limit, offset int32
		want          []int32
	}{
		{3, 0, []int32{0, 1, 2}},
		{3, 3, []int32{3, 4, 5}},
		{3, 6, []int32{6}},
		{3, 9, nil},
		{20, 0, []int32{0, 1, 2, 3, 4, 5, 6}},
	}
	for _, tc := range tests {
		got := domain.PageEntries(all, tc.limit, tc.offset)
		if len(got) != len(tc.want) {
			t.Fatalf("limit %d offset %d: got %d entries, want %d", tc.limit, tc.offset, len(got), len(tc.want))
		}
		for i, e := range got {
			if e.QuestionNumber != tc.want[i] {
				t.Fatalf("limit %d offset %d: entry %d = %d", tc.limit, tc.offset, i, e.QuestionNumber)
			}
		}
	}
	if got := domain.PageEntries(nil, 5, 0); got == nil || len(got) != 0 {
		t.Fatalf("empty page = %#v, want empty non-nil", got)
	}
}

func TestRoadmapViewHistoryAgreesWithScore(t *testing.T) {
	// The window flags of the history and the score of the Roadmap come from one calculation.
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	rows := []domain.ResultRow{row(topicSQL, 1, 70, 0, 1, 1), row(topicSQL, 2, 80, 1, 2, 1), row(topicSQL, 1, 20, 2, 3, 1)}
	v := mustView(t, rec, rows)
	h, err := domain.BuildItemHistory(rec, topicSQL, rows)
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	n := 0
	for _, e := range h {
		if e.InCurrentWindow {
			sum += *e.Score
			n++
		}
	}
	it := itemByID(t, v, topicSQL)
	if n != it.EvidenceQuestionCount || math.Abs(sum/float64(n)-*it.Score) > 1e-9 {
		t.Fatalf("window mean %v over %d questions vs item %+v", sum/float64(n), n, it)
	}
}

func TestRoadmapViewUpdateStateBecomesDelayedWhenAResultWaitsTooLong(t *testing.T) {
	rec := record(domain.StatusReady, true, item(topicSQL, "SQL", 1))
	unlinked := row(topicSQL, 1, 90, 0, 1, 1) // evaluated at(0)
	unlinked.Linked = false
	rows := []domain.ResultRow{unlinked}

	for name, tc := range map[string]struct {
		now  time.Time
		want domain.UpdateState
	}{
		"just evaluated":         {at(0).Add(time.Minute), domain.UpdateUpdating},
		"exactly at the limit":   {at(0).Add(domain.UpdateDelayAfter), domain.UpdateUpdating},
		"waiting past the limit": {at(0).Add(domain.UpdateDelayAfter + time.Second), domain.UpdateDelayed},
		"waiting for days":       {at(0).Add(72 * time.Hour), domain.UpdateDelayed},
	} {
		v, err := domain.BuildRoadmapView(rec, rows, tc.now)
		if err != nil {
			t.Fatal(err)
		}
		if v.UpdateState != tc.want || v.PendingResultCount != 1 {
			t.Errorf("%s: update state = %s/%d, want %s/1", name, v.UpdateState, v.PendingResultCount, tc.want)
		}
		if it := v.Items[0]; it.Score != nil {
			t.Errorf("%s: a pending result must never be scored", name)
		}
	}

	// One old pending result is enough; a fresh one next to it does not hide the delay.
	fresh := row(topicSQL, 2, 70, 9, 2, 1)
	fresh.Linked = false
	v, err := domain.BuildRoadmapView(rec, []domain.ResultRow{unlinked, fresh}, at(9).Add(time.Minute))
	if err != nil || v.UpdateState != domain.UpdateDelayed || v.PendingResultCount != 2 {
		t.Fatalf("view = %+v, err %v", v, err)
	}
}
