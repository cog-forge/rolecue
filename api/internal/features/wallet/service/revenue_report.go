package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// vietnamTime is the reporting timezone. Vietnam has no DST, so a fixed offset is exact.
var vietnamTime = time.FixedZone("UTC+07:00", 7*60*60)

// maxReportWindow keeps a custom window to a bounded number of buckets.
const maxReportWindow = 3660 * 24 * time.Hour

// dailyReportLimit is the longest window drawn by day; longer windows are drawn by month.
const dailyReportLimit = 62 * 24 * time.Hour

// GetRevenueReport loads the window, its comparison window and the coin totals.
func (s *Service) GetRevenueReport(ctx context.Context, req domain.RevenueReportRequest) (domain.RevenueReport, error) {
	if req.From.IsZero() || req.To.IsZero() {
		return domain.RevenueReport{}, apperror.New(apperror.CodeValidation, "date_from and date_to are required")
	}
	if !req.From.Before(req.To) {
		return domain.RevenueReport{}, apperror.New(apperror.CodeValidation, "date_from must be before date_to")
	}
	if req.To.Sub(req.From) > maxReportWindow {
		return domain.RevenueReport{}, apperror.New(apperror.CodeValidation, "the report window must be at most 10 years")
	}
	switch req.Compare {
	case "":
		req.Compare = domain.CompareSamePeriodLastYear
	case domain.CompareSamePeriodLastYear, domain.ComparePreviousPeriod:
	default:
		return domain.RevenueReport{}, apperror.New(apperror.CodeValidation, "invalid compare: must be previous_year or previous_period")
	}

	current, err := s.repo.GetRevenuePeriod(ctx, req.From, req.To)
	if err != nil {
		return domain.RevenueReport{}, err
	}
	compFrom, compTo := ComparisonWindow(req.From, req.To, req.Compare)
	comparison, err := s.repo.GetRevenuePeriod(ctx, compFrom, compTo)
	if err != nil {
		return domain.RevenueReport{}, err
	}
	outstanding, allTimeRevenue, allTimeCoinsSold, err := s.repo.GetCoinTotals(ctx)
	if err != nil {
		return domain.RevenueReport{}, err
	}
	return BuildRevenueReport(req, current, comparison, outstanding, allTimeRevenue, allTimeCoinsSold), nil
}

// ComparisonWindow returns the same dates one year earlier (29 February becomes 28 February), or the
// window of the same length that ends where this one starts.
func ComparisonWindow(from, to time.Time, compare string) (time.Time, time.Time) {
	if compare == domain.ComparePreviousPeriod {
		return from.Add(-to.Sub(from)), from
	}
	return yearEarlier(from), yearEarlier(to)
}

func yearEarlier(t time.Time) time.Time {
	local := t.In(vietnamTime)
	day := local.Day()
	if local.Month() == time.February && day == 29 {
		day = 28
	}
	return time.Date(local.Year()-1, local.Month(), day, local.Hour(), local.Minute(), local.Second(),
		local.Nanosecond(), vietnamTime)
}

// ReportGranularity draws windows of up to 62 days by day and longer ones by month.
func ReportGranularity(from, to time.Time) string {
	if to.Sub(from) <= dailyReportLimit {
		return "day"
	}
	return "month"
}

// FeatureCategory groups a coin debit by the wording the ledger writes for it.
func FeatureCategory(description string) string {
	switch {
	case strings.HasPrefix(description, "Practice interview"):
		return "practice_interview"
	case strings.HasPrefix(description, "Interview capacity of job posting"):
		return "interview_slots"
	case strings.HasPrefix(description, "Avatar inventory"):
		return "avatar_storage"
	default:
		return "other"
	}
}

// BuildRevenueReport turns the two windows' data into the report. Buckets are calendar days or months
// in Vietnam time, and the comparison is matched to the window bucket by bucket.
func BuildRevenueReport(req domain.RevenueReportRequest, current, comparison domain.RevenuePeriodData,
	outstandingCoins, allTimeRevenueVND, allTimeCoinsSold int64) domain.RevenueReport {
	compFrom, compTo := ComparisonWindow(req.From, req.To, req.Compare)
	granularity := ReportGranularity(req.From, req.To)

	report := domain.RevenueReport{
		Compare:          req.Compare,
		Granularity:      granularity,
		Window:           domain.RevenueWindow{From: req.From.UTC(), To: req.To.UTC()},
		ComparisonWindow: domain.RevenueWindow{From: compFrom.UTC(), To: compTo.UTC()},
		Current:          periodMetrics(current),
		Comparison:       periodMetrics(comparison),
		ByRole:           roleMix(current.Topups, comparison.Topups),
		ByPackage:        packageMix(current.Summary.PackageBreakdown, comparison.Summary.PackageBreakdown),
		SpendByFeature:   featureSpend(current.Summary.FeatureBreakdown, comparison.Summary.FeatureBreakdown),
		OutstandingCoins: outstandingCoins,
	}
	if allTimeCoinsSold > 0 {
		report.OutstandingValueVND = outstandingCoins * allTimeRevenueVND / allTimeCoinsSold
	}

	starts := bucketStarts(req.From, req.To, granularity)
	compStarts := bucketStarts(compFrom, compTo, granularity)
	report.Series = make([]domain.RevenueSeriesPoint, len(starts))
	for i, start := range starts {
		report.Series[i].Start = start
		if i < len(compStarts) {
			report.Series[i].ComparisonStart = compStarts[i]
		}
	}
	for _, topup := range current.Topups {
		if i := bucketIndex(starts, topup.CreatedAt, granularity); i >= 0 {
			report.Series[i].RevenueVND += topup.PaidVND
		}
	}
	for _, topup := range comparison.Topups {
		if i := bucketIndex(compStarts, topup.CreatedAt, granularity); i >= 0 && i < len(starts) {
			report.Series[i].ComparisonRevenueVND += topup.PaidVND
		}
	}
	return report
}

