package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideAccountsHandler(cfg *config.Config, pool *pgxpool.Pool) *handler.AccountsHandler {
	svc := service.New(repository.NewRepository(pool), service.NewBetterAuthAdmin(cfg.Auth))
	return handler.NewAccountsHandler(svc)
}
