package domain

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Read model of the Learning Roadmap (KAN-352, RM-11). Everything below is computed from stored rows on every
// GenerationStatus and its values come from roadmap.go, which the generation lane owns.
// read: there is no score or status cache, and nothing here is a column.

// UpdateState tells whether finished practice results are still on their way into the Roadmap (spec 8.6).
type UpdateState string

const (
	// UpdateUpToDate: every evaluated practice result of the profile is already part of the Roadmap.
	UpdateUpToDate UpdateState = "up_to_date"
	// UpdateUpdating: at least one evaluated practice result is not linked to a topic yet. The Roadmap shown is
	// correct for the results it already has; the catch-up job (or the next evaluation) links the rest.
	UpdateUpdating UpdateState = "updating"
	// UpdateDelayed: some of those results have waited longer than UpdateDelayAfter since their session ended
	// (interviews.completed_at; an evaluation that only succeeded later, after a retry, can therefore make a result
	// delayed as soon as it appears). The catch-up job normally links a result within a minute or two, so a delay means it keeps
	// failing or is switched off. The Roadmap shown is still correct for the results already linked.
	UpdateDelayed UpdateState = "delayed"
)

// UpdateDelayAfter is how long an evaluated result may wait, counted from the end of its session, for its link
// before update_state says delayed.
const UpdateDelayAfter = 10 * time.Minute

// ErrItemNotFound is returned when a topic is not part of the Roadmap.
var ErrItemNotFound = errors.New("roadmap: item not found in this roadmap")

// RoadmapRecord is the stored Roadmap with its topics, as the repository reads it for one owned Role Profile.
type RoadmapRecord struct {
	ID               uuid.UUID
	RoleProfileID    uuid.UUID
	GenerationStatus GenerationStatus
	// PracticeStarted is true when the profile has any practice session, finished or not: from then on the
	// topics can no longer be regenerated.
	PracticeStarted bool
	Items           []ItemRecord
}

// ItemRecord is one stored topic.
type ItemRecord struct {
	ID          uuid.UUID
	Title       string
	Description *string
	Position    int32
}

// ResultRow is one core question of a practice session whose core question maps to a topic of the Roadmap,
// with the stored facts the Roadmap needs. It carries no expected answer, rubric or hidden bank text.
// The repository returns every such row of the profile, whether or not it is eligible or linked yet; the
// domain applies the eligibility and scoring rules so they exist in exactly one place.
type ResultRow struct {
	ItemID         uuid.UUID
	CoreQuestionID uuid.UUID
	InterviewID    uuid.UUID
	// Position is the assignment position of the core in the interview (interview_questions.position).
	Position         int32
	InterviewType    string
	CompletedAt      *time.Time
	EvaluationStatus string
	InjectionFlagged bool
	Delivered        bool
	Answered         bool
	QuestionScore    *float64
	// QuestionText is the core turn as it was asked and shown in the practice report. Set only for an
	// answered core, so a skipped question's text is never exposed.
	QuestionText *string
	// Linked is true when a roadmap_assessments row exists for (interview, position).
	Linked bool
}

// classify applies the eligibility and per-core rules. counted is false for any row that is not evidence.
func (r ResultRow) classify() (result CoreResult, counted bool) {
	session := SessionFacts{Type: r.InterviewType, CompletedAt: r.CompletedAt, EvaluationStatus: r.EvaluationStatus, InjectionFlagged: r.InjectionFlagged}
	if !session.Eligible() {
		return CoreResult{}, false
	}
	result = ClassifyCore(CoreFacts{Position: r.Position, Delivered: r.Delivered, Answered: r.Answered, QuestionScore: r.QuestionScore})
	return result, result.Counted()
}

