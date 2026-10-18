package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideEvaluationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideEvaluationService(cfg *config.Config, repo *repository.Repository) (*service.Service, error) {
	return service.New(repo, cfg.Evaluation.WeightTotal)
}
func ProvideEvaluationHandler(svc *service.Service) *handler.EvaluationHandler {
	return handler.NewEvaluationHandler(svc)
}
