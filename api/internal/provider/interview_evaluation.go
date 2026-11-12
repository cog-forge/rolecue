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

// ProvideInterviewGrader builds the rubric grader (KAN-334) from the evaluation task route. When no LLM can be
// built, it returns a grader that fails each evaluation with the reason instead of stopping the server.
func ProvideInterviewGrader(cfg *config.Config) service.Grader {
	model, err := ai.NewTaskAdapter(context.Background(), cfg, config.TaskEvaluation)
	if err != nil {
		return evalprovider.NewUnavailableGrader(err)
	}
	return model
}

func ProvideInterviewEvaluationService(repo *repository.Repository, grader service.Grader) (*service.Scoring, error) {
	return service.NewScoring(repo, grader)
}

func ProvideInterviewReportHandler(svc *service.Scoring) *handler.InterviewReportHandler {
	return handler.NewInterviewReportHandler(svc)
}
