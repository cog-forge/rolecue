package provider

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/overview/service"
	walletdomain "github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	walletsvc "github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/jackc/pgx/v5/pgxpool"
)

// walletFinance reads the overview's finance block from the wallet's admin billing summary, so the
// dashboard and the billing screen can never disagree about revenue.
type walletFinance struct{ wallet *walletsvc.Service }

func (f walletFinance) Summary(ctx context.Context, from, to time.Time) (domain.OverviewFinance, error) {
	summary, err := f.wallet.GetBillingSummary(ctx, walletdomain.AdminTransactionFilter{DateFrom: &from, DateTo: &to})
	if err != nil {
		return domain.OverviewFinance{}, err
	}
	return domain.OverviewFinance{
		RevenueVND: summary.TotalRevenue, SuccessfulDeposits: summary.SuccessfulDeposits,
		CoinsSold: summary.CoinsSold, CoinsSpent: summary.CoinsSpent, CoinsRefunded: summary.CoinsRefunded,
		UniquePayers: summary.UniquePayers, SuccessRate: summary.SuccessRate,
	}, nil
}

func ProvideAdminOverviewRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

func ProvideAdminOverviewService(repo *repository.Repository, wallet *walletsvc.Service) (*service.Service, error) {
	return service.New(repo, walletFinance{wallet: wallet})
}

func ProvideAdminOverviewHandler(svc *service.Service) *handler.AdminOverviewHandler {
	return handler.NewAdminOverviewHandler(svc)
}
