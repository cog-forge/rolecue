package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func topUpHarness(t *testing.T) (*harness, uuid.UUID) {
	t.Helper()
	h := newHarness(t, domain.StatusReady, 1, confirmed(), valid())
	topic := uuid.New()
	h.store.item = domain.Item{ID: topic, Title: "Go concurrency", Description: "Goroutines"}
	return h, topic
}

func TestTopUpDraftsTheMissingQuestionsOfTheTopicThenAddsThemUnderTheLock(t *testing.T) {
	h, topic := topUpHarness(t)
	if err := h.svc.TopUp(context.Background(), uuid.New(), uuid.New(), topic, 5, 8); err != nil {
		t.Fatal(err)
	}
	if h.bank.topUpCalls != 1 || h.bank.topUpCount != 3 || h.bank.topUpTopic.ID != topic || h.bank.topUpTopic.Title != "Go concurrency" {
		t.Fatalf("calls=%d count=%d topic=%+v, want one call for the 3 missing questions of this topic", h.bank.topUpCalls, h.bank.topUpCount, h.bank.topUpTopic)
	}
	if h.store.locks != 1 || h.bank.appendCalls != 1 || h.bank.appendedTotal != 8 || h.bank.appendedTopic != topic {
		t.Fatalf("locks=%d appends=%d total=%d: the add must run once under the practice lock", h.store.locks, h.bank.appendCalls, h.bank.appendedTotal)
	}
	if h.topics.calls != 0 || h.store.claims != 0 {
		t.Fatal("a top-up must not regenerate topics or touch the roadmap status")
	}
}

func TestTopUpOfAnAlreadyFullTopicCallsNothing(t *testing.T) {
	h, topic := topUpHarness(t)
	if err := h.svc.TopUp(context.Background(), uuid.New(), uuid.New(), topic, 8, 8); err != nil {
		t.Fatal(err)
	}
	if h.bank.topUpCalls+h.bank.appendCalls != 0 {
		t.Fatal("no model call and no write when the topic has enough")
	}
}

func TestTopUpFailureStoresNothing(t *testing.T) {
	h, topic := topUpHarness(t)
	h.bank.topUpErr = apperror.New(apperror.CodeQuestionGenerationFailed, "failed")
	err := h.svc.TopUp(context.Background(), uuid.New(), uuid.New(), topic, 5, 8)
	if !apperror.IsCode(err, apperror.CodeQuestionGenerationFailed) || h.bank.appendCalls != 0 || h.store.locks != 0 {
		t.Fatalf("err=%v appends=%d locks=%d, want the model error and nothing stored", err, h.bank.appendCalls, h.store.locks)
	}
}

func TestTopUpNeedsAConfirmedProfileAndATopicOfIt(t *testing.T) {
	h, topic := topUpHarness(t)
	h.store.state.Confirmed = false
	if err := h.svc.TopUp(context.Background(), uuid.New(), uuid.New(), topic, 5, 8); !apperror.IsCode(err, apperror.CodeRequirementsNotConfirmed) {
		t.Fatalf("unconfirmed: %v", err)
	}
	h.store.state.Confirmed = true
	h.store.itemErr = errors.New("not found")
	if err := h.svc.TopUp(context.Background(), uuid.New(), uuid.New(), topic, 5, 8); err == nil {
		t.Fatal("a topic that is not the profile's was accepted")
	}
	if h.bank.topUpCalls != 0 {
		t.Fatal("no model call for an invalid request")
	}
}
