package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// TransactionStatus mirrors public.transaction_status postgres enum.
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusSuccess   TransactionStatus = "success"
	StatusFailed    TransactionStatus = "failed"
	StatusCancelled TransactionStatus = "cancelled"
)

// Wallet holds a candidate or recruiter coin balance.
type Wallet struct {
	ID      uuid.UUID `json:"id"`
	UserID  uuid.UUID `json:"user_id"`
	Balance int32     `json:"balance"`
}

// Transaction matches the unified transactions table in the ERD.
type Transaction struct {
	ID             uuid.UUID         `json:"id"`
	From           *uuid.UUID        `json:"from"`
	To             *uuid.UUID        `json:"to"`
	Amount         int32             `json:"amount"`
	Currency       string            `json:"currency"`
	Description    string            `json:"description"`
	Status         TransactionStatus `json:"status"`
	PayOSOrderCode *int64            `json:"payos_order_code"`
	CreatedAt      time.Time         `json:"created_at"`
}

// CoinPackage represents a top-up package.
type CoinPackage struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	PriceVND    int32     `json:"price_vnd"`
	Description string    `json:"description,omitempty"`
	CoinAmount  int32     `json:"coin_amount"`
	Currency    string    `json:"currency"`
	IsActive    bool      `json:"is_active"`
}

// DepositCheckoutResponse contains the PayOS checkout URL and details.
type DepositCheckoutResponse struct {
	CheckoutURL   string    `json:"checkout_url"`
	OrderCode     int64     `json:"order_code"`
	Amount        int32     `json:"amount"`
	CoinAmount    int32     `json:"coin_amount"`
	TransactionID uuid.UUID `json:"transaction_id"`
}

// AdminTransactionFilter represents validated read-only query filters for Admin Billing & Revenue endpoints.
// Both the Payment Transactions page and the Revenue Reports page share this exact filter model.
// The date window is [DateFrom, DateTo) on created_at; either end may be open.
type AdminTransactionFilter struct {
	Page        int
	PageSize    int
	Search      string
	Status      string // "", "pending", "success", "failed", "cancelled"
	Type        string // "", "topup", "spend", "refund"
	PackageName string
	AmountMin   *int64
	AmountMax   *int64
	DateFrom    *time.Time
	DateTo      *time.Time
	SortBy      string // "created_at", "id", "amount", "coins", "status"
	SortOrder   string // "asc", "desc"
}

// AdminTransactionItem represents a single read-only ledger row enriched with user and package details.
type AdminTransactionItem struct {
	ID               uuid.UUID         `json:"id"`
	From             string            `json:"from"`
	To               string            `json:"to"`
	UserID           string            `json:"user_id"`
	UserName         string            `json:"user_name"`
	UserEmail        string            `json:"user_email"`
	Type             string            `json:"type"`
	PackageOrFeature string            `json:"package_or_feature"`
	PaidVND          int64             `json:"paid_vnd"`
	CoinAmount       int64             `json:"coin_amount"`
	Amount           int32             `json:"amount"`
	Currency         string            `json:"currency"`
	Description      string            `json:"description"`
	Status           TransactionStatus `json:"status"`
	PayOSOrderCode   *int64            `json:"payos_order_code"`
	CreatedAt        time.Time         `json:"created_at"`
}

