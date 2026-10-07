package provider

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth"
)

func ProvideAuthService(cfg *config.Config, pool *pgxpool.Pool) auth.AuthService {
	return auth.NewService(auth.NewHTTPVerifier(cfg.Auth), pool)
}
