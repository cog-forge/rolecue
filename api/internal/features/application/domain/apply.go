package domain

import (
	"bytes"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// MaxCVBytes is the CV size limit: the same 5 MB as the JD PDF.
const MaxCVBytes int64 = 5 << 20

// cvSignature is the prefix every PDF starts with.
var cvSignature = []byte("%PDF-")

// codeAlreadyApplied is ALREADY_APPLIED. It is defined here until the shared apperror table gains
// the constant (plan step S0); the wire value is what clients match on.
const codeAlreadyApplied apperror.Code = "ALREADY_APPLIED"

// ErrAlreadyApplied is returned when the Candidate already has an Application to the posting.
var ErrAlreadyApplied = apperror.New(codeAlreadyApplied, "you have already applied to this job posting")

// ErrJobPostingNotFound is the answer for a posting the Candidate cannot apply to because it is not
// visible to them (not open, or its Recruiter is locked).
var ErrJobPostingNotFound = apperror.New(apperror.CodeJobPostingNotFound, "job posting not found")

// Submission is the Application a Candidate gets when the CV is accepted. The CV storage key never
// leaves the service.
type Submission struct {
	ID           uuid.UUID         `json:"id"`
	JobPostingID uuid.UUID         `json:"job_posting_id"`
	Status       ApplicationStatus `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ValidateCV checks the uploaded bytes: a non-empty PDF of at most MaxCVBytes. Every failure is a
// validation error, and nothing is stored before this passes.
func ValidateCV(data []byte) error {
	if int64(len(data)) > MaxCVBytes {
		return apperror.New(apperror.CodeValidation, "Choose a PDF up to 5 MB.")
	}
	if len(data) == 0 || !bytes.HasPrefix(data, cvSignature) {
		return apperror.New(apperror.CodeValidation, "Choose a valid PDF up to 5 MB.")
	}
	return nil
}
