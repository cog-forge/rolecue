package domain

import "github.com/cog-forge/rolecue/api/pkg/apperror"

var ErrNotPending = apperror.New(apperror.CodeJobPostingNotPending, "only a pending job posting can be approved or rejected")

// ErrNotSubmitted: the Recruiter is still preparing the posting and has not sent it for review.
var ErrNotSubmitted = apperror.New(apperror.CodeJobPostingNotPending, "this job posting has not been submitted for review")

var ErrQuestionBankNotReady = apperror.New(apperror.CodeQuestionBankNotReady, "a job posting needs at least one core question before it can be approved")

// ErrGradingIncomplete: approval needs every question to have an active rubric and an expected answer (KAN-331).
var ErrGradingIncomplete = apperror.New(apperror.CodeGradingConfigIncomplete, "every question needs an active rubric and an expected answer before the posting can be approved")
