//go:build ai_live && integration

package tests

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	er "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	es "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"testing"
	"time"
)

// TestRoutingPracticeClone exercises real Groq/Gemini, the service loop and PostgreSQL. It requires
// the disposable KAN-273 clone, creates isolated data and cleans it up using the integration fixture.
func TestRoutingPracticeClone(t *testing.T) {
	ctx, pool := openPool(t)
	if pool.Config().ConnConfig.Port != 55473 {
		t.Fatal("live practice requires KAN-273 clone port 55473")
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal("invalid config")
	}
	core, logs := observer.New(zapcore.DebugLevel)
	ctx = logger.ToContext(ctx, &logger.Logger{Logger: zap.New(core)})
	zero := 0
	r := config.TaskRoute{Provider: "groq", Fallback: "gemini", Timeout: 45 * time.Second, MaxRetries: &zero}
	cfg.AI.JD = r
	cfg.AI.Bank = r
	cfg.AI.FollowUp = r
	cfg.AI.Evaluation = r
	makeAdapter := func(task config.Task) *ai.TaskAdapter {
		a, e := ai.NewTaskAdapter(ctx, cfg, task)
		if e != nil {
			t.Fatal("adapter unavailable")
		}
		return a
	}
	extracted, err := makeAdapter(config.TaskJD).Extract(ctx, `Senior Backend Engineer. Required: Go, PostgreSQL, API design and automated testing. Docker is preferred. Build reliable payment services.`)
	if err != nil {
		t.Fatal("JD extraction failed")
	}
	in := qd.GenerationInput{Seniority: qd.SeniorityLevel{Name: "Senior"}}
	for _, skill := range extracted.Skills {
		in.Requirements = append(in.Requirements, qd.ConfirmedRequirement{Name: skill.Name, Target: skill.Target, Type: "required"})
	}
	bank, err := makeAdapter(config.TaskBank).Generate(ctx, in, 50)
	if err != nil {
		t.Fatal("bank generation failed")
	}
	seed := newSeed(t, ctx, pool)
	candidate := seed.user("candidate")
	seed.wallet(candidate, 10)
	profile, _ := seed.confirmedProfile(candidate, 0)
	var jd uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT jd_id FROM role_profiles WHERE id=$1`, profile).Scan(&jd); err != nil {
		t.Fatal("profile unavailable")
	}
	for _, q := range bank.Questions {
		seed.exec(`INSERT INTO core_questions (jd_id,content) VALUES ($1,$2)`, jd, q)
	}
	seed.requirement(profile, "Go")
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 8, FollowUpLimit: 1, FollowUpTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal("service unavailable")
	}
	svc.SetFollowUpDecider(makeAdapter(config.TaskFollowUp))
	session, _, err := svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile})
	if err != nil {
		t.Fatal("practice start failed")
	}
	followups := 0
	for session.CurrentQuestion != nil {
		session, err = svc.AppendTurn(ctx, candidate, "candidate", session.ID, session.CurrentQuestion.Position, "I would use the database and add tests.")
		if err != nil {
			t.Fatal("core answer failed")
		}
		delivery, e := svc.FollowUp(ctx, candidate, "candidate", session.ID)
		if e != nil {
			t.Fatal("follow-up failed")
		}
		if delivery.Question != nil {
			followups++
			session, err = svc.AppendFollowUp(ctx, candidate, "candidate", session.ID, *delivery.Question, "I would test concurrent transactions and verify atomic writes.")
			if err != nil {
				t.Fatal("follow-up answer failed")
			}
		}
	}
	_, err = svc.Complete(ctx, candidate, "candidate", session.ID)
	if err != nil {
		t.Fatal("complete failed")
	}
	scoring, err := es.NewScoring(er.NewRepository(pool), makeAdapter(config.TaskEvaluation))
	if err != nil {
		t.Fatal("scoring unavailable")
	}
	if err = scoring.Evaluate(ctx, session.ID); err != nil {
		t.Fatal("evaluation failed")
	}
	stored, err := svc.Get(ctx, candidate, "candidate", session.ID)
	if err != nil || stored.EvaluationStatus != domain.EvaluationCompleted {
		t.Fatal("evaluation not persisted")
	}
	totals := map[string]int64{}
	byProvider := map[string]int64{}
	fallbacks := 0
	for _, entry := range logs.All() {
		f := entry.ContextMap()
		if n, ok := f["total_tokens"].(int64); ok {
			totals[f["task"].(string)] += n
			byProvider[f["provider"].(string)+"/"+f["task"].(string)] += n
		}
		if f["op"] == "llm.task_attempt" && f["provider"] == "gemini" {
			fallbacks++
		}
		if f["op"] == "llm.task_attempt" {
			t.Logf("attempt task=%v provider=%v duration=%v reason=%v", f["task"], f["provider"], f["duration"], f["reason"])
		}
	}
	t.Logf("provider_tokens=%v", byProvider)
	t.Logf("bank=%d core=%d followups=%d balance=%d evaluation=%s tokens=%v fallback=%d", len(bank.Questions), stored.AnsweredCount, followups, seed.balance(candidate), stored.EvaluationStatus, totals, fallbacks)
}
