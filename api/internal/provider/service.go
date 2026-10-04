package provider

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/swp391-group3/ai-interview-practice/api/internal/config"
	"github.com/swp391-group3/ai-interview-practice/api/internal/features/auth"
)

func ProvideAuthService(cfg *config.Config, pool *pgxpool.Pool) auth.AuthService {
	return auth.NewService(auth.NewHTTPVerifier(cfg.Auth), pool)
}
