package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/repository"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideMarketplaceRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

func ProvideMarketplaceService(repo *repository.Repository, store storage.Store) *service.Service {
	return service.New(repo, store)
}

func ProvideMarketplaceHandler(svc *service.Service) *handler.MarketplaceHandler {
	return handler.NewMarketplaceHandler(svc)
}
