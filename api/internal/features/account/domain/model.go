// Package domain holds the account erasure (KAN-280) and data export (KAN-282) contracts.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ErasureConfirmation is the word a caller may send as the required confirmation. The account's own
// email address (any case) is accepted too.
const ErasureConfirmation = "DELETE"

// ExportLinkTTL is how long the presigned CV and recording links of an export stay valid.
const ExportLinkTTL = 15 * time.Minute

// Tombstone replaces the text of a conversation turn when its owner erases the account.
const Tombstone = "[deleted]"

// ErasureRequest is the body of DELETE /account.
type ErasureRequest struct {
	// Confirm must be "DELETE" or the account's email address.
	Confirm string `json:"confirm"`
}

// Identity is the identity row the service decides on.
type Identity struct {
	ID    uuid.UUID
	Email string
	Role  string
}

// ErasureBlockers counts what blocks an erasure. Counts only; no titles or names leave the repository.
type ErasureBlockers struct {
	OpenPostings           int64
	ApplicationsInProgress int64
}

// None reports whether nothing blocks the erasure.
func (w ErasureBlockers) None() bool { return w.OpenPostings == 0 && w.ApplicationsInProgress == 0 }

// DownloadLink is a short-lived read link. It is a bearer grant: do not log it.
type DownloadLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DataExport is the owner's data package.
type DataExport struct {
	GeneratedAt time.Time `json:"generated_at"`
	// LinkTTLSeconds is the lifetime of every download link in the package.
	LinkTTLSeconds int                 `json:"link_ttl_seconds"`
	Profile        ExportProfile       `json:"profile"`
	RoleProfiles   []ExportRoleProfile `json:"role_profiles,omitempty"`
	Applications   []ExportApplication `json:"applications,omitempty"`
	Interviews     []ExportInterview   `json:"interviews,omitempty"`
	Postings       []ExportPosting     `json:"job_postings,omitempty"`
}

// ExportProfile is the account's own profile data.
type ExportProfile struct {
	ID             uuid.UUID `json:"id"`
	Email          string    `json:"email"`
	FullName       string    `json:"full_name"`
	Role           string    `json:"role" enums:"candidate,recruiter"`
	Image          *string   `json:"image"`
	CompanyName    *string   `json:"company_name,omitempty"`
	CompanyWebsite *string   `json:"company_website,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// ExportRoleProfile is a Candidate's own Role Profile (practice target).
type ExportRoleProfile struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ExportApplication is one of the Candidate's Applications. CV is a 15-minute link to the uploaded CV.
type ExportApplication struct {
	ID                uuid.UUID     `json:"id"`
	JobPostingTitle   string        `json:"job_posting_title"`
	Status            string        `json:"status"`
	RejectReason      *string       `json:"reject_reason"`
	InterviewDeadline *time.Time    `json:"interview_deadline"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	CV                *DownloadLink `json:"cv"`
	// CVKey is the storage key the service turns into CV. It never reaches the response.
	CVKey *string `json:"-"`
}

// ExportInterview is one interview of the Candidate. Score, ScoreOutcome, Feedback, Scores and Criteria are
// filled only for practice interviews: a recruitment Candidate's result is the Recruiter's to announce, so the export
// shows what the report view shows and no more.
type ExportInterview struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type" enums:"practice,recruitment"`
	ApplicationID *uuid.UUID `json:"application_id"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	EndReason     *string    `json:"end_reason"`
	// Score is the weighted overall with two decimals; omitted when withheld (see ScoreOutcome).
	Score *float64 `json:"overall_score,omitempty"`
	// ScoreOutcome is complete, partial or withheld for a rubric-graded practice interview.
	ScoreOutcome *string           `json:"score_outcome,omitempty"`
	Feedback     *string           `json:"feedback,omitempty"`
	Turns        []ExportTurn      `json:"turns"`
	Scores       []ExportScore     `json:"scores,omitempty"`
	Criteria     []ExportCriterion `json:"criteria,omitempty"`
	Recording    *DownloadLink     `json:"recording"`
	// RecordKey is the storage key of a ready recording; the service turns it into Recording.
	RecordKey *string `json:"-"`
}

// ExportTurn is one question and the Candidate's answer.
type ExportTurn struct {
	Position   int32      `json:"position"`
	Kind       string     `json:"kind" enums:"core,follow_up"`
	Question   string     `json:"question"`
	Answer     *string    `json:"answer"`
	Feedback   *string    `json:"feedback,omitempty"`
	AskedAt    *time.Time `json:"asked_at"`
	AnsweredAt *time.Time `json:"answered_at"`
}

// ExportScore is one session metric of a practice interview. Score is null for a weighted metric no graded
// question assessed; Weight is the weight the overall used (null for an evaluation made before rubric
// grading, whose Evidence is the legacy metric quote).
type ExportScore struct {
	Metric   string   `json:"metric"`
	Score    *float64 `json:"score"`
	Weight   *float64 `json:"weight"`
	Evidence *string  `json:"evidence"`
}

// ExportCriterion is one graded rubric criterion of a practice interview (KAN-335): the points the
// Candidate's answer to one core question earned, the quote they rest on and the feedback.
type ExportCriterion struct {
	QuestionPosition int32   `json:"question_position"`
	Criterion        string  `json:"criterion"`
	Metric           string  `json:"metric"`
	Points           int32   `json:"points"`
	MaxPoints        int32   `json:"max_points"`
	Evidence         *string `json:"evidence"`
	Feedback         *string `json:"feedback"`
}

// ExportPosting is one of a Recruiter's own Job Postings.
type ExportPosting struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	Status         string     `json:"status"`
	Location       *string    `json:"location"`
	EmploymentType *string    `json:"employment_type"`
	SalaryMin      *int32     `json:"salary_min"`
	SalaryMax      *int32     `json:"salary_max"`
	CreatedAt      time.Time  `json:"created_at"`
	PublishedAt    *time.Time `json:"published_at"`
}