// RoadmapView is the response of GET /role-profiles/{id}/roadmap.
type RoadmapView struct {
	ID               uuid.UUID        `json:"id"`
	RoleProfileID    uuid.UUID        `json:"role_profile_id"`
	GenerationStatus GenerationStatus `json:"generation_status" enums:"pending,generating,ready,failed"`
	LearningStatus   LearningStatus   `json:"learning_status" enums:"not_started,in_progress,completed"`
	// ProgressPercent is 100 x completed_items / total_items with two decimals. Null unless generation_status is
	// ready and the Roadmap has at least one topic, so an unfinished or empty Roadmap never shows 100%.
	ProgressPercent *float64 `json:"progress_percent"`
	// CompletedItems is the number of achieved topics.
	CompletedItems int `json:"completed_items"`
	TotalItems     int `json:"total_items"`
	// PracticeStarted tells the UI whether the profile has a practice session. Regenerating the Roadmap is only
	// possible while it is false.
	PracticeStarted bool `json:"practice_started"`
	// UpdateState is updating while evaluated practice results are not part of the Roadmap yet, delayed when
	// some of them have waited more than 10 minutes, and PendingResultCount says how many such results
	// (answers) there are.
	UpdateState        UpdateState `json:"update_state" enums:"up_to_date,updating,delayed"`
	PendingResultCount int         `json:"pending_result_count"`
	// WindowSize and AchievedThreshold state the approved rules the topic statuses follow: a topic is achieved
	// when its five most recent distinct questions average at least 80.
	WindowSize        int               `json:"window_size"`
	AchievedThreshold float64           `json:"achieved_threshold"`
	Items             []RoadmapItemView `json:"items"`
}

// RoadmapItemView is one topic of the Roadmap with its computed progress.
type RoadmapItemView struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Position    int32     `json:"position"`
	// Score is the mean of the current window with two decimals; null until a result exists. With fewer than
	// five questions it is provisional, and never means achieved.
	Score *float64 `json:"score"`
	// EvidenceQuestionCount is the number of distinct questions in the current window, 0 to 5.
	EvidenceQuestionCount int         `json:"evidence_question_count"`
	LearningStatus        TopicStatus `json:"learning_status" enums:"not_started,in_progress,achieved"`
	// LastEvaluatedAt is when the newest result of the window was evaluated (the completion time of its
	// session); null until a result exists.
	LastEvaluatedAt *time.Time `json:"last_evaluated_at"`
}

// BuildRoadmapView computes the read model of one Roadmap from its stored topics and result rows.
//
// Only rows of a completed practice session with a successful evaluation, for a counted core (answered and
// graded, or delivered and not answered), and linked to a topic through an assessment feed the scores.
// Eligible rows that are not linked yet are not scored and are never shown as 0; they make update_state
// "updating", or "delayed" once the oldest of them ended more than UpdateDelayAfter before now. Rows of
// topics outside the Roadmap are ignored.
func BuildRoadmapView(rec RoadmapRecord, rows []ResultRow, now time.Time) (RoadmapView, error) {
	items := sortedItems(rec.Items)
	known := make(map[uuid.UUID]bool, len(items))
	for _, it := range items {
		known[it.ID] = true
	}
	evidence := make(map[uuid.UUID][]Evidence, len(items))
	pending := 0
	delayed := false
	for _, r := range rows {
		if !known[r.ItemID] {
			continue
		}
		result, counted := r.classify()
		if !counted {
			continue
		}
		if !r.Linked {
			pending++
			if now.Sub(*r.CompletedAt) > UpdateDelayAfter {
				delayed = true
			}
			continue
		}
		evidence[r.ItemID] = append(evidence[r.ItemID], r.evidence(result))
	}

	view := RoadmapView{
		ID: rec.ID, RoleProfileID: rec.RoleProfileID, GenerationStatus: rec.GenerationStatus,
		PracticeStarted: rec.PracticeStarted, PendingResultCount: pending, UpdateState: UpdateUpToDate,
		WindowSize: WindowSize, AchievedThreshold: AchievedThreshold,
		Items: make([]RoadmapItemView, 0, len(items)),
	}
	if pending > 0 {
		view.UpdateState = UpdateUpdating
		if delayed {
			view.UpdateState = UpdateDelayed
		}
	}
	topics := make([]TopicScore, 0, len(items))
	for _, it := range items {
		ts, err := ScoreTopic(evidence[it.ID])
		if err != nil {
			return RoadmapView{}, fmt.Errorf("score topic %s: %w", it.ID, err)
		}
		topics = append(topics, ts)
		view.Items = append(view.Items, RoadmapItemView{
			ID: it.ID, Title: it.Title, Description: it.Description, Position: it.Position,
			Score: ts.Score, EvidenceQuestionCount: ts.EvidenceQuestionCount,
			LearningStatus: ts.Status, LastEvaluatedAt: ts.LastEvaluatedAt,
		})
	}
	roadmap := ScoreRoadmap(rec.GenerationStatus == StatusReady, topics)
	view.LearningStatus = roadmap.Status
	view.ProgressPercent = roadmap.ProgressPercent
	view.CompletedItems = roadmap.AchievedItems
	view.TotalItems = roadmap.TotalItems
	return view, nil
}

