package domain

import (
	"time"

	"github.com/google/uuid"
)

// Page sizes for the Candidate's session history. The default is the common list default.
const (
	DefaultHistoryPageSize = 20
	MaxHistoryPageSize     = 100
)

// SessionStatus is the Candidate-facing state of a session. It stays in_progress until the session is
// completed; after that it is the evaluation state. The report uses the same derivation.
type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusPending    SessionStatus = "pending"
	StatusCompleted  SessionStatus = "completed"
	StatusFailed     SessionStatus = "failed"
)

// DeriveSessionStatus maps a session's completion and evaluation state to the status the Candidate sees.
// An evaluation state is only meaningful once the session is completed.
func DeriveSessionStatus(completedAt *time.Time, evaluation EvaluationStatus) SessionStatus {
	if completedAt == nil {
		return StatusInProgress
	}
	return SessionStatus(evaluation)
}

// HistoryFilter selects a page of the Candidate's sessions. An empty Type matches every session type.
type HistoryFilter struct {
	Type     Type
	Page     int32
	PageSize int32
}

// HistoryItem is one row of the Candidate's session history. It is an allow-list: the questions, the
// turns, the feedback text, the recording path and any asset link are never part of it.
type HistoryItem struct {
	ID               uuid.UUID        `json:"id"`
	Type             Type             `json:"type" enums:"practice,recruitment"`
	StartedAt        time.Time        `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
	EvaluationStatus EvaluationStatus `json:"evaluation_status" enums:"pending,completed,failed"`
	Status           SessionStatus    `json:"status" enums:"in_progress,pending,completed,failed"`
	Score            *int32           `json:"score"`
	RoleProfileTitle *string          `json:"role_profile_title"`
	JobPostingTitle  *string          `json:"job_posting_title"`
	ApplicationID    *uuid.UUID       `json:"application_id"`
}

// HistoryPage is one page of the Candidate's session history, newest first.
type HistoryPage struct {
	Items    []HistoryItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int32         `json:"page"`
	PageSize int32         `json:"page_size"`
}
