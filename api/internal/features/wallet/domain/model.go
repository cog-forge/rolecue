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
	ID             uuid.UUID         `json:"id"`
	From           *uuid.UUID        `json:"from"`
	To             *uuid.UUID        `json:"to"`
	Amount         int32             `json:"amount"`
	Currency       string            `json:"currency"`
	Description    string            `json:"description"`
	Status         TransactionStatus `json:"status"`
	PayOSOrderCode *int64            `json:"payos_order_code"`
	CreatedAt      time.Time         `json:"created_at,omitempty"`
	UpdatedAt      time.Time         `json:"updated_at,omitempty"`
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

// DepositCheckoutResponse contains the PayOS checkout URL and details.
type DepositCheckoutResponse struct {
	CheckoutURL   string    `json:"checkout_url"`
	OrderCode     int64     `json:"order_code"`
	Amount        int32     `json:"amount"`
	CoinAmount    int32     `json:"coin_amount"`
	TransactionID uuid.UUID `json:"transaction_id"`
}