func sortedItems(items []ItemRecord) []ItemRecord {
	out := append([]ItemRecord(nil), items...)
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Position != out[b].Position {
			return out[a].Position < out[b].Position
		}
		return out[a].ID.String() < out[b].ID.String()
	})
	return out
}

func (r ResultRow) evidence(result CoreResult) Evidence {
	return Evidence{
		CoreQuestionID: r.CoreQuestionID, InterviewID: r.InterviewID, Position: r.Position,
		Score: *result.Score, EventTime: *r.CompletedAt,
	}
}

// HistoryEntry is one evaluated practice answer of a topic. It links to the practice report through
// InterviewID (GET /interviews/{interview_id}/report) and carries no expected answer, rubric detail,
// feedback or recruitment data.
type HistoryEntry struct {
	InterviewID uuid.UUID `json:"interview_id"`
	// QuestionNumber is the position of the core question in that session, the same number the practice report
	// uses for the question.
	QuestionNumber int32 `json:"question_number"`
	// Result is scored for an answered and graded question, skipped for a question that was shown and not
	// answered (it counts as 0).
	Result CoreOutcome `json:"result" enums:"scored,skipped"`
	// Score is the question score, 0 to 100; 0 for a skipped question.
	Score *float64 `json:"score"`
	// QuestionText is the question as it was asked and as the practice report shows it. Null for a skipped
	// question, which the report does not list.
	QuestionText *string `json:"question_text"`
	// EvaluatedAt is when the session was completed and evaluated.
	EvaluatedAt time.Time `json:"evaluated_at"`
	// InCurrentWindow is true when this answer is part of the five answers the topic's score is computed from
	// now. A repeated question's older answers and answers beyond the five most recent questions are false.
	InCurrentWindow bool `json:"in_current_window"`
}

// BuildItemHistory returns every linked, evidence-grade answer of one topic, newest first (evaluation time,
// then interview id, then position, the same order the window uses), with the window flag from the one scoring
// calculation. An item that is not part of the Roadmap is ErrItemNotFound. The result is never nil.
func BuildItemHistory(rec RoadmapRecord, itemID uuid.UUID, rows []ResultRow) ([]HistoryEntry, error) {
	found := false
	for _, it := range rec.Items {
		if it.ID == itemID {
			found = true
		}
	}
	if !found {
		return nil, ErrItemNotFound
	}
	type kept struct {
		row    ResultRow
		result CoreResult
	}
	var list []kept
	var evidence []Evidence
	for _, r := range rows {
		if r.ItemID != itemID || !r.Linked {
			continue
		}
		result, counted := r.classify()
		if !counted {
			continue
		}
		list = append(list, kept{row: r, result: result})
		evidence = append(evidence, r.evidence(result))
	}
	score, err := ScoreTopic(evidence)
	if err != nil {
		return nil, fmt.Errorf("score topic %s: %w", itemID, err)
	}
	order := make([]int, len(list))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return later(evidence[order[a]], evidence[order[b]]) })
	out := make([]HistoryEntry, 0, len(list))
	for _, i := range order {
		k := list[i]
		entry := HistoryEntry{
			InterviewID: k.row.InterviewID, QuestionNumber: k.row.Position, Result: k.result.Outcome,
			Score: k.result.Score, EvaluatedAt: *k.row.CompletedAt, InCurrentWindow: score.InCurrentWindow[i],
		}
		if k.row.Answered {
			entry.QuestionText = k.row.QuestionText
		}
		out = append(out, entry)
	}
	return out, nil
}

// PageEntries returns the entries from offset, at most limit. It never returns nil.
func PageEntries(all []HistoryEntry, limit, offset int32) []HistoryEntry {
	if offset < 0 || limit <= 0 || int(offset) >= len(all) {
		return []HistoryEntry{}
	}
	end := int(offset) + int(limit)
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end]
}
