package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideApplicationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideApplicationService(repo *repository.Repository) (*service.Service, error) {
	return service.New(repo)
}
func ProvideApplicationHandler(svc *service.Service) *handler.ApplicationHandler {
	return handler.NewApplicationHandler(svc)
}
