package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Decision is the Recruiter's final decision on an interviewed Application (KAN-115).
type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReject  Decision = "reject"
)

// ExpiryRejectReason is stored on every Application the deadline rule rejects (KAN-214). Clients
// display it and never match on it.
const ExpiryRejectReason = "Interview deadline passed"

// ValidDecision reports whether value is one of the two final decisions.
func ValidDecision(value Decision) bool {
	return value == DecisionApprove || value == DecisionReject
}

// Status is the Application status a decision writes.
func (d Decision) Status() ApplicationStatus {
	if d == DecisionApprove {
		return StatusApproved
	}
	return StatusRejected
}

// InterviewSummary is the Recruiter's view of an Application's interview: the result and the
// recording state, never the transcript or the storage key.
type InterviewSummary struct {
	ID          uuid.UUID  `json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	// Status is in_progress until the session completes, then the evaluation status.
	Status       string `json:"status" enums:"in_progress,pending,completed,failed"`
	Score        *int32 `json:"score"`
	RecordStatus string `json:"record_status" enums:"none,uploading,processing,ready,failed"`
}

// CandidateInterview is the Candidate's view of the interview of one of their Applications: the
// score and the state only.
type CandidateInterview struct {
	ID          uuid.UUID  `json:"id"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `json:"status" enums:"in_progress,pending,completed,failed"`
	Score       *int32     `json:"score"`
}

// FinalReview says when the Recruiter may decide and whether they can decide now. OpensAt is the
// latest interview deadline of the posting: the Recruiter waits for every Candidate's window to close.
type FinalReview struct {
	OpensAt   *time.Time `json:"opens_at"`
	CanDecide bool       `json:"can_decide"`
}

// InterviewStatus derives the interview status the same way for both views: in_progress while the
// session is not completed, then its evaluation status.
func InterviewStatus(completedAt *time.Time, evaluationStatus string) string {
	if completedAt == nil {
		return "in_progress"
	}
	return evaluationStatus
}

// CanDecide reports whether the final decision is open: the Application is interviewed and the
// review window (the latest deadline) has opened.
func CanDecide(status ApplicationStatus, opensAt *time.Time, now time.Time) bool {
	return status == StatusInterviewed && opensAt != nil && !now.Before(*opensAt)
}

// ExpiredForRejection is the auto-reject rule (KAN-214): an interview_eligible Application whose
// deadline is strictly before now and whose interview has not completed. A session that was started
// but not completed counts as expired. The SQL in expiry.sql applies the same predicate.
func ExpiredForRejection(status ApplicationStatus, deadline *time.Time, interviewCompleted bool, now time.Time) bool {
	return status == StatusInterviewEligible && deadline != nil && deadline.Before(now) && !interviewCompleted
}

var (
	ErrApplicationNotInterviewed = apperror.New(apperror.CodeApplicationNotInterviewed,
		"application has no completed interview to decide on")
	ErrFinalReviewLocked = apperror.New(apperror.CodeFinalReviewLocked,
		"the final decision opens after every interview deadline of this job posting")
)
