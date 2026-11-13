package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeFocus struct {
	state domain.FocusState
	err   error
	calls int
}

func (f *fakeFocus) FocusState(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.FocusState, error) {
	f.calls++
	return f.state, f.err
}

type fakeTopUp struct {
	calls       int
	have, total int
	topic       uuid.UUID
	err         error
}

func (f *fakeTopUp) TopUp(_ context.Context, _, _, topic uuid.UUID, have, total int) error {
	f.calls++
	f.topic, f.have, f.total = topic, have, total
	return f.err
}

func focusedInput(profile, topic uuid.UUID) domain.CreateInput {
	return domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile, FocusItemID: &topic}
}

func focusedService(t *testing.T, state domain.FocusState) (*Service, *fakeRepository, *fakeFocus, *fakeTopUp, *stubBudget) {
	t.Helper()
	repo := &fakeRepository{createCreated: true, createSession: domain.Session{ID: uuid.New(), Type: domain.TypePractice}}
	svc := newTestService(t, repo, Settings{QuestionLimit: 8})
	focus, topUp, budget := &fakeFocus{state: state}, &fakeTopUp{}, &stubBudget{left: 10}
	svc.SetTopicTopUp(focus, topUp)
	svc.SetLLMBudget(budget)
	return svc, repo, focus, topUp, budget
}

func TestFocusedStartPassesTheFocusToTheRepositoryAndSaysItWasCreated(t *testing.T) {
	svc, repo, _, topUp, _ := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 10})
	profile, topic := uuid.New(), uuid.New()
	session, created, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(profile, topic))
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if repo.createParams.FocusItemID == nil || *repo.createParams.FocusItemID != topic || repo.createParams.PracticeCoins != domain.PracticeInterviewCoins {
		t.Fatalf("params = %+v, want the focus and the usual price", repo.createParams)
	}
	if session.Resumed == nil || *session.Resumed {
		t.Fatalf("resumed = %v, want false for a new session", session.Resumed)
	}
	if topUp.calls != 0 {
		t.Fatal("a topic with enough questions must not call the model")
	}
}

func TestFocusedStartOfATopicWithTooFewQuestionsTopsItUpOnceBeforeTheStart(t *testing.T) {
	svc, repo, _, topUp, budget := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 5})
	topic := uuid.New()
	if _, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), topic)); err != nil {
		t.Fatal(err)
	}
	if topUp.calls != 1 || topUp.have != 5 || topUp.total != 8 || topUp.topic != topic {
		t.Fatalf("top-up calls=%d have=%d total=%d, want one call raising 5 to 8", topUp.calls, topUp.have, topUp.total)
	}
	if repo.createCalls != 1 {
		t.Fatal("the start must follow the top-up")
	}
	if budget.calls != 2 {
		t.Fatalf("budget units = %d, want one for the session and one for the top-up call", budget.calls)
	}
}

func TestAFailedTopUpMeansNoSessionNoDebitAndTheBudgetIsGivenBack(t *testing.T) {
	svc, repo, _, topUp, budget := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 5})
	topUp.err = apperror.New(apperror.CodeQuestionGenerationFailed, "failed")
	_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New()))
	if !apperror.IsCode(err, apperror.CodeQuestionGenerationFailed) {
		t.Fatalf("err = %v, want the generation error", err)
	}
	if repo.createCalls != 0 {
		t.Fatal("a session was created although the top-up failed: that is where a coin could be debited")
	}
	if budget.released != 2 || budget.left != 10 {
		t.Fatalf("released=%d left=%d, want both units given back", budget.released, budget.left)
	}
}

func TestAForeignTopicIsRefusedEvenWhileASessionIsOpen(t *testing.T) {
	svc, repo, focus, topUp, _ := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 10})
	focus.err = apperror.New(apperror.CodeValidation, "focus_item_id is not a topic of this role profile")
	repo.existingFound = true
	repo.existingSession = domain.Session{ID: uuid.New(), Type: domain.TypePractice}
	if _, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New())); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("err = %v, want VALIDATION_ERROR", err)
	}
	if topUp.calls != 0 || repo.createCalls != 0 {
		t.Fatal("nothing may run for an invalid focus")
	}
}

func TestAResumedFocusedStartCallsNoModelAndSaysResumed(t *testing.T) {
	svc, repo, focus, topUp, _ := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 2})
	existing := uuid.New()
	repo.existingFound = true
	repo.existingSession = domain.Session{ID: existing, Type: domain.TypePractice}
	repo.createCreated = false
	repo.createSession = repo.existingSession
	session, created, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New()))
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if topUp.calls != 0 {
		t.Fatal("resuming a session needs no model call")
	}
	_ = focus
	if session.Resumed == nil || !*session.Resumed || session.ID != existing {
		t.Fatalf("session = %+v, want the existing one marked resumed", session)
	}
}

func TestASameKeyDifferentFocusStartIsAConflictBeforeAnyWork(t *testing.T) {
	svc, repo, _, topUp, budget := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 2})
	repo.existingErr = domain.ErrStartIntentConflict
	_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New()))
	if !apperror.IsCode(err, apperror.CodeInterviewStartIntentConflict) {
		t.Fatalf("err = %v, want INTERVIEW_START_INTENT_CONFLICT", err)
	}
	if topUp.calls+repo.createCalls != 0 || budget.released != 1 {
		t.Fatal("a conflict must do no work and give the session's budget unit back")
	}
}

func TestFocusedStartRefusesARoadmapThatIsNotReadyAndAForeignTopic(t *testing.T) {
	svc, repo, focus, topUp, _ := focusedService(t, domain.FocusState{RoadmapReady: false})
	if _, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New())); !apperror.IsCode(err, apperror.CodeRoadmapNotReady) {
		t.Fatalf("err = %v, want ROADMAP_NOT_READY", err)
	}
	focus.err = errors.New("lookup failed")
	if _, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New())); err == nil {
		t.Fatal("a topic that does not belong to the profile was accepted")
	}
	if topUp.calls != 0 || repo.createCalls != 0 {
		t.Fatal("nothing may be generated or created for an invalid focus")
	}
}

func TestRecruitmentNeverAcceptsAFocus(t *testing.T) {
	svc, repo, _, _, _ := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 10})
	application, topic := uuid.New(), uuid.New()
	_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &application, FocusItemID: &topic})
	if !apperror.IsCode(err, apperror.CodeValidation) || repo.createCalls != 0 {
		t.Fatalf("err = %v creates = %d, want a validation error before the repository", err, repo.createCalls)
	}
}

func TestGeneralStartIsUntouchedByTheFocusMachinery(t *testing.T) {
	svc, repo, focus, topUp, budget := focusedService(t, domain.FocusState{})
	profile := uuid.New()
	if _, _, err := svc.Create(context.Background(), uuid.New(), "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile}); err != nil {
		t.Fatal(err)
	}
	if repo.createParams.FocusItemID != nil || focus.calls+topUp.calls != 0 || budget.calls != 1 {
		t.Fatalf("focus=%v checks=%d topups=%d budget=%d, want a plain start", repo.createParams.FocusItemID, focus.calls, topUp.calls, budget.calls)
	}
}
