package service

import (
	"context"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func (h *harness) retry(role string) (domain.Roadmap, error) {
	return h.svc.Retry(context.Background(), uuid.New(), role, uuid.New())
}

func (h *harness) regenerate(role string) (domain.Roadmap, error) {
	return h.svc.Regenerate(context.Background(), uuid.New(), role, uuid.New())
}

func (h *harness) noModelCalls(t *testing.T) {
	t.Helper()
	if h.topics.calls+h.bank.calls+h.store.claims != 0 {
		t.Fatalf("topics=%d bank=%d claims=%d, want no model call and no claim", h.topics.calls, h.bank.calls, h.store.claims)
	}
}

func TestRetryAndRegenerateAreForCandidatesOnly(t *testing.T) {
	h := newHarness(t, domain.StatusFailed, 1, confirmed(), valid())
	if _, err := h.retry("recruiter"); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("retry by a recruiter: %v, want FORBIDDEN", err)
	}
	if _, err := h.regenerate("admin"); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("regenerate by an admin: %v, want FORBIDDEN", err)
	}
	h.noModelCalls(t)
}

func TestRetryAndRegenerateHideAForeignProfile(t *testing.T) {
	h := newHarness(t, domain.StatusFailed, 1, confirmed(), valid())
	h.store.stateErr = apperror.New(apperror.CodeJDNotFound, "Role Profile not found.")
	if _, err := h.retry("candidate"); !apperror.IsCode(err, apperror.CodeJDNotFound) {
		t.Fatalf("retry: %v, want JD_NOT_FOUND", err)
	}
	if _, err := h.regenerate("candidate"); !apperror.IsCode(err, apperror.CodeJDNotFound) {
		t.Fatalf("regenerate: %v, want JD_NOT_FOUND", err)
	}
	h.noModelCalls(t)
}

func TestRetryAndRegenerateNeedConfirmedRequirements(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	h.store.state.Confirmed = false
	if _, err := h.retry("candidate"); !apperror.IsCode(err, apperror.CodeRequirementsNotConfirmed) {
		t.Fatalf("retry: %v, want REQUIREMENTS_NOT_CONFIRMED", err)
	}
	if _, err := h.regenerate("candidate"); !apperror.IsCode(err, apperror.CodeRequirementsNotConfirmed) {
		t.Fatalf("regenerate: %v, want REQUIREMENTS_NOT_CONFIRMED", err)
	}
	h.noModelCalls(t)
}

func TestRetryOfAFailedRoadmapGeneratesAndReturnsIt(t *testing.T) {
	h := newHarness(t, domain.StatusFailed, 1, confirmed(), valid())
	roadmap, err := h.retry("candidate")
	if err != nil || roadmap.ID != h.store.roadmap.ID {
		t.Fatalf("roadmap=%+v err=%v", roadmap, err)
	}
	if h.topics.calls != 1 || h.bank.calls != 1 || len(h.store.published) != 1 {
		t.Fatalf("topics=%d bank=%d published=%d, want one generation", h.topics.calls, h.bank.calls, len(h.store.published))
	}
	if len(h.store.allowedFrom) != 2 || h.store.allowedFrom[0] != domain.StatusPending || h.store.allowedFrom[1] != domain.StatusFailed {
		t.Fatalf("retry claims from %v, want pending and failed only", h.store.allowedFrom)
	}
}

func TestRetryOfAReadyRoadmapIsAnIdempotentNoOp(t *testing.T) {
	h := newHarness(t, domain.StatusReady, 1, confirmed(), valid())
	h.store.state.HasSessions = true
	roadmap, err := h.retry("candidate")
	if err != nil || roadmap.GenerationStatus != domain.StatusReady {
		t.Fatalf("roadmap=%+v err=%v, want the ready roadmap back untouched", roadmap, err)
	}
	h.noModelCalls(t)
}

func TestRetryOfAGeneratingRoadmapIsAConflict(t *testing.T) {
	h := newHarness(t, domain.StatusGenerating, 1, confirmed(), valid())
	h.store.claimErr = repository.ErrNotClaimable
	if _, err := h.retry("candidate"); !apperror.IsCode(err, apperror.CodeRoadmapGenerating) {
		t.Fatalf("retry: %v, want ROADMAP_GENERATING", err)
	}
	if h.topics.calls+h.bank.calls != 0 {
		t.Fatal("a conflict must not call a model")
	}
}

func TestRetryOfAProfileWithSessionsIsRefusedBeforeAnyModelCall(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	h.store.state.HasSessions = true
	if _, err := h.retry("candidate"); !apperror.IsCode(err, apperror.CodeRoadmapNotRetryable) {
		t.Fatalf("retry: %v, want ROADMAP_NOT_RETRYABLE", err)
	}
	h.noModelCalls(t)
}

func TestRegenerateIsRefusedAfterAnySession(t *testing.T) {
	h := newHarness(t, domain.StatusReady, 1, confirmed(), valid())
	h.store.state.HasSessions = true // even a finished or abandoned session counts
	if _, err := h.regenerate("candidate"); !apperror.IsCode(err, apperror.CodeRoadmapRegenerateNotAllowed) {
		t.Fatalf("regenerate: %v, want ROADMAP_REGENERATE_NOT_ALLOWED", err)
	}
	h.noModelCalls(t)
}

func TestRegenerateOfAReadyRoadmapReplacesItInOnePublish(t *testing.T) {
	h := newHarness(t, domain.StatusReady, 1, confirmed(), valid())
	if _, err := h.regenerate("candidate"); err != nil {
		t.Fatal(err)
	}
	if h.store.previous != domain.StatusReady || len(h.store.published) != 1 || h.bank.storeCalls != 1 {
		t.Fatalf("previous=%s published=%d stored=%d", h.store.previous, len(h.store.published), h.bank.storeCalls)
	}
	allowed := map[domain.GenerationStatus]bool{}
	for _, status := range h.store.allowedFrom {
		allowed[status] = true
	}
	if !allowed[domain.StatusReady] || !allowed[domain.StatusFailed] || !allowed[domain.StatusPending] {
		t.Fatalf("regenerate claims from %v, want ready, failed and pending", h.store.allowedFrom)
	}
}

func TestAFailedRegenerationPutsTheReadyRoadmapBackAndKeepsTheOldSet(t *testing.T) {
	h := newHarness(t, domain.StatusReady, 0, confirmed(), unusable())
	if _, err := h.regenerate("candidate"); !apperror.IsCode(err, apperror.CodeInvalidRoadmapGenerationOutput) {
		t.Fatalf("regenerate: %v, want INVALID_ROADMAP_GENERATION_OUTPUT", err)
	}
	if h.store.restores != 1 || h.store.fails != 0 || len(h.store.published) != 0 || h.bank.storeCalls != 0 {
		t.Fatalf("restores=%d fails=%d published=%d stored=%d: the old set must stay and the roadmap return to ready",
			h.store.restores, h.store.fails, len(h.store.published), h.bank.storeCalls)
	}
}

func TestAFailedRegenerationOfAFailedRoadmapStaysFailed(t *testing.T) {
	h := newHarness(t, domain.StatusFailed, 0, confirmed(), unusable())
	if _, err := h.regenerate("candidate"); err == nil {
		t.Fatal("want an error")
	}
	if h.store.fails != 1 || h.store.restores != 0 {
		t.Fatalf("fails=%d restores=%d, want the roadmap left failed for a retry", h.store.fails, h.store.restores)
	}
}
