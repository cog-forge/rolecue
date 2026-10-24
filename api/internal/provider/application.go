package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideApplicationRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideApplicationService signs the CV links of Application Detail with the same storage Store the
// Job Posting service uses for its PDF links.
func ProvideApplicationService(repo *repository.Repository, store storage.Store) (*service.Service, error) {
	return service.New(repo, store)
}
func ProvideApplicationHandler(svc *service.Service) *handler.ApplicationHandler {
	return handler.NewApplicationHandler(svc)
}