// AdminTransactionPage represents a paginated read-only list of AdminTransactionItem rows.
type AdminTransactionPage struct {
	Items    []AdminTransactionItem `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// PackageRevenueItem represents actual PayOS deposit revenue grouped by package name.
type PackageRevenueItem struct {
	PackageName  string `json:"package_name"`
	RevenueVND   int64  `json:"revenue_vnd"`
	DepositCount int64  `json:"deposit_count"`
	CoinsSold    int64  `json:"coins_sold"`
}

// FeatureCoinItem represents actual internal coin consumption grouped by feature description.
type FeatureCoinItem struct {
	FeatureName string `json:"feature_name"`
	CoinsSpent  int64  `json:"coins_spent"`
	SpendCount  int64  `json:"spend_count"`
}

// DepositSuccessRate returns the share of PayOS deposit attempts that succeeded, as a percentage
// rounded to one decimal. Internal coin rows are always 'success', so they are not part of the base.
func DepositSuccessRate(successfulDeposits, totalDeposits int64) float64 {
	if totalDeposits <= 0 {
		return 0
	}
	return math.Round(float64(successfulDeposits)/float64(totalDeposits)*1000) / 10
}

// BillingSummary is the shared read-only financial summary used by both Payment Transactions and Revenue Reports.
// Per KAN-192, TotalRevenue only sums transactions with status = 'success' and non-empty payos_order_code.
type BillingSummary struct {
	TotalRevenue       int64                `json:"total_revenue"`
	SuccessfulDeposits int64                `json:"successful_deposits"`
	TotalDeposits      int64                `json:"total_deposits"`
	SuccessCount       int64                `json:"success_count"`
	PendingCount       int64                `json:"pending_count"`
	FailedCount        int64                `json:"failed_count"`
	CancelledCount     int64                `json:"cancelled_count"`
	TotalTransactions  int64                `json:"total_transactions"`
	AverageDeposit     int64                `json:"average_deposit"`
	UniquePayers       int64                `json:"unique_payers"`
	CoinsSold          int64                `json:"coins_sold"`
	CoinsSpent         int64                `json:"coins_spent"`
	CoinsRefunded      int64                `json:"coins_refunded"`
	CoinsNet           int64                `json:"coins_net"`
	SuccessRate        float64              `json:"success_rate"` // percent of PayOS deposit attempts that succeeded
	PackageBreakdown   []PackageRevenueItem `json:"package_breakdown"`
	FeatureBreakdown   []FeatureCoinItem    `json:"feature_breakdown"`
}

// Revenue report (KAN-192 iteration 2): a window compared with the same dates a year earlier or with
// the period of the same length just before it. Revenue keeps the BillingSummary definition.
const (
	CompareSamePeriodLastYear = "previous_year"
	ComparePreviousPeriod     = "previous_period"
)

// RevenueReportRequest is a validated [From, To) window and the comparison to draw.
type RevenueReportRequest struct {
	From    time.Time
	To      time.Time
	Compare string
}

// RevenueWindow is a [From, To) window in UTC.
type RevenueWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// RevenuePeriodMetrics are the headline figures of one window.
type RevenuePeriodMetrics struct {
	RevenueVND         int64   `json:"revenue_vnd"`
	SuccessfulTopups   int64   `json:"successful_topups"`
	TopupAttempts      int64   `json:"topup_attempts"`       // settled PayOS attempts: paid, failed or cancelled
	PaymentSuccessRate float64 `json:"payment_success_rate"` // percent of settled PayOS attempts that were paid
	PayingUsers        int64   `json:"paying_users"`
	NewPayers          int64   `json:"new_payers"` // paying users whose first ever successful top-up is in the window
	AverageOrderVND    int64   `json:"average_order_vnd"`
	CoinsSold          int64   `json:"coins_sold"`
	CoinsSpent         int64   `json:"coins_spent"`
	CoinsRefunded      int64   `json:"coins_refunded"`
}

// RevenueSeriesPoint is one bucket of the window and the bucket at the same index of the comparison.
type RevenueSeriesPoint struct {
	Start                time.Time `json:"start"`
	ComparisonStart      time.Time `json:"comparison_start"`
	RevenueVND           int64     `json:"revenue_vnd"`
	ComparisonRevenueVND int64     `json:"comparison_revenue_vnd"`
}

// RevenueMixItem is revenue from one role or one package in both windows.
type RevenueMixItem struct {
	Key                  string `json:"key"`
	RevenueVND           int64  `json:"revenue_vnd"`
	ComparisonRevenueVND int64  `json:"comparison_revenue_vnd"`
	Topups               int64  `json:"topups"`
}

// CoinFeatureItem is coins spent on one feature in both windows.
type CoinFeatureItem struct {
	Feature         string `json:"feature"` // practice_interview, interview_slots, avatar_storage, other
	Coins           int64  `json:"coins"`
	ComparisonCoins int64  `json:"comparison_coins"`
	Charges         int64  `json:"charges"`
}

// RevenueReport is the admin revenue overview.
type RevenueReport struct {
	Compare          string               `json:"compare"`
	Granularity      string               `json:"granularity"` // day or month, in Vietnam time
	Window           RevenueWindow        `json:"window"`
	ComparisonWindow RevenueWindow        `json:"comparison_window"`
	Current          RevenuePeriodMetrics `json:"current"`
	Comparison       RevenuePeriodMetrics `json:"comparison"`
	Series           []RevenueSeriesPoint `json:"series"`
	ByRole           []RevenueMixItem     `json:"by_role"`
	ByPackage        []RevenueMixItem     `json:"by_package"`
	SpendByFeature   []CoinFeatureItem    `json:"spend_by_feature"`
	// Coins users hold now: prepaid, not yet used, and not revenue. Its VND value uses the average price
	// paid per coin over all time.
	OutstandingCoins    int64 `json:"outstanding_coins"`
	OutstandingValueVND int64 `json:"outstanding_value_vnd"`
}

// RevenueTopup is one successful PayOS top-up as the revenue report reads it.
type RevenueTopup struct {
	CreatedAt   time.Time
	UserRole    string // candidate, recruiter or "" when the user is unknown
	PackageName string
	PaidVND     int64
}

// RevenuePeriodData is what the repository returns for one window.
type RevenuePeriodData struct {
	Summary   BillingSummary
	Topups    []RevenueTopup
	NewPayers int64
}
