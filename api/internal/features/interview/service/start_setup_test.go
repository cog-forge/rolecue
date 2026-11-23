package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	setupdomain "github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeStartSetup struct {
	setup setupdomain.Setup
	err   error
	calls int
}

func (f *fakeStartSetup) Practice(context.Context, uuid.UUID, string, uuid.UUID) (setupdomain.Setup, error) {
	f.calls++
	return f.setup, f.err
}

func (f *fakeStartSetup) Recruitment(context.Context, uuid.UUID, string, uuid.UUID) (setupdomain.Setup, error) {
	f.calls++
	return f.setup, f.err
}

func readyPractice() setupdomain.Setup {
	return setupdomain.Setup{
		Mode:   setupdomain.ModePractice,
		Avatar: &setupdomain.Choice{ID: uuid.New(), Usable: true}, Voice: &setupdomain.Choice{ID: uuid.New(), Usable: true},
	}.WithBlockers(nil)
}

func setupService(t *testing.T, setup setupdomain.Setup) (*Service, *fakeRepository, *fakeStartSetup, *stubBudget) {
	t.Helper()
	repo := &fakeRepository{createCreated: true, createSession: domain.Session{ID: uuid.New(), Type: domain.TypePractice}}
	svc := newTestService(t, repo, Settings{QuestionLimit: 8})
	checker, budget := &fakeStartSetup{setup: setup}, &stubBudget{left: 10}
	svc.SetStartSetup(checker)
	svc.SetLLMBudget(budget)
	return svc, repo, checker, budget
}

func practiceInput() domain.CreateInput {
	profile := uuid.New()
	return domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile}
}

func recruitmentInput() domain.CreateInput {
	application := uuid.New()
	return domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &application}
}

func notReady(blockers ...setupdomain.Blocker) setupdomain.Setup {
	return setupdomain.Setup{Mode: setupdomain.ModePractice}.WithBlockers(blockers)
}

func TestNotReadyPracticeStartIsRefusedWithTheBlockersAndNothingIsTaken(t *testing.T) {
	setup := notReady(
		setupdomain.Blocker{Code: setupdomain.BlockerAvatarRequired, Field: "avatar"},
		setupdomain.Blocker{Code: setupdomain.BlockerEnvironmentRequired, Field: "environment"})
	svc, repo, _, budget := setupService(t, setup)
	_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", practiceInput())
	if !apperror.IsCode(err, apperror.CodeSetupNotReady) {
		t.Fatalf("err = %v, want SETUP_NOT_READY", err)
	}
	var appErr *apperror.AppError
	if !asAppError(err, &appErr) {
		t.Fatal("not an AppError")
	}
	details, ok := appErr.Details.(SetupNotReadyDetails)
	if !ok || len(details.Blockers) != 2 || details.Blockers[0] != (SetupBlocker{Code: "avatar_required", Field: "avatar"}) ||
		details.Blockers[1] != (SetupBlocker{Code: "environment_required", Field: "environment"}) {
		t.Fatalf("details = %#v", appErr.Details)
	}
	if repo.createCalls != 0 {
		t.Fatal("a session was created, which is where a coin is debited")
	}
	if budget.left != 10 {
		t.Fatalf("budget left = %d: the reservation of a refused start must be given back", budget.left)
	}
}

func TestNotReadyRecruitmentStartIsRefused(t *testing.T) {
	svc, repo, _, _ := setupService(t, notReady(setupdomain.Blocker{Code: setupdomain.BlockerEnvironmentRequired, Field: "environment"}))
	_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", recruitmentInput())
	if !apperror.IsCode(err, apperror.CodeSetupNotReady) || repo.createCalls != 0 {
		t.Fatalf("err = %v creates = %d", err, repo.createCalls)
	}
}

func TestStructuralBlockersKeepTheirOwnCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		blocker setupdomain.BlockerCode
		input   domain.CreateInput
		want    apperror.Code
	}{
		"unconfirmed role profile": {setupdomain.BlockerRoleProfileNotConfirmed, practiceInput(), apperror.CodeRoleProfileRequired},
		"bank not ready":           {setupdomain.BlockerQuestionBankNotReady, practiceInput(), apperror.CodeQuestionBankNotReady},
		"application not eligible": {setupdomain.BlockerApplicationNotEligible, recruitmentInput(), apperror.CodeApplicationNotEligible},
		"deadline passed":          {setupdomain.BlockerDeadlinePassed, recruitmentInput(), apperror.CodeApplicationNotEligible},
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo, _, _ := setupService(t, notReady(
				setupdomain.Blocker{Code: setupdomain.BlockerEnvironmentRequired, Field: "environment"},
				setupdomain.Blocker{Code: tc.blocker}))
			_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", tc.input)
			if !apperror.IsCode(err, tc.want) || repo.createCalls != 0 {
				t.Fatalf("err = %v creates = %d, want %s", err, repo.createCalls, tc.want)
			}
		})
	}
}

