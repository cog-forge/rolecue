package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/inventory/repository"
	"github.com/cog-forge/rolecue/api/internal/features/inventory/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideInventoryRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideInventoryService(repo *repository.Repository, store storage.Store) *service.Service {
	return service.NewService(repo, store)
}
func ProvideInventoryHandler(svc *service.Service) *handler.InventoryHandler {
	return handler.NewInventoryHandler(svc)
}
