package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// When every model provider is congested the JD analysis reports AI_SERVICE_BUSY and releases the claim, so the
// user can analyse again; any other failure keeps EXTRACTION_FAILED (KAN-386).
func TestAnalyzeReportsProviderCongestionAsBusy(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   apperror.Code
	}{
		{llmerr.ReasonRateLimited, apperror.CodeAIServiceBusy},
		{llmerr.ReasonTimeout, apperror.CodeAIServiceBusy},
		{llmerr.ReasonHTTP5xx, apperror.CodeAIServiceBusy},
		{llmerr.ReasonNetwork, apperror.CodeAIServiceBusy},
		{"auth_rejected", apperror.CodeExtractionFailed},
		{"other", apperror.CodeExtractionFailed},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			app, repo, _ := fixture(t)
			app.extraction.extractor = extractorFunc(func(context.Context, string) (domain.ExtractionCandidate, error) {
				return domain.ExtractionCandidate{}, llmerr.NewFailure(tc.reason)
			})
			_, err := app.Analyze(context.Background(), uuid.New(), "candidate", repo.profile.ID)
			assertCode(t, err, tc.want)
			if !repo.released || repo.profile.Status != domain.Draft {
				t.Fatalf("released=%t status=%s, want the claim released", repo.released, repo.profile.Status)
			}
		})
	}
}

// A confirmation that failed because the AI service was busy leaves the profile confirmable, and the next
// confirmation succeeds.
func TestConfirmCanBeRetriedAfterTheAIServiceWasBusy(t *testing.T) {
	app, repo, bank := fixture(t)
	bank.err = apperror.New(apperror.CodeAIServiceBusy, "AI service is busy")
	_, err := app.Confirm(context.Background(), uuid.New(), "candidate", repo.profile.ID)
	assertCode(t, err, apperror.CodeAIServiceBusy)
	if repo.confirmed {
		t.Fatal("the profile was confirmed although generation failed")
	}
	bank.err = nil
	if _, err := app.Confirm(context.Background(), uuid.New(), "candidate", repo.profile.ID); err != nil || !repo.confirmed {
		t.Fatalf("second confirmation: err=%v confirmed=%t", err, repo.confirmed)
	}
}
