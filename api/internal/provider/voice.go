package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/voice/repository"
	"github.com/cog-forge/rolecue/api/internal/features/voice/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideVoiceRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideVoiceService(repo *repository.Repository) (*service.Service, error) {
	return service.New(repo)
}
func ProvideVoiceHandler(svc *service.Service) *handler.VoiceHandler {
	return handler.NewVoiceHandler(svc)
}
