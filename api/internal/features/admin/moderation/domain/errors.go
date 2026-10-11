package domain

import "github.com/cog-forge/rolecue/api/pkg/apperror"

var ErrNotPending = apperror.New(apperror.CodeJobPostingNotPending, "only a pending job posting can be approved or rejected")
