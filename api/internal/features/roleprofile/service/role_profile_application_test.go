package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	qb "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type profileFake struct {
	RoleProfileRepository
	profile                   domain.RoleProfile
	saved, confirmed, claimed bool
	completeErr               error
	releaseContextErr         error
	released                  bool
}

func (f *profileFake) GetProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error) {
	return f.profile, nil
}
func (f *profileFake) SaveProfile(_ context.Context, _, _ uuid.UUID, in domain.SaveRoleProfile) (domain.RoleProfile, error) {
	f.saved = true
	f.profile.RoleReview = in.RoleReview
	return f.profile, nil
}
func (f *profileFake) ConfirmProfile(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error) {
	f.confirmed = true
	f.profile.Status = domain.Confirmed
	return f.profile, nil
}
func (f *profileFake) ClaimAnalysis(context.Context, uuid.UUID, uuid.UUID) (domain.RoleProfile, error) {
	f.claimed = true
	f.profile.Status = domain.Analyzing
	return f.profile, nil
}
func (f *profileFake) ReleaseAnalysis(ctx context.Context, _ uuid.UUID) error {
	f.released = true
	f.releaseContextErr = ctx.Err()
	f.profile.Status = domain.Draft
	return nil
}
func (f *profileFake) CompleteAnalysis(_ context.Context, _, _ uuid.UUID, review domain.RoleReview) (domain.RoleProfile, error) {
	if f.completeErr != nil {
		return domain.RoleProfile{}, f.completeErr
	}
	f.profile.RoleReview = review
	f.profile.Status = domain.Review
	return f.profile, nil
}

type bankFake struct {
	err       error
	generated bool
	jd        uuid.UUID
}

func (f *bankFake) GenerateForPractice(_ context.Context, _, jd uuid.UUID) ([]qb.CoreQuestion, error) {
	f.generated = true
	f.jd = jd
	return nil, f.err
}

type extractorFunc func(context.Context, string) (domain.ExtractionCandidate, error)

func (f extractorFunc) Extract(ctx context.Context, raw string) (domain.ExtractionCandidate, error) {
	return f(ctx, raw)
}
func fixture(t *testing.T) (*RoleProfileApplication, *profileFake, *bankFake) {
	t.Helper()
	level := domain.Senior
	repo := &profileFake{profile: domain.RoleProfile{ID: uuid.New(), JDID: uuid.New(), RawText: strings.Repeat("Backend engineer using Go and PostgreSQL. ", 4), Status: domain.Review, RoleReview: domain.RoleReview{Title: "Backend engineer", SeniorityLevel: &level, Requirements: []domain.RoleRequirement{{Name: "Go", Target: "Proficient", Kind: "technology"}}}}}
	extraction, err := NewExtractionService(extractorFunc(func(context.Context, string) (domain.ExtractionCandidate, error) {
		return domain.ExtractionCandidate{Title: "Backend engineer", Skills: []domain.ExtractedSkill{}, Technologies: []string{"Go"}, DomainKnowledge: []string{}}, nil
	}), 0)
	if err != nil {
		t.Fatal(err)
	}
	bank := &bankFake{}
	app, err := NewRoleProfileApplication(extraction, repo, bank, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return app, repo, bank
}
func assertCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if code == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var e *apperror.AppError
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}
func TestConfirm(t *testing.T) {
	for _, tc := range []struct {
		name         string
		missingLevel bool
		bankErr      error
		want         apperror.Code
		confirmed    bool
	}{
		{name: "seniority required", missingLevel: true, want: apperror.CodeValidation},
		{name: "generation succeeds", confirmed: true},
		{name: "existing bank succeeds on retry", bankErr: fmt.Errorf("retry: %w", apperror.New(apperror.CodeQuestionBankExists, "exists")), confirmed: true},
		{name: "generation fails", bankErr: apperror.New(apperror.CodeQuestionBankUnavailable, "disabled"), want: apperror.CodeQuestionBankUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, repo, bank := fixture(t)
			bank.err = tc.bankErr
			if tc.missingLevel {
				repo.profile.SeniorityLevel = nil
			}
			got, err := app.Confirm(context.Background(), uuid.New(), "candidate", repo.profile.ID)
			assertCode(t, err, tc.want)
			if repo.confirmed != tc.confirmed {
				t.Fatalf("confirmed = %t", repo.confirmed)
			}
			if tc.missingLevel && bank.generated {
				t.Fatal("generated before seniority validation")
			}
			if tc.confirmed && (got.Status != domain.Confirmed || bank.jd != repo.profile.JDID) {
				t.Fatal("wrong confirmation or JD")
			}
		})
	}
}
func TestSaveEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, evidence string
		valid          bool
	}{
		{"exact quote", "using Go and PostgreSQL", true},
		{"normalized whitespace", "using  Go\nand PostgreSQL", true},
		{"blank evidence", " ", true},
		{"invented evidence", "10 years of Rust", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, repo, _ := fixture(t)
			review := repo.profile.RoleReview
			review.Requirements = append([]domain.RoleRequirement(nil), review.Requirements...)
			review.Requirements[0].EvidenceText = &tc.evidence
			_, err := app.Save(context.Background(), uuid.New(), "candidate", repo.profile.ID, domain.SaveRoleProfile{RoleReview: review})
			want := apperror.CodeValidation
			if tc.valid {
				want = ""
			}
			assertCode(t, err, want)
			if repo.saved != tc.valid {
				t.Fatalf("saved = %t", repo.saved)
			}
		})
	}
}
func TestAnalyze(t *testing.T) {
	for _, tc := range []struct {
		name     string
		modify   func(*RoleProfileApplication, *profileFake, context.CancelFunc)
		want     apperror.Code
		released bool
	}{
		{name: "success"},
		{name: "extraction failure releases claim", modify: func(a *RoleProfileApplication, _ *profileFake, _ context.CancelFunc) {
			a.extraction.extractor = extractorFunc(func(context.Context, string) (domain.ExtractionCandidate, error) {
				return domain.ExtractionCandidate{}, errors.New("provider down")
			})
		}, want: apperror.CodeExtractionFailed, released: true},
		{name: "invalid review maps validation error", modify: func(a *RoleProfileApplication, _ *profileFake, _ context.CancelFunc) {
			a.extraction.extractor = extractorFunc(func(context.Context, string) (domain.ExtractionCandidate, error) {
				return domain.ExtractionCandidate{Title: strings.Repeat("x", 201), Skills: []domain.ExtractedSkill{}, Technologies: []string{"Go"}, DomainKnowledge: []string{}}, nil
			})
		}, want: apperror.CodeInvalidExtractionOutput, released: true},
		{name: "invented evidence maps validation error", modify: func(a *RoleProfileApplication, _ *profileFake, _ context.CancelFunc) {
			a.extraction.extractor = extractorFunc(func(context.Context, string) (domain.ExtractionCandidate, error) {
				evidence := "Invented quote"
				return domain.ExtractionCandidate{Title: "Backend", Skills: []domain.ExtractedSkill{{Name: "Go", Category: domain.ProgrammingLanguage, EvidenceText: &evidence}}, Technologies: []string{}, DomainKnowledge: []string{}}, nil
			})
		}, want: apperror.CodeInvalidExtractionOutput, released: true},
		{name: "persistence failure releases claim", modify: func(_ *RoleProfileApplication, r *profileFake, _ context.CancelFunc) {
			r.completeErr = apperror.New(apperror.CodeInternal, "write failed")
		}, want: apperror.CodeInternal, released: true},
		{name: "cancelled request still releases claim", modify: func(a *RoleProfileApplication, _ *profileFake, cancel context.CancelFunc) {
			a.extraction.extractor = extractorFunc(func(context.Context, string) (domain.ExtractionCandidate, error) {
				cancel()
				return domain.ExtractionCandidate{}, context.Canceled
			})
		}, want: apperror.CodeExtractionFailed, released: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, repo, _ := fixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.modify != nil {
				tc.modify(app, repo, cancel)
			}
			got, err := app.Analyze(ctx, uuid.New(), "candidate", repo.profile.ID)
			assertCode(t, err, tc.want)
			if repo.released != tc.released || repo.releaseContextErr != nil {
				t.Fatalf("cleanup released=%t error=%v", repo.released, repo.releaseContextErr)
			}
			if tc.released && repo.profile.Status != domain.Draft {
				t.Fatal("analysis claim retained")
			}
			if tc.want == "" && got.Status != domain.Review {
				t.Fatal("analysis did not reach review")
			}
		})
	}
}