func periodMetrics(data domain.RevenuePeriodData) domain.RevenuePeriodMetrics {
	s := data.Summary
	// A payment still waiting at PayOS has no outcome yet, so it is not an attempt that failed.
	settled := max(s.TotalDeposits-s.PendingCount, 0)
	metrics := domain.RevenuePeriodMetrics{
		RevenueVND:         s.TotalRevenue,
		SuccessfulTopups:   s.SuccessfulDeposits,
		TopupAttempts:      settled,
		PaymentSuccessRate: domain.DepositSuccessRate(s.SuccessfulDeposits, settled),
		PayingUsers:        s.UniquePayers,
		NewPayers:          data.NewPayers,
		CoinsSold:          s.CoinsSold,
		CoinsSpent:         s.CoinsSpent,
		CoinsRefunded:      s.CoinsRefunded,
	}
	if s.SuccessfulDeposits > 0 {
		metrics.AverageOrderVND = s.TotalRevenue / s.SuccessfulDeposits
	}
	return metrics
}

// bucketStarts lists the first instant of every Vietnam day or month that overlaps [from, to).
func bucketStarts(from, to time.Time, granularity string) []time.Time {
	local := from.In(vietnamTime)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, vietnamTime)
	if granularity == "month" {
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, vietnamTime)
	}
	var starts []time.Time
	for t := start; t.Before(to); {
		starts = append(starts, t)
		if granularity == "month" {
			t = t.AddDate(0, 1, 0)
		} else {
			t = t.AddDate(0, 0, 1)
		}
	}
	return starts
}

func bucketIndex(starts []time.Time, at time.Time, granularity string) int {
	if len(starts) == 0 {
		return -1
	}
	local, first := at.In(vietnamTime), starts[0]
	var i int
	if granularity == "month" {
		i = (local.Year()-first.Year())*12 + int(local.Month()-first.Month())
	} else {
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, vietnamTime)
		i = int(day.Sub(first).Hours() / 24)
	}
	if i < 0 || i >= len(starts) {
		return -1
	}
	return i
}

type mixTotals struct{ revenue, comparison, topups int64 }

func sortedMix(totals map[string]*mixTotals) []domain.RevenueMixItem {
	items := make([]domain.RevenueMixItem, 0, len(totals))
	for key, t := range totals {
		items = append(items, domain.RevenueMixItem{Key: key, RevenueVND: t.revenue, ComparisonRevenueVND: t.comparison, Topups: t.topups})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RevenueVND != items[j].RevenueVND {
			return items[i].RevenueVND > items[j].RevenueVND
		}
		if items[i].ComparisonRevenueVND != items[j].ComparisonRevenueVND {
			return items[i].ComparisonRevenueVND > items[j].ComparisonRevenueVND
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func roleMix(current, comparison []domain.RevenueTopup) []domain.RevenueMixItem {
	totals := map[string]*mixTotals{}
	at := func(role string) *mixTotals {
		if role == "" {
			role = "unknown"
		}
		if totals[role] == nil {
			totals[role] = &mixTotals{}
		}
		return totals[role]
	}
	for _, t := range current {
		m := at(t.UserRole)
		m.revenue += t.PaidVND
		m.topups++
	}
	for _, t := range comparison {
		at(t.UserRole).comparison += t.PaidVND
	}
	return sortedMix(totals)
}

func packageMix(current, comparison []domain.PackageRevenueItem) []domain.RevenueMixItem {
	totals := map[string]*mixTotals{}
	at := func(name string) *mixTotals {
		if totals[name] == nil {
			totals[name] = &mixTotals{}
		}
		return totals[name]
	}
	for _, p := range current {
		m := at(p.PackageName)
		m.revenue += p.RevenueVND
		m.topups += p.DepositCount
	}
	for _, p := range comparison {
		at(p.PackageName).comparison += p.RevenueVND
	}
	return sortedMix(totals)
}

func featureSpend(current, comparison []domain.FeatureCoinItem) []domain.CoinFeatureItem {
	totals := map[string]*domain.CoinFeatureItem{}
	at := func(description string) *domain.CoinFeatureItem {
		key := FeatureCategory(description)
		if totals[key] == nil {
			totals[key] = &domain.CoinFeatureItem{Feature: key}
		}
		return totals[key]
	}
	for _, f := range current {
		item := at(f.FeatureName)
		item.Coins += f.CoinsSpent
		item.Charges += f.SpendCount
	}
	for _, f := range comparison {
		at(f.FeatureName).ComparisonCoins += f.CoinsSpent
	}
	items := make([]domain.CoinFeatureItem, 0, len(totals))
	for _, item := range totals {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Coins != items[j].Coins {
			return items[i].Coins > items[j].Coins
		}
		return items[i].Feature < items[j].Feature
	})
	return items
}
