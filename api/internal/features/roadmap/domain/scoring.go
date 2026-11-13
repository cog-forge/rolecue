// Scoring of the Learning Roadmap (KAN-346, RM-05): pure functions, no I/O. It follows the approved spec
// (Learning-Roadmaps-Spec section 4): a topic is scored from the latest result of each of its five most
// recent distinct core questions, and the Roadmap progress is the share of achieved topics.

package domain

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	// WindowSize is the number of distinct core questions a topic needs, and the most it ever counts.
	// Approved product value; not configurable.
	WindowSize = 5
	// AchievedThreshold is the minimum mean score (inclusive) of a full window. Approved product value.
	AchievedThreshold = 80

	// hundredths is the fixed-point scale of stored question scores (numeric(5,2) in Postgres).
	hundredths = 100
)

// TopicStatus is the learning status of one Roadmap topic.
type TopicStatus string

const (
	TopicNotStarted TopicStatus = "not_started"
	TopicInProgress TopicStatus = "in_progress"
	TopicAchieved   TopicStatus = "achieved"
)

// LearningStatus is the learning status of a whole Roadmap.
type LearningStatus string

const (
	LearningNotStarted LearningStatus = "not_started"
	LearningInProgress LearningStatus = "in_progress"
	LearningCompleted  LearningStatus = "completed"
)

// ErrInvalidEvidence is returned when an evidence row cannot be scored. The caller supplies stored,
// validated rows, so this signals a data bug rather than a user error.
var ErrInvalidEvidence = errors.New("roadmap: invalid evidence")

// Evidence is one evaluated answer to a core question that feeds a topic.
//
// The function assumes the caller already filtered the rows: only completed practice sessions of the profile
// owner, with a successful evaluation, for core questions mapped to this topic, and only cores that were
// delivered and answered or deliberately skipped (a skipped core arrives with Score 0). Unasked cores and
// technical failures must not be passed at all. Recruitment results must never be passed.
type Evidence struct {
	CoreQuestionID uuid.UUID
	InterviewID    uuid.UUID
	// Position is the assignment position of the core in the interview (interview_questions.position), not
	// the conversation position of a follow-up. Follow-ups never add evidence; they change the core's score.
	Position int32
	// Score is the normalized question score, 0..100 with at most two decimals (interview_questions.score).
	Score float64
	// EventTime is interviews.completed_at: when the session was completed. It is never the time an
	// assessment row was linked or a catch-up job ran.
	EventTime time.Time
}

// TopicScore is the computed state of one topic.
type TopicScore struct {
	// Score is the mean of the current window rounded half-up to two decimals; nil without evidence. It is
	// provisional while EvidenceQuestionCount < WindowSize.
	Score *float64
	// EvidenceQuestionCount is the number of distinct questions in the window, 0..WindowSize.
	EvidenceQuestionCount int
	Status                TopicStatus
	// LastEvaluatedAt is the newest event time of the window; nil without evidence.
	LastEvaluatedAt *time.Time
	// InCurrentWindow is parallel to the input slice: true for the rows that make up the current window.
	// Older answers to a repeated question and answers beyond the five most recent questions are false.
	InCurrentWindow []bool
}

// ScoreTopic computes the score, status and window of one topic from its evidence rows (any order).
//
// Per core question only the latest row counts; "latest" is the greatest EventTime, then the greater
// InterviewID (byte order), then the greater Position. The five most recent of those distinct questions form
// the window. The status is decided on the exact mean, before any rounding, so 79.99 is never achieved.
func ScoreTopic(rows []Evidence) (TopicScore, error) {
	out := TopicScore{Status: TopicNotStarted, InCurrentWindow: make([]bool, len(rows))}
	cents := make([]int64, len(rows))
	for i, r := range rows {
		c, err := toHundredths(r.Score)
		if err != nil {
			return TopicScore{}, err
		}
		if r.EventTime.IsZero() {
			return TopicScore{}, fmt.Errorf("%w: zero event time for core question %s", ErrInvalidEvidence, r.CoreQuestionID)
		}
		cents[i] = c
	}

	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	// Newest first; a total order, so the result does not depend on the input order.
	sort.SliceStable(order, func(a, b int) bool { return later(rows[order[a]], rows[order[b]]) })

	seen := make(map[uuid.UUID]struct{}, len(rows))
	var window []int
	for _, i := range order {
		if _, dup := seen[rows[i].CoreQuestionID]; dup {
			continue
		}
		seen[rows[i].CoreQuestionID] = struct{}{}
		if len(window) < WindowSize {
			window = append(window, i)
		}
	}
	if len(window) == 0 {
		return out, nil
	}

	var sum int64
	for _, i := range window {
		sum += cents[i]
		out.InCurrentWindow[i] = true
	}
	n := int64(len(window))
	score := float64(roundHalfUpDiv(sum, n)) / hundredths
	out.Score = &score
	out.EvidenceQuestionCount = len(window)
	newest := rows[window[0]].EventTime
	out.LastEvaluatedAt = &newest

	out.Status = TopicInProgress
	// sum/n >= 80 in exact integer arithmetic (both sides in hundredths).
	if len(window) == WindowSize && sum >= AchievedThreshold*hundredths*n {
		out.Status = TopicAchieved
	}
	return out, nil
}

// later reports whether a is a more recent result than b.
func later(a, b Evidence) bool {
	if !a.EventTime.Equal(b.EventTime) {
		return a.EventTime.After(b.EventTime)
	}
	if c := bytes.Compare(a.InterviewID[:], b.InterviewID[:]); c != 0 {
		return c > 0
	}
	return a.Position > b.Position
}

func toHundredths(score float64) (int64, error) {
	if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 100 {
		return 0, fmt.Errorf("%w: score %v out of range 0..100", ErrInvalidEvidence, score)
	}
	// Stored scores carry two decimals, so this is exact for every real input.
	return int64(math.Round(score * hundredths)), nil
}

// roundHalfUpDiv returns a/b rounded half-up for non-negative a and positive b.
func roundHalfUpDiv(a, b int64) int64 {
	return (2*a + b) / (2 * b)
}

// RoadmapScore is the computed progress of a whole Roadmap.
type RoadmapScore struct {
	Status        LearningStatus
	AchievedItems int
	TotalItems    int
	// ProgressPercent is 100 x achieved / total rounded half-up to two decimals. It is nil unless the content
	// is ready and has at least one topic, so an empty or unfinished Roadmap never shows 0/0 as 100%.
	ProgressPercent *float64
}

// ScoreRoadmap aggregates topic scores. ready is generation_status == "ready".
//
// not_started: no topic has any result. completed: ready, at least one topic and every topic achieved.
// Anything else with results is in_progress. All of it reflects the current results only, so a new low score
// can move an achieved topic, and a completed Roadmap, back to in_progress.
func ScoreRoadmap(ready bool, topics []TopicScore) RoadmapScore {
	out := RoadmapScore{Status: LearningNotStarted, TotalItems: len(topics)}
	hasResults := false
	for _, t := range topics {
		if t.EvidenceQuestionCount > 0 {
			hasResults = true
		}
		if t.Status == TopicAchieved {
			out.AchievedItems++
		}
	}
	if ready && out.TotalItems > 0 {
		pct := float64(roundHalfUpDiv(int64(out.AchievedItems)*100*hundredths, int64(out.TotalItems))) / hundredths
		out.ProgressPercent = &pct
	}
	switch {
	case ready && out.TotalItems > 0 && out.AchievedItems == out.TotalItems:
		out.Status = LearningCompleted
	case hasResults:
		out.Status = LearningInProgress
	}
	return out
}
