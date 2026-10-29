package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	voiceprovider "github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/provider"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideVoiceCatalogRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideVoiceCatalogProvider builds the Azure AI Speech adapter. Missing credentials don't stop the API:
// the sync reports the provider as not configured.
func ProvideVoiceCatalogProvider(cfg *config.Config) (service.VoiceProvider, error) {
	return voiceprovider.NewAzure(cfg.TTS.Key, cfg.TTS.Region, cfg.TTS.Timeout)
}
func ProvideVoiceCatalogService(repo *repository.Repository, voices service.VoiceProvider, cfg *config.Config, log *logger.Logger) (*service.Service, error) {
	return service.New(repo, voices, cfg.TTS.SyncLocales, log)
}
func ProvideVoiceCatalogHandler(svc *service.Service) *handler.VoiceCatalogHandler {
	return handler.NewVoiceCatalogHandler(svc)
}
