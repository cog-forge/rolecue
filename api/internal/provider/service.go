package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth/repository"
	"github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideAuthRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

func ProvideAuthService(cfg *config.Config, repo *repository.Repository) service.AuthService {
	return service.NewService(service.NewHTTPVerifier(cfg.Auth), repo)
}
