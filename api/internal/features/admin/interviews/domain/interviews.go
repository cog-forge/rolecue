package domain

import (
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Interview types as stored in the interview_type enum.
const (
	TypePractice    = "practice"
	TypeRecruitment = "recruitment"
)

// Page sizes for the Admin interview list. The default matches the other Admin lists.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ErrInterviewNotFound is returned for an unknown interview ID.
var ErrInterviewNotFound = apperror.New(apperror.CodeInterviewNotFound, "interview not found")

// ListFilter selects interviews. An empty Type or nil ID filter matches every interview. Offset is
// computed by the service from the page.
type ListFilter struct {
	Type         string
	CandidateID  *uuid.UUID
	JobPostingID *uuid.UUID
	Page         int32
	PageSize     int32
}

// OwnerRef identifies the Candidate who owns an interview through its Role Profile.
type OwnerRef struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

// JobPostingRef is the Job Posting of a recruitment interview.
type JobPostingRef struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

// InterviewRow is operational metadata for one interview. It is an allow-list: recording paths,
// audio, video, transcripts, answers and feedback text are never part of it. HasRecording only says
// that a recording exists.
type InterviewRow struct {
	ID           uuid.UUID      `json:"id"`
	Type         string         `json:"type"`
	Owner        OwnerRef       `json:"owner"`
	JobPosting   *JobPostingRef `json:"job_posting"`
	Score        *int32         `json:"score"`
	HasReport    bool           `json:"has_report"`
	HasRecording bool           `json:"has_recording"`
}

// InterviewPage is one page of the Admin interview list.
type InterviewPage struct {
	Items    []InterviewRow `json:"items"`
	Total    int64          `json:"total"`
	Page     int32          `json:"page"`
	PageSize int32          `json:"page_size"`
}

// NamedRef is a referenced catalog item (avatar or voice) with its display name.
type NamedRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// RoleProfileRef is the Role Profile an interview was run against.
type RoleProfileRef struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

// InterviewDetail is the read-only operational view of one interview. It embeds the list row so the
// shared fields stay identical, and adds only identifiers, names and counts. It has no field for a
// recording path, audio, video, transcript, answer or feedback text.
type InterviewDetail struct {
	InterviewRow
	RoleProfile   RoleProfileRef `json:"role_profile"`
	ApplicationID *uuid.UUID     `json:"application_id"`
	Avatar        *NamedRef      `json:"avatar"`
	Voice         *NamedRef      `json:"voice"`
	QuestionCount int64          `json:"question_count"`
	TurnCount     int64          `json:"turn_count"`
}
