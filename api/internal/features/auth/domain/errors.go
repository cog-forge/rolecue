package domain

import "github.com/cog-forge/rolecue/api/pkg/apperror"

var ErrInvalidSession = apperror.New(apperror.CodeInvalidToken, "a valid session is required")
var ErrUnavailable = apperror.New(apperror.CodeAuthUnavailable, "authentication is temporarily unavailable")
