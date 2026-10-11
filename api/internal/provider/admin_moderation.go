package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideModerationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideModerationService(repo *repository.Repository, log *logger.Logger) (*service.Service, error) {
	return service.New(repo, log)
}
func ProvideModerationHandler(svc *service.Service) *handler.ModerationHandler {
	return handler.NewModerationHandler(svc)
}
