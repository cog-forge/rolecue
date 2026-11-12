package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvideEvaluationRepository reads the session question limit from the interview config, the same value
// the interview start uses, so the coverage view of a weight change agrees with the sampler.
func ProvideEvaluationRepository(pool *pgxpool.Pool, cfg *config.Config) *repository.Repository {
	return repository.NewRepository(pool, repository.WithQuestionLimit(cfg.Interview.QuestionLimit))
}
func ProvideEvaluationService(cfg *config.Config, repo *repository.Repository) (*service.Service, error) {
	svc, err := service.New(repo, cfg.Evaluation.WeightTotal)
	if err != nil {
		return nil, err
	}
	svc.SetCoverageReader(repo)
	return svc, nil
}
func ProvideEvaluationHandler(svc *service.Service) *handler.EvaluationHandler {
	return handler.NewEvaluationHandler(svc)
}

func ProvideRubricService(repo *repository.Repository) (*service.Rubrics, error) {
	return service.NewRubrics(repo)
}
func ProvideRubricHandler(svc *service.Rubrics) *handler.RubricHandler {
	return handler.NewRubricHandler(svc)
}
