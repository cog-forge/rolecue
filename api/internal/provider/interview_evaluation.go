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

// ProvideInterviewScorer builds the scorer from the evaluation task route. When no LLM
// can be built, it returns a scorer that fails each evaluation with the reason instead of stopping the server.
func ProvideInterviewScorer(cfg *config.Config) service.Scorer {
	model, err := ai.NewTaskAdapter(context.Background(), cfg, config.TaskEvaluation)
	if err != nil {
		return evalprovider.NewUnavailableScorer(err)
	}
	return model
}
func ProvideInterviewEvaluationService(repo *repository.Repository, scorer service.Scorer) (*service.Scoring, error) {
	return service.NewScoring(repo, scorer)
}
func ProvideInterviewReportHandler(svc *service.Scoring) *handler.InterviewReportHandler {
	return handler.NewInterviewReportHandler(svc)
}
