//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	evaluationrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	evaluationservice "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/google/uuid"
)

func TestModerationDossierUsesConfirmedRequirementsAndEffectiveEvaluation(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.recruiter()
	text := "Role description"
	posting := s.submittedPosting(owner, nil, &text)
	var jdID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT jd_id FROM job_postings WHERE id=$1`, posting).Scan(&jdID); err != nil {
		t.Fatal(err)
	}
	s.exec(`INSERT INTO requirements(jd_id,name,target,type) VALUES($1,'Go','programming_language','required')`, jdID)
	s.later(`DELETE FROM requirements WHERE jd_id=$1`, jdID)
	s.exec(`UPDATE job_descriptions SET refinement_note='Focus on concurrency' WHERE id=$1`, jdID)
	repo := repository.NewRepository(pool)
	got, err := repo.Get(ctx, posting)
	if err != nil || got.CompanyName == nil || *got.CompanyName != "Acme" || got.SeniorityLevel == "" || got.RefinementNote != "Focus on concurrency" || len(got.Requirements) != 1 || got.Requirements[0].Name != "Go" {
		t.Fatalf("dossier=%+v error=%v", got, err)
	}
	svc := newService(t, pool)
	evaluation, err := evaluationservice.New(evaluationrepository.NewRepository(pool), 100)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetEvaluationReader(evaluation)
	got, err = svc.Get(ctx, "admin", posting)
	if err != nil || got.Evaluation == nil || !got.Evaluation.UsingDefaults || got.Evaluation.Editable || len(got.Evaluation.Weights) == 0 {
		t.Fatalf("effective evaluation missing: %+v %v", got.Evaluation, err)
	}
	sum := 0.0
	for _, weight := range got.Evaluation.Weights {
		sum += weight.Percentage
	}
	if sum != 100 {
		t.Fatalf("defaults sum=%g", sum)
	}
}
