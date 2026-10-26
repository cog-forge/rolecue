package domain

import "github.com/google/uuid"

// TransactionStatus mirrors public.transaction_status postgres enum.
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusSuccess   TransactionStatus = "success"
	StatusFailed    TransactionStatus = "failed"
	StatusCancelled TransactionStatus = "cancelled"
)

// FrozenSchemaTimestampNotice documents the frozen schema limitation per KAN-191:
// public.transactions in migration 000006_wallet_payment does not include a created_at column.
const FrozenSchemaTimestampNotice = "public.transactions in the frozen schema (000006_wallet_payment) does not store a created_at timestamp; date range parameters are validated in UTC+7 (Asia/Ho_Chi_Minh) and figures reflect all matching ledger records."

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
type AdminTransactionFilter struct {
	Page        int
	PageSize    int
	Search      string
	Status      string // "", "pending", "success", "failed", "cancelled"
	Type        string // "", "topup", "spend", "refund"
	PackageName string
	AmountMin   *int64
	AmountMax   *int64
	DateFrom    string // Validated YYYY-MM-DD in Asia/Ho_Chi_Minh (UTC+7)
	DateTo      string // Validated YYYY-MM-DD in Asia/Ho_Chi_Minh (UTC+7)
	SortBy      string // "id", "amount", "coins", "status"
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
}

// AdminTransactionPage represents a paginated read-only list of AdminTransactionItem rows.
type AdminTransactionPage struct {
	Items            []AdminTransactionItem `json:"items"`
	Total            int64                  `json:"total"`
	Page             int                    `json:"page"`
	PageSize         int                    `json:"page_size"`
	SchemaLimitation string                 `json:"schema_limitation"`
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

// BillingSummary is the shared read-only financial summary used by both Payment Transactions and Revenue Reports.
// Per KAN-192, TotalRevenue only sums transactions with status = 'success' and non-empty payos_order_code.
type BillingSummary struct {
	TotalRevenue       int64                `json:"total_revenue"`
	NetRevenue         int64                `json:"net_revenue"`
	SuccessfulDeposits int64                `json:"successful_deposits"`
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
	SuccessRate        float64              `json:"success_rate"`
	PackageBreakdown   []PackageRevenueItem `json:"package_breakdown"`
	FeatureBreakdown   []FeatureCoinItem    `json:"feature_breakdown"`
	SchemaLimitation   string               `json:"schema_limitation"`
}
