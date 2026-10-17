package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideVoiceCatalogRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideVoiceCatalogService(repo *repository.Repository, log *logger.Logger) (*service.Service, error) {
	return service.New(repo, log)
}
func ProvideVoiceCatalogHandler(svc *service.Service) *handler.VoiceCatalogHandler {
	return handler.NewVoiceCatalogHandler(svc)
}
