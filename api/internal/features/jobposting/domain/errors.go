package domain

import "github.com/cog-forge/rolecue/api/pkg/apperror"

var (
	ErrNotEditable = apperror.New(apperror.CodeJobPostingNotEditable, "only a pending or rejected job posting can be edited")

	// ErrTextLocked: the JD text is corrected only after a rejection, when it may be what the Admin
	// objected to.
	ErrTextLocked = apperror.New(apperror.CodeJobPostingTextLocked, "the job description text can only be corrected after a rejection")

	ErrNotRejected = apperror.New(apperror.CodeJobPostingNotRejected, "only a rejected job posting can be sent for review again")

	// ErrRequirementsLocked keeps the question bank in step with what it was generated from.
	ErrRequirementsLocked = apperror.New(apperror.CodeJobPostingRequirementsLocked,
		"seniority and requirements cannot change while the posting is pending and its question bank exists; delete its questions first")

	ErrAvatarNotOwned = apperror.New(apperror.CodeValidation, "required avatar must be one of your own avatars")
	ErrVoiceNotActive = apperror.New(apperror.CodeValidation, "required voice must be an active voice profile")
	ErrSalaryRange    = apperror.New(apperror.CodeValidation, "salary_min must not be greater than salary_max")
)

// CheckSalary is the cross-field rule shared by create and the merged result of an edit.
func CheckSalary(min, max *int32) error {
	if min != nil && max != nil && *min > *max {
		return ErrSalaryRange
	}
	return nil
}
