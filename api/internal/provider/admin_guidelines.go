package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideAdminGuidelinesRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// ProvideAdminGuidelinesService hands the service the configured follow-up limit, the ceiling a saved
// follow_up_max can only lower.
func ProvideAdminGuidelinesService(repo *repository.Repository, cfg *config.Config) (*service.Service, error) {
	return service.New(repo, int32(cfg.Interview.FollowUpLimit))
}

func ProvideAdminGuidelinesHandler(svc *service.Service) *handler.AdminGuidelinesHandler {
	return handler.NewAdminGuidelinesHandler(svc)
}
