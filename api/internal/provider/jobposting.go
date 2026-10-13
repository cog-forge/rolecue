package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/jobposting/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jobposting/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideJobPostingRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideJobPostingService(repo *repository.Repository, store storage.Store) (*service.Service, error) {
	return service.New(repo, store)
}
func ProvideJobPostingHandler(svc *service.Service) *handler.JobPostingHandler {
	return handler.NewJobPostingHandler(svc)
}
