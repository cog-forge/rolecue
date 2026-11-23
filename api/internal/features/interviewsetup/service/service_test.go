package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// fakeRepository serves canned facts and records what a save wrote.
type fakeRepository struct {
	practice    domain.PracticeFacts
	recruitment domain.RecruitmentFacts
	readErr     error
	saveErr     error

	saves        int
	savedPractic domain.PracticeInput
	savedEnv     *string
}

func (f *fakeRepository) Practice(context.Context, uuid.UUID, uuid.UUID) (domain.PracticeFacts, error) {
	return f.practice, f.readErr
}
func (f *fakeRepository) Recruitment(context.Context, uuid.UUID, uuid.UUID) (domain.RecruitmentFacts, error) {
	return f.recruitment, f.readErr
}
func (f *fakeRepository) SavePractice(_ context.Context, _, _ uuid.UUID, in domain.PracticeInput) error {
	f.saves++
	f.savedPractic = in
	return f.saveErr
}
func (f *fakeRepository) SaveRecruitment(_ context.Context, _ uuid.UUID, env *string) error {
	f.saves++
	f.savedEnv = env
	return f.saveErr
}

func code(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func str(s string) *string { return &s }

func confirmedPractice() domain.PracticeFacts {
	return domain.PracticeFacts{Title: "Backend", Confirmed: true, BankReady: true}
}

func eligibleRecruitment() domain.RecruitmentFacts {
	return domain.RecruitmentFacts{
		Title: "Posting", Status: domain.ApplicationInterviewEligible, WithinDeadline: true, PostingAccepting: true,
		Avatar: &domain.Choice{ID: uuid.New(), Name: "A", Usable: true}, Voice: &domain.Choice{ID: uuid.New(), Name: "V", Usable: true},
	}
}

func TestOnlyCandidatesUseSetups(t *testing.T) {
	svc := New(&fakeRepository{practice: confirmedPractice(), recruitment: eligibleRecruitment()})
	ctx, id := context.Background(), uuid.New()
	for _, role := range []string{"recruiter", "admin", ""} {
		calls := map[string]error{}
		_, calls["practice"] = svc.Practice(ctx, id, role, id)
		_, calls["save practice"] = svc.SavePractice(ctx, id, role, id, domain.PracticeInput{})
		_, calls["recruitment"] = svc.Recruitment(ctx, id, role, id)
		_, calls["save recruitment"] = svc.SaveRecruitment(ctx, id, role, id, nil)
		for name, err := range calls {
			if code(err) != apperror.CodeForbidden {
				t.Errorf("%s as %q: err = %v, want FORBIDDEN", name, role, err)
			}
		}
	}
}

func TestSavePracticeRefusalsSaveNothing(t *testing.T) {
	tests := []struct {
		name  string
		facts func() domain.PracticeFacts
		read  error
		in    domain.PracticeInput
		want  apperror.Code
	}{
		{"foreign or missing profile", confirmedPractice, domain.ErrRoleProfileNotFound, domain.PracticeInput{}, apperror.CodeJDNotFound},
		{"unconfirmed profile", func() domain.PracticeFacts { f := confirmedPractice(); f.Confirmed = false; return f }, nil,
			domain.PracticeInput{}, apperror.CodeRoleProfileRequired},
		{"no ready bank", func() domain.PracticeFacts { f := confirmedPractice(); f.BankReady = false; return f }, nil,
			domain.PracticeInput{}, apperror.CodeQuestionBankNotReady},
		{"environment outside the catalog", confirmedPractice, nil, domain.PracticeInput{Environment: str("moon")}, apperror.CodeValidation},
		{"empty environment", confirmedPractice, nil, domain.PracticeInput{Environment: str("")}, apperror.CodeValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{practice: tc.facts(), readErr: tc.read}
			_, err := New(repo).SavePractice(context.Background(), uuid.New(), "candidate", uuid.New(), tc.in)
			if code(err) != tc.want {
				t.Fatalf("err = %v, want %s", err, tc.want)
			}
			if repo.saves != 0 {
				t.Fatal("a refused save must write nothing")
			}
		})
	}
}

func TestSavePracticeStoresTheChoicesAndRevalidates(t *testing.T) {
	avatar, voice := uuid.New(), uuid.New()
	repo := &fakeRepository{practice: confirmedPractice()}
	in := domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("office-3d")}
	setup, err := New(repo).SavePractice(context.Background(), uuid.New(), "candidate", uuid.New(), in)
	if err != nil {
		t.Fatal(err)
	}
	if repo.saves != 1 || repo.savedPractic != in {
		t.Fatalf("saves = %d, saved %+v", repo.saves, repo.savedPractic)
	}
	// The fake keeps serving the unsaved facts, so the answer shows the readiness is recomputed, not echoed.
	if setup.Ready {
		t.Fatal("the answer must be the revalidated setup, not what was submitted")
	}
}

