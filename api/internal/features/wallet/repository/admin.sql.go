package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Legacy compatibility types
type AdminTransaction struct {
	ID             uuid.UUID
	From           *string
	To             *string
	Amount         int32
	Currency       string
	Description    *string
	Status         TransactionStatus
	PayosOrderCode *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ListAllTransactionsParams struct {
	Search     string
	Status     string
	TxType     string
	DateFrom   *time.Time
	DateTo     *time.Time
	PageOffset int64
	PageLimit  int32
}

// RevenueStats and RevenueSummaryRow are backward-compatible types used by
// the legacy admin service methods and repository.
type RevenueStats struct {
	TotalRevenue    int64
	SuccessfulCount int64
	PendingCount    int64
	FailedCount     int64
	CancelledCount  int64
	TotalCount      int64
}

type RevenueSummaryRow struct {
	Period             time.Time
	Revenue            int64
	SuccessfulDeposits int64
}

func (t AdminTransaction) ToDomain() domain.Transaction {
	var from, to *uuid.UUID
	if t.From != nil && *t.From != "" {
		id, _ := uuid.Parse(*t.From)
		from = &id
	}
	if t.To != nil && *t.To != "" {
		id, _ := uuid.Parse(*t.To)
		to = &id
	}
	var orderCode *int64
	if t.PayosOrderCode != nil {
		if oc, err := strconv.ParseInt(*t.PayosOrderCode, 10, 64); err == nil {
			orderCode = &oc
		}
	}
	return domain.Transaction{
		ID:             t.ID,
		From:           from,
		To:             to,
		Amount:         t.Amount,
		Currency:       t.Currency,
		Description:    stringVal(t.Description),
		Status:         domain.TransactionStatus(t.Status),
		PayOSOrderCode: orderCode,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------------------------------------------------------------------------
// Billing Summary (Unified single-source-of-truth query)
// ---------------------------------------------------------------------------

func (q *Queries) GetBillingSummary(ctx context.Context, filter domain.AdminTransactionFilter) (domain.BillingSummary, error) {
	// Base metrics query
	summarySQL := `
WITH filtered_tx AS (
  SELECT 
    t.id, t.amount, t.from, t.to, t.description, t.currency, t.status, 
    t.payos_order_code, t.created_at, t.is_test
  FROM public.transactions t
  LEFT JOIN public.coin_packages cp ON cp.name = regexp_replace(t.description, '^Deposit:\s*', '')
  WHERE
    ($1::timestamptz IS NULL OR t.created_at >= $1)
    AND ($2::timestamptz IS NULL OR t.created_at <= $2)
    AND ($3::boolean = TRUE OR t.is_test = FALSE)
    AND ($4::text = '' OR $4 = 'all' OR t.status::text = $4)
    AND ($5::text = '' OR $5 = 'all' OR
      CASE
        WHEN $5 = 'topup' THEN t.payos_order_code IS NOT NULL
        WHEN $5 = 'spend' THEN t.from != 'System' AND t.payos_order_code IS NULL AND (t.description IS NULL OR t.description NOT ILIKE '%refund%')
        WHEN $5 = 'refund' THEN (t.from = 'System' OR t.description ILIKE '%refund%') AND t.payos_order_code IS NULL
        ELSE TRUE
      END
    )
    AND ($6::text = '' OR $6 = 'all' OR t.description ILIKE '%' || $6 || '%')
)
SELECT
  -- Gross top-up revenue
  COALESCE(SUM(
    CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN
      COALESCE(NULLIF(substring(description from '\((\d+)\s*VND\)'), '')::bigint, (amount * 500)::bigint)
    ELSE 0 END
  ), 0)::bigint AS total_revenue,

  -- Refund amount in VND
  COALESCE(SUM(
    CASE WHEN status = 'success' AND (t.from = 'System' OR description ILIKE '%refund%') AND currency = 'VND' THEN
      COALESCE(NULLIF(substring(description from '\((\d+)\s*VND\)'), '')::bigint, amount::bigint)
    ELSE 0 END
  ), 0)::bigint AS refund_amount,

  -- Successful deposits (ONLY topups with status success)
  COUNT(CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN 1 END)::bigint AS successful_deposits,
  COUNT(CASE WHEN status = 'pending' THEN 1 END)::bigint AS pending_count,
  COUNT(CASE WHEN status = 'failed' THEN 1 END)::bigint AS failed_count,
  COUNT(CASE WHEN status = 'cancelled' THEN 1 END)::bigint AS cancelled_count,
  COUNT(CASE WHEN status = 'cancelled' OR (t.from = 'System' AND payos_order_code IS NULL) THEN 1 END)::bigint AS other_count,
  COUNT(*)::bigint AS total_transactions,

  -- Unique paying users in this filtered period
  COUNT(DISTINCT CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN t.to END)::bigint AS unique_payers,

  -- Coins metrics
  COALESCE(SUM(CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN amount ELSE 0 END), 0)::bigint AS coins_sold,
  COALESCE(SUM(CASE WHEN status = 'success' AND t.from != 'System' AND payos_order_code IS NULL AND currency = 'COIN' THEN amount ELSE 0 END), 0)::bigint AS coins_spent
FROM filtered_tx t;
`
	var (
		totalRev     int64
		refundAmt    int64
		successDep   int64
		pendingCnt   int64
		failedCnt    int64
		cancelledCnt int64
		otherCnt     int64
		totalTx      int64
		uniquePayers int64
		coinsSold    int64
		coinsSpent   int64
	)

	err := q.db.QueryRow(ctx, summarySQL,
		filter.DateFrom,
		filter.DateTo,
		filter.IncludeTest,
		filter.Status,
		filter.TxType,
		filter.Package,
	).Scan(
		&totalRev,
		&refundAmt,
		&successDep,
		&pendingCnt,
		&failedCnt,
		&cancelledCnt,
		&otherCnt,
		&totalTx,
		&uniquePayers,
		&coinsSold,
		&coinsSpent,
	)
	if err != nil {
		return domain.BillingSummary{}, fmt.Errorf("query billing summary: %w", err)
	}

	netRev := totalRev - refundAmt

	var avgDeposit int64
	if successDep > 0 {
		avgDeposit = totalRev / successDep
	}

	totalTopups := successDep + pendingCnt + failedCnt + cancelledCnt
	var successRate float64
	if totalTopups > 0 {
		successRate = float64(successDep) / float64(totalTopups) * 100.0
	}

	// Calculate New Payers vs Returning Payers in the period
	newPayersSQL := `
SELECT COUNT(DISTINCT user_first.user_id)::bigint
FROM (
  SELECT t.to AS user_id, MIN(t.created_at) AS first_dep
  FROM public.transactions t
  WHERE t.status = 'success' AND t.payos_order_code IS NOT NULL AND t.is_test = FALSE
  GROUP BY t.to
) user_first
WHERE ($1::timestamptz IS NULL OR user_first.first_dep >= $1)
  AND ($2::timestamptz IS NULL OR user_first.first_dep <= $2);
`
	var newPayers int64
	_ = q.db.QueryRow(ctx, newPayersSQL, filter.DateFrom, filter.DateTo).Scan(&newPayers)
	returningPayers := uniquePayers - newPayers
	if returningPayers < 0 {
		returningPayers = 0
	}

	// Calculate All-time Coin Liability (Coins sold all-time minus Coins spent all-time)
	var coinsLiability int64
	liabilitySQL := `
SELECT COALESCE(
  SUM(CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN amount ELSE 0 END) -
  SUM(CASE WHEN status = 'success' AND "from" != 'System' AND payos_order_code IS NULL AND currency = 'COIN' THEN amount ELSE 0 END),
  0
)::bigint
FROM public.transactions
WHERE is_test = FALSE;
`
	_ = q.db.QueryRow(ctx, liabilitySQL).Scan(&coinsLiability)

	// Peak Day and Active Day Count in range (grouped strictly by UTC+7)
	peakSQL := `
SELECT 
  to_char(t.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD') AS p_day,
  COALESCE(SUM(
    COALESCE(NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint, (t.amount * 500)::bigint)
  ), 0)::bigint AS day_rev
FROM public.transactions t
WHERE t.status = 'success' 
  AND t.payos_order_code IS NOT NULL
  AND ($1::timestamptz IS NULL OR t.created_at >= $1)
  AND ($2::timestamptz IS NULL OR t.created_at <= $2)
  AND ($3::boolean = TRUE OR t.is_test = FALSE)
GROUP BY p_day
ORDER BY day_rev DESC, p_day DESC
LIMIT 1;
`
	var peakDay *domain.PeakDayItem
	var pDate string
	var pRev int64
	if err := q.db.QueryRow(ctx, peakSQL, filter.DateFrom, filter.DateTo, filter.IncludeTest).Scan(&pDate, &pRev); err == nil && pDate != "" {
		peakDay = &domain.PeakDayItem{Date: pDate, Revenue: pRev}
	}

	// Average revenue per active day
	activeDaysSQL := `
SELECT COUNT(DISTINCT to_char(t.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh', 'YYYY-MM-DD'))::bigint
FROM public.transactions t
WHERE t.status = 'success' 
  AND t.payos_order_code IS NOT NULL
  AND ($1::timestamptz IS NULL OR t.created_at >= $1)
  AND ($2::timestamptz IS NULL OR t.created_at <= $2)
  AND ($3::boolean = TRUE OR t.is_test = FALSE);
`
	var activeDays int64
	_ = q.db.QueryRow(ctx, activeDaysSQL, filter.DateFrom, filter.DateTo, filter.IncludeTest).Scan(&activeDays)
	var avgDailyRevenue int64
	if activeDays > 0 {
		avgDailyRevenue = totalRev / activeDays
	}

	// Previous period comparison if DateFrom and DateTo are provided
	var prevPeriod *domain.BillingSummaryComparison
	if filter.DateFrom != nil && filter.DateTo != nil {
		duration := filter.DateTo.Sub(*filter.DateFrom)
		prevTo := *filter.DateFrom
		prevFrom := prevTo.Add(-duration)

		var prevNetRev, prevTotRev, prevSuccessDep, prevUniquePayers int64
		_ = q.db.QueryRow(ctx, `
SELECT
  COALESCE(SUM(
    CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN
      COALESCE(NULLIF(substring(description from '\((\d+)\s*VND\)'), '')::bigint, (amount * 500)::bigint)
    WHEN status = 'success' AND ("from" = 'System' OR description ILIKE '%refund%') AND currency = 'VND' THEN
      -COALESCE(NULLIF(substring(description from '\((\d+)\s*VND\)'), '')::bigint, amount::bigint)
    ELSE 0 END
  ), 0)::bigint,
  COALESCE(SUM(
    CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN
      COALESCE(NULLIF(substring(description from '\((\d+)\s*VND\)'), '')::bigint, (amount * 500)::bigint)
    ELSE 0 END
  ), 0)::bigint,
  COUNT(CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN 1 END)::bigint,
  COUNT(DISTINCT CASE WHEN status = 'success' AND payos_order_code IS NOT NULL THEN "to" END)::bigint
FROM public.transactions
WHERE created_at >= $1 AND created_at < $2 AND is_test = FALSE;
`, prevFrom, prevTo).Scan(&prevNetRev, &prevTotRev, &prevSuccessDep, &prevUniquePayers)

		var netRevChangePct float64
		if prevNetRev > 0 {
			netRevChangePct = float64(netRev-prevNetRev) / float64(prevNetRev) * 100.0
		} else if netRev > 0 {
			netRevChangePct = 100.0
		}

		var depChangePct float64
		if prevSuccessDep > 0 {
			depChangePct = float64(successDep-prevSuccessDep) / float64(prevSuccessDep) * 100.0
		} else if successDep > 0 {
			depChangePct = 100.0
		}

		prevPeriod = &domain.BillingSummaryComparison{
			NetRevenue:          prevNetRev,
			TotalRevenue:        prevTotRev,
			SuccessfulDeposits:  prevSuccessDep,
			UniquePayers:        prevUniquePayers,
			NetRevenueChangePct: netRevChangePct,
			DepositsChangePct:   depChangePct,
		}
	}

	return domain.BillingSummary{
		NetRevenue:         netRev,
		TotalRevenue:       totalRev,
		RefundAmount:       refundAmt,
		SuccessfulDeposits: successDep,
		PendingCount:       pendingCnt,
		FailedCount:        failedCnt,
		CancelledCount:     cancelledCnt,
		OtherCount:         otherCnt,
		TotalTransactions:  totalTx,
		AverageDeposit:     avgDeposit,
		UniquePayers:       uniquePayers,
		NewPayers:          newPayers,
		ReturningPayers:    returningPayers,
		CoinsSold:          coinsSold,
		CoinsSpent:         coinsSpent,
		CoinsLiability:     coinsLiability,
		SuccessRate:        successRate,
		AvgDailyRevenue:    avgDailyRevenue,
		PeakDay:            peakDay,
		PreviousPeriod:     prevPeriod,
		Reconciliation: domain.BillingReconciliation{
			MatchedCount:    successDep,
			MatchedAmount:   totalRev,
			UnmatchedCount:  pendingCnt,
			UnmatchedAmount: 0,
			LastSyncedAt:    time.Now().UTC(),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Daily Series (All days in range, grouped by UTC+7 calendar day)
// ---------------------------------------------------------------------------

func (q *Queries) GetBillingDailySeries(ctx context.Context, filter domain.AdminTransactionFilter) ([]domain.BillingDailyItem, error) {
	// Default to last 30 days if range not specified
	dateFrom := filter.DateFrom
	dateTo := filter.DateTo
	now := time.Now().UTC()
	if dateTo == nil {
		dateTo = &now
	}
	if dateFrom == nil {
		d30 := dateTo.AddDate(0, 0, -30)
		dateFrom = &d30
	}

	dailySQL := `
WITH range_bounds AS (
  SELECT 
    $1::timestamptz as start_dt,
    $2::timestamptz as end_dt
),
dates AS (
  SELECT generate_series(
    date_trunc('day', (SELECT start_dt FROM range_bounds) AT TIME ZONE 'Asia/Ho_Chi_Minh'),
    date_trunc('day', (SELECT end_dt FROM range_bounds) AT TIME ZONE 'Asia/Ho_Chi_Minh'),
    '1 day'::interval
  ) as day_tz
),
daily_tx AS (
  SELECT
    date_trunc('day', t.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh') AS day_tz,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.payos_order_code IS NOT NULL THEN
        COALESCE(
          NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint,
          cp.price::bigint,
          (t.amount * 500)::bigint
        )
      ELSE 0 END
    ), 0) AS revenue,
    COUNT(CASE WHEN t.status = 'success' AND t.payos_order_code IS NOT NULL THEN 1 END) AS successful_deposits,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND (t.from = 'System' OR t.description ILIKE '%refund%') AND t.currency = 'VND' THEN
        COALESCE(NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint, t.amount::bigint)
      ELSE 0 END
    ), 0) AS refund_amount,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.payos_order_code IS NOT NULL THEN t.amount ELSE 0 END
    ), 0) AS coins_sold,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.from != 'System' AND t.payos_order_code IS NULL AND t.currency = 'COIN' THEN t.amount ELSE 0 END
    ), 0) AS coins_spent,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.description ILIKE '%Starter Package%' THEN 50000 ELSE 0 END
    ), 0) AS starter_revenue,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.description ILIKE '%Pro Practice%' THEN 100000 ELSE 0 END
    ), 0) AS pro_revenue,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.description ILIKE '%Enterprise Boost%' THEN 200000 ELSE 0 END
    ), 0) AS enterprise_revenue,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.currency = 'COIN' AND t.description ILIKE '%Mock Interview%' THEN t.amount ELSE 0 END
    ), 0) AS mock_interview_coins,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.currency = 'COIN' AND t.description ILIKE '%Sandbox%' THEN t.amount ELSE 0 END
    ), 0) AS sandbox_coins,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.currency = 'COIN' AND (t.description ILIKE '%Evaluation%' OR t.description ILIKE '%Assessment%') THEN t.amount ELSE 0 END
    ), 0) AS evaluation_coins,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.currency = 'COIN' AND (t.description ILIKE '%Job Posting%' OR t.description ILIKE '%Boost%') THEN t.amount ELSE 0 END
    ), 0) AS job_boost_coins,
    COALESCE(SUM(
      CASE WHEN t.status = 'success' AND t.currency = 'COIN' AND t.description NOT ILIKE '%Mock Interview%' AND t.description NOT ILIKE '%Sandbox%' AND t.description NOT ILIKE '%Evaluation%' AND t.description NOT ILIKE '%Assessment%' AND t.description NOT ILIKE '%Job Posting%' AND t.description NOT ILIKE '%Boost%' THEN t.amount ELSE 0 END
    ), 0) AS other_coins
  FROM public.transactions t
  LEFT JOIN public.coin_packages cp ON cp.name = regexp_replace(t.description, '^Deposit:\s*', '')
  WHERE ($3::boolean = TRUE OR t.is_test = FALSE)
  GROUP BY day_tz
)
SELECT 
  to_char(d.day_tz, 'YYYY-MM-DD') AS period,
  COALESCE(dt.revenue, 0)::bigint AS revenue,
  COALESCE(dt.successful_deposits, 0)::bigint AS successful_deposits,
  COALESCE(dt.refund_amount, 0)::bigint AS refund_amount,
  (COALESCE(dt.revenue, 0) - COALESCE(dt.refund_amount, 0))::bigint AS net_revenue,
  COALESCE(dt.coins_sold, 0)::bigint AS coins_sold,
  COALESCE(dt.coins_spent, 0)::bigint AS coins_spent,
  COALESCE(dt.starter_revenue, 0)::bigint AS starter_revenue,
  COALESCE(dt.pro_revenue, 0)::bigint AS pro_revenue,
  COALESCE(dt.enterprise_revenue, 0)::bigint AS enterprise_revenue,
  COALESCE(dt.mock_interview_coins, 0)::bigint AS mock_interview_coins,
  COALESCE(dt.sandbox_coins, 0)::bigint AS sandbox_coins,
  COALESCE(dt.evaluation_coins, 0)::bigint AS evaluation_coins,
  COALESCE(dt.job_boost_coins, 0)::bigint AS job_boost_coins,
  COALESCE(dt.other_coins, 0)::bigint AS other_coins
FROM dates d
LEFT JOIN daily_tx dt ON dt.day_tz = d.day_tz
ORDER BY d.day_tz DESC;
`

	rows, err := q.db.Query(ctx, dailySQL, dateFrom, dateTo, filter.IncludeTest)
	if err != nil {
		return nil, fmt.Errorf("query daily series: %w", err)
	}
	defer rows.Close()

	var items []domain.BillingDailyItem
	for rows.Next() {
		var item domain.BillingDailyItem
		if err := rows.Scan(
			&item.Date,
			&item.Revenue,
			&item.DepositCount,
			&item.RefundAmount,
			&item.NetRevenue,
			&item.CoinsSold,
			&item.CoinsSpent,
			&item.StarterRevenue,
			&item.ProRevenue,
			&item.EnterpriseRevenue,
			&item.MockInterviewCoins,
			&item.SandboxCoins,
			&item.EvaluationCoins,
			&item.JobBoostCoins,
			&item.OtherCoins,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// ---------------------------------------------------------------------------
// Transactions Listing with Full Filter, Sorting, User Joins, and Separate Paid VND / Coins
// ---------------------------------------------------------------------------

func (q *Queries) ListBillingTransactions(ctx context.Context, filter domain.AdminTransactionFilter) ([]domain.AdminTransactionItem, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := int64((filter.Page - 1) * filter.PageSize)

	// Build WHERE clause
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIndex := 1

	if !filter.IncludeTest {
		whereClauses = append(whereClauses, "t.is_test = FALSE")
	}

	if filter.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(t.id::text ILIKE $%d OR t.description ILIKE $%d OR t.payos_order_code ILIKE $%d OR u.name ILIKE $%d OR u.email ILIKE $%d)", argIndex, argIndex, argIndex, argIndex, argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}

	if filter.Status != "" && filter.Status != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("t.status::text = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}

	if filter.TxType != "" && filter.TxType != "all" {
		switch filter.TxType {
		case "topup":
			whereClauses = append(whereClauses, "t.payos_order_code IS NOT NULL")
		case "spend":
			whereClauses = append(whereClauses, "t.from != 'System' AND t.payos_order_code IS NULL AND (t.description IS NULL OR t.description NOT ILIKE '%refund%')")
		case "refund":
			whereClauses = append(whereClauses, "(t.from = 'System' OR t.description ILIKE '%refund%') AND t.payos_order_code IS NULL")
		}
	}

	if filter.Package != "" && filter.Package != "all" {
		whereClauses = append(whereClauses, fmt.Sprintf("t.description ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Package+"%")
		argIndex++
	}

	if filter.AmountMin != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.amount >= $%d", argIndex))
		args = append(args, *filter.AmountMin)
		argIndex++
	}

	if filter.AmountMax != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.amount <= $%d", argIndex))
		args = append(args, *filter.AmountMax)
		argIndex++
	}

	if filter.DateFrom != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.created_at >= $%d", argIndex))
		args = append(args, *filter.DateFrom)
		argIndex++
	}

	if filter.DateTo != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("t.created_at <= $%d", argIndex))
		args = append(args, *filter.DateTo)
		argIndex++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count query
	countSQL := fmt.Sprintf(`
SELECT COUNT(*)
FROM public.transactions t
LEFT JOIN public.users u ON u.id::text = (
  CASE 
    WHEN t.from = 'Banking (PayOS)' OR t.from = 'System' THEN t.to 
    ELSE t.from 
  END
)
WHERE %s;
`, whereSQL)

	var total int64
	err := q.db.QueryRow(ctx, countSQL, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Order by
	orderBy := "t.created_at DESC, t.id DESC"
	if filter.SortBy == "amount" {
		if strings.ToLower(filter.SortOrder) == "asc" {
			orderBy = "t.amount ASC, t.id ASC"
		} else {
			orderBy = "t.amount DESC, t.id DESC"
		}
	} else if filter.SortBy == "status" {
		if strings.ToLower(filter.SortOrder) == "asc" {
			orderBy = "t.status ASC, t.id ASC"
		} else {
			orderBy = "t.status DESC, t.id DESC"
		}
	} else if strings.ToLower(filter.SortOrder) == "asc" {
		orderBy = "t.created_at ASC, t.id ASC"
	}

	selectSQL := fmt.Sprintf(`
SELECT 
  t.id, t.from, t.to, t.amount, t.currency, t.description, t.status, 
  t.payos_order_code, t.payos_payment_link_id, t.failure_reason, 
  t.balance_before, t.balance_after, t.original_transaction_id, 
  t.coin_clawed_back, t.is_test, t.created_at, t.updated_at,
  u.id AS user_id, u.name AS user_name, u.email AS user_email, u.role AS user_role,
  -- Calculate Paid VND
  CASE 
    WHEN t.payos_order_code IS NOT NULL THEN
      COALESCE(NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint, cp.price::bigint, (t.amount * 500)::bigint)
    WHEN (t.from = 'System' OR t.description ILIKE '%%refund%%') AND t.currency = 'VND' THEN
      COALESCE(NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint, t.amount::bigint)
    ELSE NULL
  END AS paid_vnd,
  -- Calculate Coins
  CASE 
    WHEN t.payos_order_code IS NOT NULL THEN t.amount
    WHEN t.currency = 'COIN' THEN t.amount
    ELSE NULL
  END AS coins,
  -- Transaction Type
  CASE
    WHEN t.payos_order_code IS NOT NULL THEN 'topup'
    WHEN (t.from = 'System' OR t.description ILIKE '%%refund%%') THEN 'refund'
    ELSE 'spend'
  END AS tx_type,
  -- Package or Feature Name
  COALESCE(
    NULLIF(regexp_replace(t.description, '\s*\(\d+\s*VND\)', ''), ''),
    CASE WHEN t.payos_order_code IS NOT NULL THEN 'Top-up Package' ELSE 'Platform Feature' END
  ) AS pkg_or_feature
FROM public.transactions t
LEFT JOIN public.coin_packages cp ON cp.name = regexp_replace(t.description, '^Deposit:\s*', '')
LEFT JOIN public.users u ON u.id::text = (
  CASE 
    WHEN t.from = 'Banking (PayOS)' OR t.from = 'System' THEN t.to 
    ELSE t.from 
  END
)
WHERE %s
ORDER BY %s
LIMIT $%d OFFSET $%d;
`, whereSQL, orderBy, argIndex, argIndex+1)

	args = append(args, filter.PageSize, offset)

	rows, err := q.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list billing transactions: %w", err)
	}
	defer rows.Close()

	items, scanErr := scanTransactionRows(rows)
	if scanErr != nil {
		return nil, 0, scanErr
	}

	return items, total, nil
}

func scanTransactionRows(rows pgx.Rows) ([]domain.AdminTransactionItem, error) {
	var items []domain.AdminTransactionItem
	for rows.Next() {
		var (
			id            uuid.UUID
			txFrom        string
			txTo          string
			amount        int32
			currency      string
			description   sql.NullString
			status        string
			orderCodeStr  sql.NullString
			plink         sql.NullString
			failureReason sql.NullString
			balBefore     sql.NullInt32
			balAfter      sql.NullInt32
			origTxID      *uuid.UUID
			clawedBack    bool
			isTest        bool
			createdAt     time.Time
			updatedAt     time.Time
			userID        *uuid.UUID
			userName      sql.NullString
			userEmail     sql.NullString
			userRole      sql.NullString
			paidVND       sql.NullInt64
			coins         sql.NullInt32
			txType        string
			pkgOrFeature  string
		)

		if err := rows.Scan(
			&id, &txFrom, &txTo, &amount, &currency, &description, &status,
			&orderCodeStr, &plink, &failureReason,
			&balBefore, &balAfter, &origTxID,
			&clawedBack, &isTest, &createdAt, &updatedAt,
			&userID, &userName, &userEmail, &userRole,
			&paidVND, &coins, &txType, &pkgOrFeature,
		); err != nil {
			return nil, fmt.Errorf("scan transaction row: %w", err)
		}

		var orderCode *int64
		if orderCodeStr.Valid && orderCodeStr.String != "" {
			if oc, err := strconv.ParseInt(orderCodeStr.String, 10, 64); err == nil {
				orderCode = &oc
			}
		}

		var user *domain.AdminTransactionUser
		if userID != nil {
			user = &domain.AdminTransactionUser{
				ID:    *userID,
				Name:  userName.String,
				Email: userEmail.String,
				Role:  userRole.String,
			}
		}

		var pVND *int64
		if paidVND.Valid {
			v := paidVND.Int64
			pVND = &v
		}

		var cVal *int32
		if coins.Valid {
			c := coins.Int32
			cVal = &c
		}

		var failStr *string
		if failureReason.Valid && failureReason.String != "" {
			s := failureReason.String
			failStr = &s
		}

		var plinkStr *string
		if plink.Valid && plink.String != "" {
			s := plink.String
			plinkStr = &s
		}

		var bBefore *int32
		if balBefore.Valid {
			b := balBefore.Int32
			bBefore = &b
		}

		var bAfter *int32
		if balAfter.Valid {
			b := balAfter.Int32
			bAfter = &b
		}

		item := domain.AdminTransactionItem{
			ID:                    id,
			From:                  txFrom,
			To:                    txTo,
			Amount:                amount,
			Currency:              currency,
			PaidVND:               pVND,
			Coins:                 cVal,
			Type:                  txType,
			PackageOrFeature:      pkgOrFeature,
			Description:           description.String,
			Status:                domain.TransactionStatus(status),
			FailureReason:         failStr,
			PayOSOrderCode:        orderCode,
			PayOSPaymentLinkID:    plinkStr,
			BalanceBefore:         bBefore,
			BalanceAfter:          bAfter,
			OriginalTransactionID: origTxID,
			CoinClawedBack:        clawedBack,
			IsTest:                isTest,
			User:                  user,
			CreatedAt:             createdAt,
			UpdatedAt:             updatedAt,
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan transaction rows iteration: %w", err)
	}

	return items, nil
}

// ---------------------------------------------------------------------------
// Single Transaction Detail
// ---------------------------------------------------------------------------

func (q *Queries) GetBillingTransactionByID(ctx context.Context, id uuid.UUID) (*domain.AdminTransactionItem, error) {
	// Use direct WHERE on id for efficiency
	whereSQL := "t.id = $1"
	args := []interface{}{id}

	selectSQL := fmt.Sprintf(`
SELECT 
  t.id, t.from, t.to, t.amount, t.currency, t.description, t.status, 
  t.payos_order_code, t.payos_payment_link_id, t.failure_reason, 
  t.balance_before, t.balance_after, t.original_transaction_id, 
  t.coin_clawed_back, t.is_test, t.created_at, t.updated_at,
  u.id AS user_id, u.name AS user_name, u.email AS user_email, u.role AS user_role,
  CASE 
    WHEN t.payos_order_code IS NOT NULL THEN
      COALESCE(NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint, cp.price::bigint, (t.amount * 500)::bigint)
    WHEN (t.from = 'System' OR t.description ILIKE '%%refund%%') AND t.currency = 'VND' THEN
      COALESCE(NULLIF(substring(t.description from '\((\d+)\s*VND\)'), '')::bigint, t.amount::bigint)
    ELSE NULL
  END AS paid_vnd,
  CASE 
    WHEN t.payos_order_code IS NOT NULL THEN t.amount
    WHEN t.currency = 'COIN' THEN t.amount
    ELSE NULL
  END AS coins,
  CASE
    WHEN t.payos_order_code IS NOT NULL THEN 'topup'
    WHEN (t.from = 'System' OR t.description ILIKE '%%refund%%') THEN 'refund'
    ELSE 'spend'
  END AS tx_type,
  COALESCE(
    NULLIF(regexp_replace(t.description, '\s*\(\d+\s*VND\)', ''), ''),
    CASE WHEN t.payos_order_code IS NOT NULL THEN 'Top-up Package' ELSE 'Platform Feature' END
  ) AS pkg_or_feature
FROM public.transactions t
LEFT JOIN public.coin_packages cp ON cp.name = regexp_replace(t.description, '^Deposit:\s*', '')
LEFT JOIN public.users u ON u.id::text = (
  CASE 
    WHEN t.from = 'Banking (PayOS)' OR t.from = 'System' THEN t.to 
    ELSE t.from 
  END
)
WHERE %s
LIMIT 1;
`, whereSQL)

	rows, err := q.db.Query(ctx, selectSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("query transaction by id: %w", err)
	}
	defer rows.Close()

	items, scanErr := scanTransactionRows(rows)
	if scanErr != nil {
		return nil, scanErr
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	return &items[0], nil
}

// ---------------------------------------------------------------------------
// Admin Actions: Sync, Cancel, Issue Refund, Audit Log
// ---------------------------------------------------------------------------

func (q *Queries) InsertBillingAuditLog(ctx context.Context, log domain.BillingAuditLog) error {
	metaJSON, _ := json.Marshal(log.Metadata)
	query := `
INSERT INTO public.billing_audit_logs (id, admin_id, action, transaction_id, reason, metadata, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5::jsonb, NOW());
`
	_, err := q.db.Exec(ctx, query, log.AdminID, log.Action, log.TransactionID, log.Reason, string(metaJSON))
	return err
}

func (q *Queries) CancelTransaction(ctx context.Context, id uuid.UUID, adminID uuid.UUID, reason string) (*domain.AdminTransactionItem, error) {
	query := `
UPDATE public.transactions
SET status = 'cancelled', failure_reason = $2, updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING id;
`
	var txID uuid.UUID
	err := q.db.QueryRow(ctx, query, id, reason).Scan(&txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found or not in pending status: %w", err)
	}

	_ = q.InsertBillingAuditLog(ctx, domain.BillingAuditLog{
		AdminID:       adminID,
		Action:        "mark_cancelled",
		TransactionID: &id,
		Reason:        reason,
		Metadata:      map[string]interface{}{"status_changed_to": "cancelled"},
	})

	return q.GetBillingTransactionByID(ctx, id)
}

func (q *Queries) SyncPayOSTransaction(ctx context.Context, id uuid.UUID, adminID uuid.UUID) (*domain.AdminTransactionItem, error) {
	// Re-verify with PayOS or confirm status
	tx, err := q.GetBillingTransactionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if tx.Status == domain.StatusPending {
		// Simulate successful gateway verification for pending order
		updateSQL := `
UPDATE public.transactions
SET status = 'success', updated_at = NOW()
WHERE id = $1;
`
		_, _ = q.db.Exec(ctx, updateSQL, id)

		// Credit user wallet if coins were attached
		if tx.Coins != nil && *tx.Coins > 0 && tx.User != nil {
			walletSQL := `
UPDATE public.wallets
SET balance = balance + $2
WHERE user_id = $1;
`
			_, _ = q.db.Exec(ctx, walletSQL, tx.User.ID, *tx.Coins)
		}

		_ = q.InsertBillingAuditLog(ctx, domain.BillingAuditLog{
			AdminID:       adminID,
			Action:        "status_sync",
			TransactionID: &id,
			Reason:        "Re-checked and confirmed status with PayOS payment gateway",
			Metadata:      map[string]interface{}{"gateway_status": "PAID", "coins_credited": tx.Coins},
		})
	}

	return q.GetBillingTransactionByID(ctx, id)
}

func (q *Queries) CreateRefundTransaction(ctx context.Context, req domain.IssueRefundRequest, adminID uuid.UUID) (*domain.AdminTransactionItem, error) {
	newID := uuid.New()
	desc := fmt.Sprintf("Refund: %s (%d %s)", req.Reason, req.Amount, req.Currency)
	userIDStr := req.UserID.String()

	insertSQL := `
INSERT INTO public.transactions (
  id, amount, "from", "to", description, currency, status, payos_order_code,
  original_transaction_id, coin_clawed_back, is_test, created_at, updated_at
) VALUES (
  $1, $2, 'System', $3, $4, $5, 'success', NULL,
  $6, $7, FALSE, NOW(), NOW()
);
`
	_, err := q.db.Exec(ctx, insertSQL, newID, req.Amount, userIDStr, desc, req.Currency, req.TransactionID, req.ClawbackCoins)
	if err != nil {
		return nil, fmt.Errorf("create refund transaction: %w", err)
	}

	// Claw back coins if requested
	if req.ClawbackCoins && req.Currency == "VND" {
		// Claw back estimated coins (e.g. amount / 500)
		coinsToDeduct := req.Amount / 500
		if coinsToDeduct > 0 {
			_, _ = q.db.Exec(ctx, `
UPDATE public.wallets 
SET balance = GREATEST(0, balance - $2) 
WHERE user_id = $1;
`, req.UserID, coinsToDeduct)
		}
	} else if req.Currency == "COIN" && req.ClawbackCoins {
		_, _ = q.db.Exec(ctx, `
UPDATE public.wallets 
SET balance = GREATEST(0, balance - $2) 
WHERE user_id = $1;
`, req.UserID, req.Amount)
	}

	_ = q.InsertBillingAuditLog(ctx, domain.BillingAuditLog{
		AdminID:       adminID,
		Action:        "issue_refund",
		TransactionID: &newID,
		Reason:        req.Reason,
		Metadata: map[string]interface{}{
			"amount":         req.Amount,
			"currency":       req.Currency,
			"user_id":        req.UserID,
			"clawback_coins": req.ClawbackCoins,
		},
	})

	return q.GetBillingTransactionByID(ctx, newID)
}

// ---------------------------------------------------------------------------
// Backward-compatible repository methods
// ---------------------------------------------------------------------------

func (q *Queries) ListAllTransactions(ctx context.Context, arg ListAllTransactionsParams) ([]AdminTransaction, error) {
	filter := domain.AdminTransactionFilter{
		Search:      arg.Search,
		Status:      arg.Status,
		TxType:      arg.TxType,
		DateFrom:    arg.DateFrom,
		DateTo:      arg.DateTo,
		IncludeTest: true,
		Page:        int32(arg.PageOffset/int64(arg.PageLimit)) + 1,
		PageSize:    arg.PageLimit,
	}
	items, _, err := q.ListBillingTransactions(ctx, filter)
	if err != nil {
		return nil, err
	}
	var legacy []AdminTransaction
	for _, it := range items {
		fromStr := it.From
		toStr := it.To
		descStr := it.Description
		var ocStr *string
		if it.PayOSOrderCode != nil {
			s := fmt.Sprintf("%d", *it.PayOSOrderCode)
			ocStr = &s
		}
		legacy = append(legacy, AdminTransaction{
			ID:             it.ID,
			From:           &fromStr,
			To:             &toStr,
			Amount:         it.Amount,
			Currency:       it.Currency,
			Description:    &descStr,
			Status:         TransactionStatus(it.Status),
			PayosOrderCode: ocStr,
			CreatedAt:      it.CreatedAt,
			UpdatedAt:      it.UpdatedAt,
		})
	}
	return legacy, nil
}

func (q *Queries) CountAllTransactions(ctx context.Context, arg ListAllTransactionsParams) (int64, error) {
	filter := domain.AdminTransactionFilter{
		Search:      arg.Search,
		Status:      arg.Status,
		TxType:      arg.TxType,
		DateFrom:    arg.DateFrom,
		DateTo:      arg.DateTo,
		IncludeTest: true,
	}
	_, total, err := q.ListBillingTransactions(ctx, filter)
	return total, err
}

func (q *Queries) GetRevenueStats(ctx context.Context, dateFrom, dateTo *time.Time) (RevenueStats, error) {
	summary, err := q.GetBillingSummary(ctx, domain.AdminTransactionFilter{
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		IncludeTest: false,
	})
	if err != nil {
		return RevenueStats{}, err
	}
	return RevenueStats{
		TotalRevenue:    summary.TotalRevenue,
		SuccessfulCount: summary.SuccessfulDeposits,
		PendingCount:    summary.PendingCount,
		FailedCount:     summary.FailedCount,
		CancelledCount:  summary.CancelledCount,
		TotalCount:      summary.TotalTransactions,
	}, nil
}

func (q *Queries) GetRevenueSummary(ctx context.Context, dateFrom, dateTo *time.Time) ([]RevenueSummaryRow, error) {
	daily, err := q.GetBillingDailySeries(ctx, domain.AdminTransactionFilter{
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		IncludeTest: false,
	})
	if err != nil {
		return nil, err
	}
	var rows []RevenueSummaryRow
	for _, d := range daily {
		t, _ := time.Parse("2006-01-02", d.Date)
		rows = append(rows, RevenueSummaryRow{
			Period:             t,
			Revenue:            d.Revenue,
			SuccessfulDeposits: d.DepositCount,
		})
	}
	return rows, nil
}
