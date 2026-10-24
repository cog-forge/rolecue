package domain

import (
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
	ID                    uuid.UUID         `json:"id"`
	From                  *uuid.UUID        `json:"from"`
	To                    *uuid.UUID        `json:"to"`
	Amount                int32             `json:"amount"`
	Currency              string            `json:"currency"`
	Description           string            `json:"description"`
	Status                TransactionStatus `json:"status"`
	PayOSOrderCode        *int64            `json:"payos_order_code"`
	PayOSPaymentLinkID    *string           `json:"payos_payment_link_id,omitempty"`
	FailureReason         *string           `json:"failure_reason,omitempty"`
	BalanceBefore         *int32            `json:"balance_before,omitempty"`
	BalanceAfter          *int32            `json:"balance_after,omitempty"`
	OriginalTransactionID *uuid.UUID        `json:"original_transaction_id,omitempty"`
	CoinClawedBack        bool              `json:"coin_clawed_back"`
	IsTest                bool              `json:"is_test"`
	CreatedAt             time.Time         `json:"created_at,omitempty"`
	UpdatedAt             time.Time         `json:"updated_at,omitempty"`
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
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// AdminTransactionUser contains user information joined for admin lists.
type AdminTransactionUser struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

// AdminTransactionFilter holds query parameters for admin transaction listing.
type AdminTransactionFilter struct {
	Search      string
	Status      string
	TxType      string
	Package     string
	AmountMin   *int64
	AmountMax   *int64
	DateFrom    *time.Time
	DateTo      *time.Time
	IncludeTest bool
	SortBy      string
	SortOrder   string
	Page        int32
	PageSize    int32
}

// AdminTransactionItem represents an administrative transaction item with
// full user information, separation of money and coins, and failure reasons.
type AdminTransactionItem struct {
	ID                    uuid.UUID             `json:"id"`
	From                  string                `json:"from"`
	To                    string                `json:"to"`
	Amount                int32                 `json:"amount"`
	Currency              string                `json:"currency"`
	PaidVND               *int64                `json:"paid_vnd"`
	Coins                 *int32                `json:"coins"`
	Type                  string                `json:"type"` // "topup" | "spend" | "refund"
	PackageOrFeature      string                `json:"package_or_feature"`
	Description           string                `json:"description"`
	Status                TransactionStatus     `json:"status"`
	FailureReason         *string               `json:"failure_reason,omitempty"`
	PayOSOrderCode        *int64                `json:"payos_order_code,omitempty"`
	PayOSPaymentLinkID    *string               `json:"payos_payment_link_id,omitempty"`
	BalanceBefore         *int32                `json:"balance_before,omitempty"`
	BalanceAfter          *int32                `json:"balance_after,omitempty"`
	OriginalTransactionID *uuid.UUID            `json:"original_transaction_id,omitempty"`
	CoinClawedBack        bool                  `json:"coin_clawed_back"`
	IsTest                bool                  `json:"is_test"`
	User                  *AdminTransactionUser `json:"user,omitempty"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
}

// AdminTransactionPage wraps a page of admin transactions.
type AdminTransactionPage struct {
	Items    []AdminTransactionItem `json:"items"`
	Total    int64                  `json:"total"`
	Page     int32                  `json:"page"`
	PageSize int32                  `json:"page_size"`
}

// PeakDayItem represents peak revenue day.
type PeakDayItem struct {
	Date    string `json:"date"`
	Revenue int64  `json:"revenue"`
}

// BillingReconciliation contains gateway verification metrics.
type BillingReconciliation struct {
	MatchedCount    int64     `json:"matched_count"`
	MatchedAmount   int64     `json:"matched_amount"`
	UnmatchedCount  int64     `json:"unmatched_count"`
	UnmatchedAmount int64     `json:"unmatched_amount"`
	LastSyncedAt    time.Time `json:"last_synced_at"`
}

// BillingSummaryComparison stores previous period metrics.
type BillingSummaryComparison struct {
	NetRevenue          int64   `json:"net_revenue"`
	TotalRevenue        int64   `json:"total_revenue"`
	SuccessfulDeposits  int64   `json:"successful_deposits"`
	UniquePayers        int64   `json:"unique_payers"`
	NetRevenueChangePct float64 `json:"net_revenue_change_pct"`
	DepositsChangePct   float64 `json:"deposits_change_pct"`
}

// BillingSummary holds aggregated revenue and billing metrics.
type BillingSummary struct {
	NetRevenue         int64                     `json:"net_revenue"`
	TotalRevenue       int64                     `json:"total_revenue"`
	RefundAmount       int64                     `json:"refund_amount"`
	SuccessfulDeposits int64                     `json:"successful_deposits"`
	PendingCount       int64                     `json:"pending_count"`
	FailedCount        int64                     `json:"failed_count"`
	CancelledCount     int64                     `json:"cancelled_count"`
	OtherCount         int64                     `json:"other_count"`
	TotalTransactions  int64                     `json:"total_transactions"`
	AverageDeposit     int64                     `json:"average_deposit"`
	UniquePayers       int64                     `json:"unique_payers"`
	NewPayers          int64                     `json:"new_payers"`
	ReturningPayers    int64                     `json:"returning_payers"`
	CoinsSold          int64                     `json:"coins_sold"`
	CoinsSpent         int64                     `json:"coins_spent"`
	CoinsLiability     int64                     `json:"coins_liability"`
	SuccessRate        float64                   `json:"success_rate"`
	AvgDailyRevenue    int64                     `json:"avg_daily_revenue"`
	PeakDay            *PeakDayItem              `json:"peak_day,omitempty"`
	PreviousPeriod     *BillingSummaryComparison `json:"previous_period,omitempty"`
	Reconciliation     BillingReconciliation     `json:"reconciliation"`
}

// RevenueStats is maintained for backward compatibility.
type RevenueStats struct {
	TotalRevenue    int64 `json:"total_revenue"`
	SuccessfulCount int64 `json:"successful_count"`
	PendingCount    int64 `json:"pending_count"`
	FailedCount     int64 `json:"failed_count"`
	CancelledCount  int64 `json:"cancelled_count"`
	TotalCount      int64 `json:"total_count"`
}

// RevenueSummaryItem represents daily revenue summary (backward compatible).
type RevenueSummaryItem struct {
	Period             time.Time `json:"period"`
	Revenue            int64     `json:"revenue"`
	SuccessfulDeposits int64     `json:"successful_deposits"`
}

// RevenueReport combines stats with daily summary.
type RevenueReport struct {
	Stats   RevenueStats         `json:"stats"`
	Summary []RevenueSummaryItem `json:"summary"`
}

// BillingDailyItem represents every single day in the period (UTC+7 calendar day).
type BillingDailyItem struct {
	Date               string `json:"date"` // "YYYY-MM-DD"
	Revenue            int64  `json:"revenue"`
	DepositCount       int64  `json:"deposit_count"`
	RefundAmount       int64  `json:"refund_amount"`
	NetRevenue         int64  `json:"net_revenue"`
	CoinsSold          int64  `json:"coins_sold"`
	CoinsSpent         int64  `json:"coins_spent"`
	PrevRevenue        *int64 `json:"prev_revenue,omitempty"`
	StarterRevenue     int64  `json:"starter_revenue"`
	ProRevenue         int64  `json:"pro_revenue"`
	EnterpriseRevenue  int64  `json:"enterprise_revenue"`
	MockInterviewCoins int64  `json:"mock_interview_coins"`
	SandboxCoins       int64  `json:"sandbox_coins"`
	EvaluationCoins    int64  `json:"evaluation_coins"`
	JobBoostCoins      int64  `json:"job_boost_coins"`
	OtherCoins         int64  `json:"other_coins"`
}

// BillingAuditLog represents an audit log entry.
type BillingAuditLog struct {
	ID            uuid.UUID              `json:"id"`
	AdminID       uuid.UUID              `json:"admin_id"`
	Action        string                 `json:"action"`
	TransactionID *uuid.UUID             `json:"transaction_id,omitempty"`
	Reason        string                 `json:"reason"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// IssueRefundRequest holds parameters for admin refund creation.
type IssueRefundRequest struct {
	TransactionID *uuid.UUID `json:"transaction_id"`
	UserID        uuid.UUID  `json:"user_id"`
	Amount        int32      `json:"amount"`
	Currency      string     `json:"currency"` // "VND" or "COIN"
	Reason        string     `json:"reason"`
	ClawbackCoins bool       `json:"clawback_coins"`
}

// DepositCheckoutResponse contains the PayOS checkout URL and details.
type DepositCheckoutResponse struct {
	CheckoutURL   string    `json:"checkout_url"`
	OrderCode     int64     `json:"order_code"`
	Amount        int32     `json:"amount"`
	CoinAmount    int32     `json:"coin_amount"`
	TransactionID uuid.UUID `json:"transaction_id"`
}
