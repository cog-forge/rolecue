//go:build integration

package tests

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	qbdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/repository"
	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixtureAI struct{}

func (*fixtureAI) Extract(context.Context, string) (domain.ExtractionCandidate, error) {
	level := domain.Senior
	return domain.ExtractionCandidate{Title: "Backend Engineer", SeniorityLevel: &level, Skills: []domain.ExtractedSkill{{Name: "Go", Category: domain.ProgrammingLanguage, Target: "Proficient", Requirement: domain.Required}}, Technologies: []string{}, DomainKnowledge: []string{}}, nil
}

type fixtureQuestionGenerator struct{}

func (fixtureQuestionGenerator) Generate(_ context.Context, _ qbdomain.GenerationInput, count int) (qbdomain.GenerationCandidate, error) {
	questions := make([]string, count)
	for i := range questions {
		questions[i] = "How do you use Go concurrency in a production service?"
	}
	return qbdomain.GenerationCandidate{Questions: questions}, nil
}

func TestRoleProfileLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("DATABASE_URL required", err)
	}
	defer pool.Close()
	user := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO users(id,name,email,role,email_verified) VALUES($1,'KAN20 fixture',$2,'candidate',true)`, user, user.String()+"@kan20.invalid"); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewRoleProfileRepository(pool)
	var profile domain.RoleProfile
	defer func() {
		if profile.ID != uuid.Nil {
			_ = repo.DeleteProfile(context.Background(), user, profile.ID)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user)
	}()
	extraction, _ := service.NewExtractionService(&fixtureAI{}, 0)
	questionBank, err := qbservice.New(qbrepository.NewRepository(pool), fixtureQuestionGenerator{}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	app, err := service.NewRoleProfileApplication(extraction, repo, questionBank, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.Repeat("Senior backend engineer using Go, concurrency, caching and PostgreSQL. ", 4)
	profile, err = app.Create(ctx, user, raw)
	if err != nil || profile.Status != domain.Draft {
		t.Fatal("draft create", err)
	}
	pdfKey := uuid.NewString()
	pdfProfile, err := repo.CreatePDFProfile(ctx, user, raw, pdfKey)
	if err != nil || !pdfProfile.HasOriginalDocument || pdfProfile.ContentPath != pdfKey {
		t.Fatal("PDF path persistence", err)
	}
	defer func() { _ = repo.DeleteProfile(context.Background(), user, pdfProfile.ID) }()
	profile, err = app.Analyze(ctx, user, profile.ID)
	if err != nil || profile.Status != domain.Review || len(profile.Requirements) != 1 {
		t.Fatal("analysis", err)
	}
	review := profile.RoleReview
	review.RefinementNote = "Focus on caching"
	profile, err = app.Save(ctx, user, profile.ID, domain.SaveRoleProfile{RoleReview: review})
	if err != nil {
		t.Fatal("save review", err)
	}
	profile, err = app.Confirm(ctx, user, profile.ID)
	if err != nil || profile.Status != domain.Confirmed {
		t.Fatal("confirm", err)
	}
	input, err := qbrepository.NewRepository(pool).GenerationInput(ctx, profile.JDID)
	if err != nil || input.RefinementNote != review.RefinementNote || len(input.Requirements) != 1 {
		t.Fatal("saved review not readable", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID).Scan(&count); err != nil || count != 1 {
		t.Fatal("confirmed profile question bank was not generated", err)
	}
}
