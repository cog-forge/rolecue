package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// When every model provider is congested the topic step reports AI_SERVICE_BUSY and records the failure, so the
// generation is retryable; any other failure keeps ROADMAP_GENERATION_FAILED (KAN-386).
func TestPrepareReportsProviderCongestionAsBusy(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   apperror.Code
	}{
		{llmerr.ReasonRateLimited, apperror.CodeAIServiceBusy},
		{llmerr.ReasonTimeout, apperror.CodeAIServiceBusy},
		{llmerr.ReasonHTTP5xx, apperror.CodeAIServiceBusy},
		{llmerr.ReasonNetwork, apperror.CodeAIServiceBusy},
		{"auth_rejected", apperror.CodeRoadmapGenerationFailed},
		{"other", apperror.CodeRoadmapGenerationFailed},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			h := newHarness(t, domain.StatusPending, 0, confirmed(), func() (domain.TopicCandidate, error) {
				return domain.TopicCandidate{}, llmerr.NewFailure(tc.reason)
			})
			err := h.prepare(context.Background())
			if !apperror.IsCode(err, tc.want) {
				t.Fatalf("err = %v, want %s", err, tc.want)
			}
			if h.store.fails != 1 || len(h.store.published) != 0 {
				t.Fatalf("fails=%d published=%d, want the failure recorded and nothing published", h.store.fails, len(h.store.published))
			}
		})
	}
}
