package tests

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var vn = time.FixedZone("UTC+07:00", 7*60*60)

func vnDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, vn)
}

func TestComparisonWindow(t *testing.T) {
	cases := []struct {
		name             string
		from, to         time.Time
		compare          string
		wantFrom, wantTo time.Time
	}{
		{
			name: "month to date against the same days last year",
			from: vnDay(2026, 10, 1), to: vnDay(2026, 10, 28), compare: domain.CompareSamePeriodLastYear,
			wantFrom: vnDay(2025, 10, 1), wantTo: vnDay(2025, 10, 28),
		},
		{
			name: "leap day maps to 28 February",
			from: vnDay(2028, 2, 29), to: vnDay(2028, 3, 1), compare: domain.CompareSamePeriodLastYear,
			wantFrom: vnDay(2027, 2, 28), wantTo: vnDay(2027, 3, 1),
		},
		{
			name: "previous period has the same length and ends where the window starts",
			from: vnDay(2026, 10, 1), to: vnDay(2026, 10, 28), compare: domain.ComparePreviousPeriod,
			wantFrom: vnDay(2026, 9, 4), wantTo: vnDay(2026, 10, 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to := service.ComparisonWindow(tc.from, tc.to, tc.compare)
			assert.True(t, from.Equal(tc.wantFrom), "from %v, want %v", from, tc.wantFrom)
			assert.True(t, to.Equal(tc.wantTo), "to %v, want %v", to, tc.wantTo)
		})
	}
}

func TestReportGranularity(t *testing.T) {
	assert.Equal(t, "day", service.ReportGranularity(vnDay(2026, 10, 1), vnDay(2026, 10, 28)))
	assert.Equal(t, "day", service.ReportGranularity(vnDay(2026, 9, 1), vnDay(2026, 11, 2)), "62 days is still daily")
	assert.Equal(t, "month", service.ReportGranularity(vnDay(2026, 7, 1), vnDay(2026, 10, 1)))
}

func TestFeatureCategory(t *testing.T) {
	assert.Equal(t, "practice_interview", service.FeatureCategory("Practice interview 5c1f2e8a-0000-4000-8000-000000000001"))
	assert.Equal(t, "interview_slots", service.FeatureCategory("Interview capacity of job posting 7b0c4d2e, 10 slots"))
	assert.Equal(t, "avatar_storage", service.FeatureCategory("Avatar inventory, 1 slots"))
	assert.Equal(t, "practice_interview", service.FeatureCategory("Practice interview charge"))
	assert.Equal(t, "other", service.FeatureCategory("Manual adjustment"))
	assert.Equal(t, "other", service.FeatureCategory(""))
}

