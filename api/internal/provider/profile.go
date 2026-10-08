package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/profile/repository"
	"github.com/cog-forge/rolecue/api/internal/features/profile/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideProfileRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideProfileService(repo *repository.Repository) *service.Service {
	return service.NewService(repo)
}
func ProvideProfileHandler(svc *service.Service) *handler.ProfileHandler {
	return handler.NewProfileHandler(svc)
}
