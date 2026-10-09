package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	walletrepo "github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	walletsvc "github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideWalletRepository(pool *pgxpool.Pool) *walletrepo.Repository {
	return walletrepo.NewRepository(pool)
}

func ProvidePayOSClient(cfg *config.Config) *payos.Client {
	return payos.NewClient(cfg.PayOS.ClientID, cfg.PayOS.APIKey, cfg.PayOS.ChecksumKey)
}

func ProvideWalletService(repo *walletrepo.Repository, payosClient *payos.Client) *walletsvc.Service {
	return walletsvc.NewService(repo, payosClient)
}

func ProvideWalletHandler(svc *walletsvc.Service) *handler.WalletHandler {
	return handler.NewWalletHandler(svc)
}