func TestSavePracticeAllowsADraft(t *testing.T) {
	repo := &fakeRepository{practice: confirmedPractice()}
	setup, err := New(repo).SavePractice(context.Background(), uuid.New(), "candidate", uuid.New(), domain.PracticeInput{})
	if err != nil || repo.saves != 1 {
		t.Fatalf("a draft with nothing chosen must save: err = %v, saves = %d", err, repo.saves)
	}
	if setup.Ready || len(setup.Blockers) != 3 {
		t.Fatalf("draft blockers = %v", setup.Blockers)
	}
}

func TestSavePracticePassesRepositoryRefusals(t *testing.T) {
	repo := &fakeRepository{practice: confirmedPractice(), saveErr: domain.ErrAvatarNotFound}
	_, err := New(repo).SavePractice(context.Background(), uuid.New(), "candidate", uuid.New(), domain.PracticeInput{AvatarID: ptr(uuid.New())})
	if code(err) != apperror.CodeAvatarNotFound {
		t.Fatalf("err = %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestSaveRecruitment(t *testing.T) {
	t.Run("stores the environment only", func(t *testing.T) {
		repo := &fakeRepository{recruitment: eligibleRecruitment()}
		setup, err := New(repo).SaveRecruitment(context.Background(), uuid.New(), "candidate", uuid.New(), str("cafe"))
		if err != nil || repo.saves != 1 || repo.savedEnv == nil || *repo.savedEnv != "cafe" {
			t.Fatalf("err = %v, saves = %d", err, repo.saves)
		}
		if len(setup.LockedFields) != 2 || setup.Mode != domain.ModeRecruitment {
			t.Fatalf("setup = %+v", setup)
		}
	})
	for name, mutate := range map[string]func(*domain.RecruitmentFacts){
		"cv not approved":  func(f *domain.RecruitmentFacts) { f.Status = "pending" },
		"deadline passed":  func(f *domain.RecruitmentFacts) { f.WithinDeadline = false },
		"already started":  func(f *domain.RecruitmentFacts) { f.Started = true },
		"posting not open": func(f *domain.RecruitmentFacts) { f.PostingAccepting = false },
	} {
		t.Run(name, func(t *testing.T) {
			facts := eligibleRecruitment()
			mutate(&facts)
			repo := &fakeRepository{recruitment: facts}
			_, err := New(repo).SaveRecruitment(context.Background(), uuid.New(), "candidate", uuid.New(), str("cafe"))
			if code(err) != apperror.CodeApplicationNotEligible || repo.saves != 0 {
				t.Fatalf("err = %v, saves = %d", err, repo.saves)
			}
		})
	}
	t.Run("foreign application", func(t *testing.T) {
		repo := &fakeRepository{readErr: domain.ErrApplicationNotFound}
		_, err := New(repo).SaveRecruitment(context.Background(), uuid.New(), "candidate", uuid.New(), nil)
		if code(err) != apperror.CodeApplicationNotFound || repo.saves != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("environment outside the catalog", func(t *testing.T) {
		repo := &fakeRepository{recruitment: eligibleRecruitment()}
		_, err := New(repo).SaveRecruitment(context.Background(), uuid.New(), "candidate", uuid.New(), str("moon"))
		if code(err) != apperror.CodeValidation || repo.saves != 0 {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestReadsRevalidateAndNeverWrite(t *testing.T) {
	stale := confirmedPractice()
	stale.Saved = true
	stale.Avatar = &domain.Choice{ID: uuid.New(), Name: "Old", Usable: false}
	stale.Voice = &domain.Choice{ID: uuid.New(), Name: "Off", Usable: false}
	stale.Environment = str("office-3d")
	repo := &fakeRepository{practice: stale, recruitment: eligibleRecruitment()}
	svc := New(repo)
	setup, err := svc.Practice(context.Background(), uuid.New(), "candidate", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if setup.Ready || len(setup.Blockers) != 2 || setup.Avatar == nil || setup.Voice == nil {
		t.Fatalf("stale choices must stay visible and block: %+v", setup)
	}
	if _, err := svc.Recruitment(context.Background(), uuid.New(), "candidate", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if repo.saves != 0 {
		t.Fatal("a read must not write")
	}
}
