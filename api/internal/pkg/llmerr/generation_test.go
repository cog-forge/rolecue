package llmerr

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// A generation that fails because every model provider is congested is reported as busy; any other failure keeps
// the code the caller chose.
func TestWrap(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
		want  apperror.Code
	}{
		{"rate limited", NewFailure(ReasonRateLimited), apperror.CodeAIServiceBusy},
		{"timeout", NewFailure(ReasonTimeout), apperror.CodeAIServiceBusy},
		{"server error", NewFailure(ReasonHTTP5xx), apperror.CodeAIServiceBusy},
		{"network", NewFailure(ReasonNetwork), apperror.CodeAIServiceBusy},
		{"wrapped rate limit", errors.Join(errors.New("batch 2"), NewFailure(ReasonRateLimited)), apperror.CodeAIServiceBusy},
		{"invalid output", NewFailure(ReasonInvalidOutput), apperror.CodeQuestionGenerationFailed},
		{"auth rejected", NewFailure("auth_rejected"), apperror.CodeQuestionGenerationFailed},
		{"other", NewFailure("other"), apperror.CodeQuestionGenerationFailed},
		{"plain cancellation", context.Canceled, apperror.CodeQuestionGenerationFailed},
		{"unclassified", errors.New("boom"), apperror.CodeQuestionGenerationFailed},
		{"no cause", nil, apperror.CodeQuestionGenerationFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Wrap(apperror.CodeQuestionGenerationFailed, "Question generation failed.", tc.cause)
			if !apperror.IsCode(err, tc.want) {
				t.Fatalf("code of %v, want %s", err, tc.want)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatal("the cause was lost")
			}
		})
	}
}

func TestWrapBusyKeepsNoProviderText(t *testing.T) {
	err := Wrap(apperror.CodeExtractionFailed, "Job description extraction failed.", NewFailure(ReasonRateLimited))
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Message != "AI service is busy" {
		t.Fatalf("error = %#v", err)
	}
}
