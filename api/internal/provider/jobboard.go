package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/jobboard/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jobboard/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideJobBoardRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}
func ProvideJobBoardService(repo *repository.Repository, store storage.Store) (*service.Service, error) {
	return service.New(repo, store)
}
func ProvideJobBoardHandler(svc *service.Service) *handler.JobBoardHandler {
	return handler.NewJobBoardHandler(svc)
}
