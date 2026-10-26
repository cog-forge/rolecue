package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideAdminInterviewsRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

func ProvideAdminInterviewsService(repo *repository.Repository) (*service.Service, error) {
	return service.New(repo)
}

func ProvideAdminInterviewsHandler(svc *service.Service) *handler.AdminInterviewsHandler {
	return handler.NewAdminInterviewsHandler(svc)
}
