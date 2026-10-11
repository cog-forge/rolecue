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
	return payos.MustNewClientWithOptions(&payos.Options{
		ClientId:    cfg.PayOS.ClientID,
		ApiKey:      cfg.PayOS.APIKey,
		ChecksumKey: cfg.PayOS.ChecksumKey,
		Timeout:     cfg.PayOS.Timeout,
		MaxRetries:  cfg.PayOS.MaxRetries,
	})
}

func ProvideWalletService(repo *walletrepo.Repository, payosClient *payos.Client) *walletsvc.Service {
	return walletsvc.NewService(repo, payosClient)
}

func ProvideWalletHandler(svc *walletsvc.Service) *handler.WalletHandler {
	return handler.NewWalletHandler(svc)
}