func TestReadyPracticeStartUsesTheSavedAvatarAndVoice(t *testing.T) {
	setup := readyPractice()
	svc, repo, _, _ := setupService(t, setup)
	_, created, err := svc.Create(context.Background(), uuid.New(), "candidate", practiceInput())
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if repo.createParams.AvatarID == nil || *repo.createParams.AvatarID != setup.Avatar.ID ||
		repo.createParams.VoiceID == nil || *repo.createParams.VoiceID != setup.Voice.ID {
		t.Fatalf("params = %+v, want the saved avatar and voice", repo.createParams)
	}
}

func TestPracticeBodyMayRepeatTheSavedChoicesButNotChangeThem(t *testing.T) {
	setup := readyPractice()
	other := uuid.New()
	for name, tc := range map[string]struct {
		avatar, voice *uuid.UUID
		ok            bool
	}{
		"same":                   {&setup.Avatar.ID, &setup.Voice.ID, true},
		"only avatar":            {&setup.Avatar.ID, nil, true},
		"another avatar":         {&other, nil, false},
		"another voice":          {nil, &other, false},
		"avatar and wrong voice": {&setup.Avatar.ID, &other, false},
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo, _, budget := setupService(t, setup)
			input := practiceInput()
			input.AvatarID, input.VoiceID = tc.avatar, tc.voice
			_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", input)
			if tc.ok {
				if err != nil || repo.createCalls != 1 {
					t.Fatalf("err = %v creates = %d", err, repo.createCalls)
				}
				return
			}
			if !apperror.IsCode(err, apperror.CodeValidation) || repo.createCalls != 0 || budget.left != 10 {
				t.Fatalf("err = %v creates = %d budget = %d", err, repo.createCalls, budget.left)
			}
		})
	}
}

func TestStartsThatOnlyReturnASessionAreNeverCheckedAgainstTheSetup(t *testing.T) {
	// A setup that is no longer ready (or never saved) must not block the open session, a retry or the
	// Application's session.
	for name, input := range map[string]domain.CreateInput{"practice": practiceInput(), "recruitment": recruitmentInput()} {
		t.Run(name, func(t *testing.T) {
			svc, repo, checker, _ := setupService(t, notReady(setupdomain.Blocker{Code: setupdomain.BlockerInterviewStarted}))
			existing := domain.Session{ID: uuid.New(), Type: input.Type}
			repo.existingFound, repo.existingSession = true, existing
			repo.createCreated, repo.createSession = false, existing
			session, created, err := svc.Create(context.Background(), uuid.New(), "candidate", input)
			if err != nil || created || session.ID != existing.ID || session.Resumed == nil || !*session.Resumed {
				t.Fatalf("session=%+v created=%v err=%v", session, created, err)
			}
			if checker.calls != 0 {
				t.Fatal("the setup was checked for a start that creates nothing")
			}
		})
	}
}

func TestAFocusedStartIsCheckedAgainstTheSetupBeforeAnyTopUp(t *testing.T) {
	svc, repo, focus, topUp, _ := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 2})
	checker := &fakeStartSetup{setup: notReady(setupdomain.Blocker{Code: setupdomain.BlockerVoiceRequired, Field: "voice"})}
	svc.SetStartSetup(checker)
	_, _, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New()))
	if !apperror.IsCode(err, apperror.CodeSetupNotReady) {
		t.Fatalf("err = %v, want SETUP_NOT_READY", err)
	}
	if topUp.calls != 0 || repo.createCalls != 0 || focus.calls != 1 {
		t.Fatalf("topUp=%d creates=%d focus=%d: a refused start must not spend a model call", topUp.calls, repo.createCalls, focus.calls)
	}
}

func TestAReadyFocusedStartStillTopsUpAndStarts(t *testing.T) {
	svc, repo, _, topUp, _ := focusedService(t, domain.FocusState{RoadmapReady: true, Questions: 2})
	setup := readyPractice()
	svc.SetStartSetup(&fakeStartSetup{setup: setup})
	_, created, err := svc.Create(context.Background(), uuid.New(), "candidate", focusedInput(uuid.New(), uuid.New()))
	if err != nil || !created || topUp.calls != 1 || repo.createCalls != 1 || *repo.createParams.AvatarID != setup.Avatar.ID {
		t.Fatalf("created=%v err=%v topUp=%d creates=%d", created, err, topUp.calls, repo.createCalls)
	}
}

func TestWithoutASetupCheckTheRequestChoicesAreUsedAsBefore(t *testing.T) {
	repo := &fakeRepository{createCreated: true, createSession: domain.Session{ID: uuid.New(), Type: domain.TypePractice}}
	svc := newTestService(t, repo, Settings{QuestionLimit: 8})
	avatar := uuid.New()
	input := practiceInput()
	input.AvatarID = &avatar
	if _, _, err := svc.Create(context.Background(), uuid.New(), "candidate", input); err != nil || *repo.createParams.AvatarID != avatar {
		t.Fatalf("err = %v params = %+v", err, repo.createParams)
	}
}

func asAppError(err error, target **apperror.AppError) bool { return errors.As(err, target) }
