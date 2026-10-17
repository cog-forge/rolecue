package domain

import (
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// MaxVoiceQueryRunes bounds the name search; voice names are short.
const MaxVoiceQueryRunes = 100

// VoiceStatusFilter selects voices by is_active. All is the default.
type VoiceStatusFilter string

const (
	VoiceStatusAll      VoiceStatusFilter = "all"
	VoiceStatusActive   VoiceStatusFilter = "active"
	VoiceStatusInactive VoiceStatusFilter = "inactive"
)

func (s VoiceStatusFilter) Valid() bool {
	return s == VoiceStatusAll || s == VoiceStatusActive || s == VoiceStatusInactive
}

// VoiceListFilter is the Admin catalog query. Query is a case-insensitive name search.
type VoiceListFilter struct {
	Status VoiceStatusFilter
	Query  string
	Limit  int32
	Offset int32
}

// VoiceProfile is one row of the voices catalog with how often it is referenced. A referenced
// voice can only be deactivated, never removed, so past interviews and postings keep it.
type VoiceProfile struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	IsActive        bool      `json:"is_active"`
	InterviewCount  int64     `json:"interview_count"`
	JobPostingCount int64     `json:"job_posting_count"`
	InUse           bool      `json:"in_use"`
}

// VoiceDeleteOutcome tells the Admin what removing a voice actually did.
type VoiceDeleteOutcome string

const (
	VoiceDeactivated VoiceDeleteOutcome = "deactivated"
	VoiceDeleted     VoiceDeleteOutcome = "deleted"
)

// VoiceDeleteResult is the response of removing a voice. Voice is its state after the action
// (for a deleted voice, the state it had just before).
type VoiceDeleteResult struct {
	Outcome VoiceDeleteOutcome `json:"outcome"`
	Voice   VoiceProfile       `json:"voice"`
}

var ErrVoiceNotFound = apperror.New(apperror.CodeVoiceNotFound, "voice not found")
