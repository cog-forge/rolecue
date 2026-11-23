package provider

import (
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Persisted pre-interview setup (KAN-231).

func ProvideInterviewSetupRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

func ProvideInterviewSetupService(repo *repository.Repository) *service.Service {
	return service.New(repo)
}

func ProvideInterviewSetupHandler(svc *service.Service) *handler.InterviewSetupHandler {
	return handler.NewInterviewSetupHandler(svc)
}
