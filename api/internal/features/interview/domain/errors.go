package domain

import "github.com/cog-forge/rolecue/api/pkg/apperror"

var (
	ErrInterviewNotFound      = apperror.New(apperror.CodeInterviewNotFound, "interview not found")
	ErrInterviewCompleted     = apperror.New(apperror.CodeInterviewCompleted, "interview is already completed")
	ErrTurnConflict           = apperror.New(apperror.CodeInterviewTurnConflict, "answer does not match the next question")
	ErrRoleProfileRequired    = apperror.New(apperror.CodeRoleProfileRequired, "a confirmed role profile is required")
	ErrRoleProfileNotFound    = apperror.New(apperror.CodeJDNotFound, "Role Profile not found.")
	ErrStartIntentConflict    = apperror.New(apperror.CodeInterviewStartIntentConflict, "this start request was already used for a different practice")
	ErrRoadmapNotReady        = apperror.New(apperror.CodeRoadmapNotReady, "the learning roadmap is not ready")
	ErrQuestionBankNotReady   = apperror.New(apperror.CodeQuestionBankNotReady, "the question bank is not ready")
	ErrApplicationNotFound    = apperror.New(apperror.CodeApplicationNotFound, "application not found")
	ErrApplicationNotEligible = apperror.New(apperror.CodeApplicationNotEligible,
		"application is not eligible for a recruitment interview")
	ErrCapacityNotFunded = apperror.New(apperror.CodeInterviewCapacityNotFunded, "job posting has no funded interview capacity")
	ErrCapacityFull      = apperror.New(apperror.CodeInterviewCapacityFull, "all interview slots of this job posting are taken")
)
