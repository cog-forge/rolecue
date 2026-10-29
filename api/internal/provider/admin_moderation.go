package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/service"
	evaluationservice "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideModerationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideModerationService(repo *repository.Repository, store storage.Store, log *logger.Logger, evaluation *evaluationservice.Service) (*service.Service, error) {
	svc, err := service.New(repo, store, log)
	if err != nil {
		return nil, err
	}
	svc.SetEvaluationReader(evaluation)
	return svc, nil
}
func ProvideModerationHandler(svc *service.Service) *handler.ModerationHandler {
	return handler.NewModerationHandler(svc)
}