func TestCandidateAuthorization(t *testing.T) {
	for _, role := range []string{"recruiter", "admin", ""} {
		t.Run(role, func(t *testing.T) {
			app, repo, bank := fixture(t)
			id, user := uuid.New(), uuid.New()
			ctx := context.Background()
			operations := map[string]func() error{
				"create":  func() error { _, err := app.Create(ctx, user, role, ""); return err },
				"get":     func() error { _, err := app.Get(ctx, user, role, id); return err },
				"list":    func() error { _, _, err := app.List(ctx, user, role, 10, 0); return err },
				"save":    func() error { _, err := app.Save(ctx, user, role, id, domain.SaveRoleProfile{}); return err },
				"confirm": func() error { _, err := app.Confirm(ctx, user, role, id); return err },
				"analyze": func() error { _, err := app.Analyze(ctx, user, role, id); return err },
				"delete":  func() error { return app.Delete(ctx, user, role, id) },
			}
			for name, run := range operations {
				t.Run(name, func(t *testing.T) { assertCode(t, run(), apperror.CodeForbidden) })
			}
			if repo.claimed || repo.saved || repo.confirmed || bank.generated {
				t.Fatal("unauthorized side effects")
			}
		})
	}
}
func TestDocumentCandidateAuthorization(t *testing.T) {
	// Nil dependencies ensure authorization runs before storage, parsing or persistence.
	app := NewRoleProfileDocumentApplication(nil, nil, nil)
	for _, role := range []string{"recruiter", "admin", ""} {
		t.Run(role, func(t *testing.T) {
			ctx := context.Background()
			user, id := uuid.New(), uuid.New()
			_, err := app.Create(ctx, role, UploadRequest{})
			assertCode(t, err, apperror.CodeForbidden)
			_, err = app.Complete(ctx, user, role, id)
			assertCode(t, err, apperror.CodeForbidden)
			_, err = app.Document(ctx, user, role, id)
			assertCode(t, err, apperror.CodeForbidden)
		})
	}
}
