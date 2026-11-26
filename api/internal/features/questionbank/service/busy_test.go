package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// When every model provider is congested the user is told the service is busy; any other failure keeps the old
// code (KAN-386).
func TestGenerateReportsProviderCongestionAsBusy(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   apperror.Code
	}{
		{llmerr.ReasonRateLimited, apperror.CodeAIServiceBusy},
		{llmerr.ReasonTimeout, apperror.CodeAIServiceBusy},
		{llmerr.ReasonHTTP5xx, apperror.CodeAIServiceBusy},
		{llmerr.ReasonNetwork, apperror.CodeAIServiceBusy},
		{"auth_rejected", apperror.CodeQuestionGenerationFailed},
		{"other", apperror.CodeQuestionGenerationFailed},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			gen := &fakeGenerator{fn: func(int) (domain.GenerationCandidate, error) {
				return domain.GenerationCandidate{}, llmerr.NewFailure(tc.reason)
			}}
			_, err := newService(t, readyRepo(), gen, 0, 3).Generate(context.Background(), uuid.New(), "recruiter", uuid.New())
			assertCode(t, err, tc.want)
		})
	}
}