func TestBuildRevenueReport_ComparesBucketsMixAndFeatures(t *testing.T) {
	req := domain.RevenueReportRequest{From: vnDay(2026, 10, 1), To: vnDay(2026, 10, 4), Compare: domain.CompareSamePeriodLastYear}
	current := domain.RevenuePeriodData{
		Summary: domain.BillingSummary{
			TotalRevenue: 150000, SuccessfulDeposits: 2, TotalDeposits: 3, UniquePayers: 2, CoinsSold: 150,
			CoinsSpent: 15, CoinsRefunded: 4,
			PackageBreakdown: []domain.PackageRevenueItem{{PackageName: "Standard", RevenueVND: 100000, DepositCount: 1}, {PackageName: "Starter", RevenueVND: 50000, DepositCount: 1}},
			FeatureBreakdown: []domain.FeatureCoinItem{
				{FeatureName: "Practice interview a", CoinsSpent: 5, SpendCount: 1},
				{FeatureName: "Practice interview b", CoinsSpent: 5, SpendCount: 1},
				{FeatureName: "Avatar inventory, 5 slots", CoinsSpent: 5, SpendCount: 1},
			},
		},
		// 23:59 on 1 Oct in Vietnam is still 1 Oct; 00:01 on 3 Oct is 3 Oct.
		Topups: []domain.RevenueTopup{
			{CreatedAt: time.Date(2026, 10, 1, 16, 59, 0, 0, time.UTC), UserRole: "recruiter", PackageName: "Standard", PaidVND: 100000},
			{CreatedAt: time.Date(2026, 10, 2, 17, 1, 0, 0, time.UTC), UserRole: "candidate", PackageName: "Starter", PaidVND: 50000},
		},
		NewPayers: 1,
	}
	comparison := domain.RevenuePeriodData{
		Summary: domain.BillingSummary{
			TotalRevenue: 100000, SuccessfulDeposits: 1, TotalDeposits: 1, UniquePayers: 1, CoinsSold: 100,
			PackageBreakdown: []domain.PackageRevenueItem{{PackageName: "Standard", RevenueVND: 100000, DepositCount: 1}},
			FeatureBreakdown: []domain.FeatureCoinItem{{FeatureName: "Practice interview c", CoinsSpent: 5, SpendCount: 1}},
		},
		Topups: []domain.RevenueTopup{
			{CreatedAt: time.Date(2025, 10, 2, 3, 0, 0, 0, time.UTC), UserRole: "recruiter", PackageName: "Standard", PaidVND: 100000},
		},
	}

	report := service.BuildRevenueReport(req, current, comparison, 300, 600000, 600)

	assert.Equal(t, "day", report.Granularity)
	assert.True(t, report.ComparisonWindow.From.Equal(vnDay(2025, 10, 1)))
	assert.Equal(t, domain.RevenuePeriodMetrics{
		RevenueVND: 150000, SuccessfulTopups: 2, TopupAttempts: 3, PaymentSuccessRate: 66.7, PayingUsers: 2,
		NewPayers: 1, AverageOrderVND: 75000, CoinsSold: 150, CoinsSpent: 15, CoinsRefunded: 4,
	}, report.Current)
	assert.Equal(t, int64(100000), report.Comparison.AverageOrderVND)

	require.Len(t, report.Series, 3)
	assert.Equal(t, []int64{100000, 0, 50000}, []int64{report.Series[0].RevenueVND, report.Series[1].RevenueVND, report.Series[2].RevenueVND})
	assert.Equal(t, []int64{0, 100000, 0}, []int64{report.Series[0].ComparisonRevenueVND, report.Series[1].ComparisonRevenueVND, report.Series[2].ComparisonRevenueVND})
	assert.True(t, report.Series[0].Start.Equal(vnDay(2026, 10, 1)))
	assert.True(t, report.Series[1].ComparisonStart.Equal(vnDay(2025, 10, 2)))

	assert.Equal(t, []domain.RevenueMixItem{
		{Key: "recruiter", RevenueVND: 100000, ComparisonRevenueVND: 100000, Topups: 1},
		{Key: "candidate", RevenueVND: 50000, ComparisonRevenueVND: 0, Topups: 1},
	}, report.ByRole)
	assert.Equal(t, []domain.RevenueMixItem{
		{Key: "Standard", RevenueVND: 100000, ComparisonRevenueVND: 100000, Topups: 1},
		{Key: "Starter", RevenueVND: 50000, ComparisonRevenueVND: 0, Topups: 1},
	}, report.ByPackage)
	assert.Equal(t, []domain.CoinFeatureItem{
		{Feature: "practice_interview", Coins: 10, ComparisonCoins: 5, Charges: 2},
		{Feature: "avatar_storage", Coins: 5, ComparisonCoins: 0, Charges: 1},
	}, report.SpendByFeature)

	assert.Equal(t, int64(300), report.OutstandingCoins)
	assert.Equal(t, int64(300000), report.OutstandingValueVND, "1,000 VND per coin over all time")
}

