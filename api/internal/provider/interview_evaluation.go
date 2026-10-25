package provider

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/config"
	evalprovider "github.com/cog-forge/rolecue/api/internal/features/evaluation/provider"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	interviewservice "github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
)

// The interview evaluation reuses ProvideEvaluationRepository for its repository.
var (
	_ interviewservice.Evaluator     = (*service.Scoring)(nil)
	_ handler.InterviewReportService = (*service.Scoring)(nil)
)

// ProvideInterviewScorer builds the LLM scorer from the same LLM config as question generation. When no LLM
// can be built, it returns a scorer that fails each evaluation with the reason instead of stopping the server.
func ProvideInterviewScorer(cfg *config.Config) service.Scorer {
	model, err := ai.NewChatModel(context.Background(), cfg.LLM)
	if err != nil {
		return evalprovider.NewUnavailableScorer(err)
	}
	scorer, err := evalprovider.New(model)
	if err != nil {
		return evalprovider.NewUnavailableScorer(err)
	}
	return scorer
}
func ProvideInterviewEvaluationService(repo *repository.Repository, scorer service.Scorer) (*service.Scoring, error) {
	return service.NewScoring(repo, scorer)
}
func ProvideInterviewReportHandler(svc *service.Scoring) *handler.InterviewReportHandler {
	return handler.NewInterviewReportHandler(svc)
}
