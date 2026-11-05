package provider

import (
	accounthandler "github.com/cog-forge/rolecue/api/internal/features/account/handler"
	accountrepository "github.com/cog-forge/rolecue/api/internal/features/account/repository"
	accountservice "github.com/cog-forge/rolecue/api/internal/features/account/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideAccountRepository(pool *pgxpool.Pool) *accountrepository.Repository {
	return accountrepository.NewRepository(pool)
}

func ProvideAccountService(repo *accountrepository.Repository, store storage.Store) *accountservice.Service {
	return accountservice.NewService(repo, store)
}

func ProvideAccountHandler(svc *accountservice.Service) *accounthandler.Handler {
	return accounthandler.NewHandler(svc)
}