func TestBuildRevenueReport_MonthBucketsAndEmptyComparison(t *testing.T) {
	req := domain.RevenueReportRequest{From: vnDay(2026, 8, 1), To: vnDay(2026, 10, 28), Compare: domain.CompareSamePeriodLastYear}
	current := domain.RevenuePeriodData{
		Topups: []domain.RevenueTopup{
			{CreatedAt: time.Date(2026, 8, 31, 17, 30, 0, 0, time.UTC), PaidVND: 50000}, // 1 Sep in Vietnam
		},
	}

	report := service.BuildRevenueReport(req, current, domain.RevenuePeriodData{}, 0, 0, 0)

	assert.Equal(t, "month", report.Granularity)
	require.Len(t, report.Series, 3)
	assert.Equal(t, int64(0), report.Series[0].RevenueVND)
	assert.Equal(t, int64(50000), report.Series[1].RevenueVND)
	assert.True(t, report.Series[2].Start.Equal(vnDay(2026, 10, 1)))
	assert.Equal(t, []domain.RevenueMixItem{{Key: "unknown", RevenueVND: 50000, Topups: 1}}, report.ByRole)
	assert.Equal(t, int64(0), report.OutstandingValueVND, "no coins sold yet means no price to value them at")
	assert.Equal(t, 0.0, report.Current.PaymentSuccessRate)
}

func TestGetRevenueReport_ValidatesAndLoadsBothWindows(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects a missing, reversed or overlong window and an unknown comparison", func(t *testing.T) {
		svc := service.NewService(new(MockRepo), new(MockPayOS))
		for _, req := range []domain.RevenueReportRequest{
			{},
			{From: vnDay(2026, 10, 2), To: vnDay(2026, 10, 1)},
			{From: vnDay(2016, 1, 1), To: vnDay(2026, 10, 1)},
			{From: vnDay(2026, 10, 1), To: vnDay(2026, 10, 2), Compare: "last_week"},
		} {
			_, err := svc.GetRevenueReport(ctx, req)
			assert.True(t, apperror.IsCode(err, apperror.CodeValidation), "%+v", req)
		}
	})

	t.Run("defaults to the same period last year", func(t *testing.T) {
		repo := new(MockRepo)
		svc := service.NewService(repo, new(MockPayOS))
		from, to := vnDay(2026, 10, 1), vnDay(2026, 10, 28)
		repo.On("GetRevenuePeriod", mock.Anything, mock.MatchedBy(func(t time.Time) bool { return t.Equal(from) }), mock.Anything).
			Return(domain.RevenuePeriodData{Summary: domain.BillingSummary{TotalRevenue: 200000}}, nil).Once()
		repo.On("GetRevenuePeriod", mock.Anything, mock.MatchedBy(func(t time.Time) bool { return t.Equal(vnDay(2025, 10, 1)) }), mock.Anything).
			Return(domain.RevenuePeriodData{Summary: domain.BillingSummary{TotalRevenue: 150000}}, nil).Once()
		repo.On("GetCoinTotals", mock.Anything).Return(int64(503), int64(700000), int64(700), nil).Once()

		report, err := svc.GetRevenueReport(ctx, domain.RevenueReportRequest{From: from, To: to})

		require.NoError(t, err)
		assert.Equal(t, domain.CompareSamePeriodLastYear, report.Compare)
		assert.Equal(t, int64(200000), report.Current.RevenueVND)
		assert.Equal(t, int64(150000), report.Comparison.RevenueVND)
		assert.Equal(t, int64(503000), report.OutstandingValueVND)
		repo.AssertExpectations(t)
	})
}

func TestBuildRevenueReport_LeavesPendingPaymentsOutOfTheSuccessRate(t *testing.T) {
	req := domain.RevenueReportRequest{From: vnDay(2026, 10, 1), To: vnDay(2026, 10, 28), Compare: domain.CompareSamePeriodLastYear}
	// 3 paid, 2 failed and 1 still waiting at PayOS: the waiting one has no outcome yet.
	current := domain.RevenuePeriodData{Summary: domain.BillingSummary{SuccessfulDeposits: 3, TotalDeposits: 6, PendingCount: 1}}

	report := service.BuildRevenueReport(req, current, domain.RevenuePeriodData{}, 0, 0, 0)

	assert.Equal(t, int64(5), report.Current.TopupAttempts)
	assert.Equal(t, 60.0, report.Current.PaymentSuccessRate)
}
