package payos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	payosSDK "github.com/payOSHQ/payos-lib-golang/v2"
)

// Re-exported types from the official payOS SDK
type Options = payosSDK.PayOSOptions
type PaymentLink = payosSDK.PaymentLink
type PaymentLinkStatus = payosSDK.PaymentLinkStatus
type CreatePaymentLinkRequest = payosSDK.CreatePaymentLinkRequest
type CreatePaymentLinkResponse = payosSDK.CreatePaymentLinkResponse
type CancelPaymentLinkRequest = payosSDK.CancelPaymentLinkRequest
type Webhook = payosSDK.Webhook
type SDKWebhookData = payosSDK.WebhookData
type ConfirmWebhookRequest = payosSDK.ConfirmWebhookRequest
type ConfirmWebhookResponse = payosSDK.ConfirmWebhookResponse

const (
	PaymentLinkStatusPending    = payosSDK.PaymentLinkStatusPending
	PaymentLinkStatusCancelled  = payosSDK.PaymentLinkStatusCancelled
	PaymentLinkStatusUnderpaid  = payosSDK.PaymentLinkStatusUnderpaid
	PaymentLinkStatusPaid       = payosSDK.PaymentLinkStatusPaid
	PaymentLinkStatusExpired    = payosSDK.PaymentLinkStatusExpired
	PaymentLinkStatusProcessing = payosSDK.PaymentLinkStatusProcessing
	PaymentLinkStatusFailed     = payosSDK.PaymentLinkStatusFailed
)

type CreateLinkRequest struct {
	OrderCode   int64  `json:"orderCode"`
	Amount      int    `json:"amount"`
	Description string `json:"description"`
	ReturnURL   string `json:"returnUrl"`
	CancelURL   string `json:"cancelUrl"`
}

type CreateLinkResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
	QRCode      string `json:"qrCode"`
	OrderCode   int64  `json:"orderCode"`
}

type WebhookData struct {
	Code    string `json:"code"`
	Desc    string `json:"desc"`
	Success bool   `json:"success"`
	Data    struct {
		OrderCode       int64  `json:"orderCode"`
		Amount          int    `json:"amount"`
		Description     string `json:"description"`
		AccountNumber   string `json:"accountNumber"`
		Reference       string `json:"reference"`
		TransactionTime string `json:"transactionDateTime"`
		PaymentLinkID   string `json:"paymentLinkId"`
		Code            string `json:"code"`
		Desc            string `json:"desc"`
	} `json:"data"`
	Signature string `json:"signature"`
}

type Client struct {
	sdk *payosSDK.PayOS
}

// NewClientWithOptions initializes a PayOS Client using full PayOSOptions.
func NewClientWithOptions(opts *payosSDK.PayOSOptions) (*Client, error) {
	if opts == nil {
		return nil, errors.New("payos: options cannot be nil")
	}
	if strings.TrimSpace(opts.ClientId) == "" {
		return nil, errors.New("payos: clientID is required")
	}
	if strings.TrimSpace(opts.ApiKey) == "" {
		return nil, errors.New("payos: apiKey is required")
	}
	if strings.TrimSpace(opts.ChecksumKey) == "" {
		return nil, errors.New("payos: checksumKey is required")
	}

	sdk, err := payosSDK.NewPayOS(opts)
	if err != nil {
		return nil, fmt.Errorf("payos: initialize sdk: %w", err)
	}

	return &Client{sdk: sdk}, nil
}

// MustNewClientWithOptions initializes a PayOS Client with PayOSOptions or panics on error.
func MustNewClientWithOptions(opts *payosSDK.PayOSOptions) *Client {
	c, err := NewClientWithOptions(opts)
	if err != nil {
		panic(err)
	}
	return c
}

// NewClient initializes a PayOS Client with basic credentials.
func NewClient(clientID, apiKey, checksumKey string) (*Client, error) {
	return NewClientWithOptions(&payosSDK.PayOSOptions{
		ClientId:    clientID,
		ApiKey:      apiKey,
		ChecksumKey: checksumKey,
	})
}

// MustNewClient initializes a PayOS Client with basic credentials or panics on error.
func MustNewClient(clientID, apiKey, checksumKey string) *Client {
	c, err := NewClient(clientID, apiKey, checksumKey)
	if err != nil {
		panic(err)
	}
	return c
}

// SDK returns the underlying official PayOS SDK client instance.
func (c *Client) SDK() *payosSDK.PayOS {
	return c.sdk
}

// PaymentRequests returns the SDK PaymentRequests service.
func (c *Client) PaymentRequests() *payosSDK.PaymentRequests {
	return c.sdk.PaymentRequests
}

// Webhooks returns the SDK Webhooks service.
func (c *Client) Webhooks() *payosSDK.Webhooks {
	return c.sdk.Webhooks
}

// Payouts returns the SDK Payouts service.
func (c *Client) Payouts() *payosSDK.Payouts {
	return c.sdk.Payouts
}

// PayoutsAccount returns the SDK PayoutsAccount service.
func (c *Client) PayoutsAccount() *payosSDK.PayoutsAccount {
	return c.sdk.PayoutsAccount
}

// CreatePaymentLink generates a payment link using PayOS PaymentRequests.Create.
func (c *Client) CreatePaymentLink(ctx context.Context, req CreateLinkRequest) (CreateLinkResponse, error) {
	resp, err := c.sdk.PaymentRequests.Create(ctx, payosSDK.CreatePaymentLinkRequest{
		OrderCode:   req.OrderCode,
		Amount:      req.Amount,
		Description: req.Description,
		ReturnUrl:   req.ReturnURL,
		CancelUrl:   req.CancelURL,
	})
	if err != nil {
		return CreateLinkResponse{}, fmt.Errorf("payos: create payment link: %w", err)
	}

	return CreateLinkResponse{
		CheckoutURL: resp.CheckoutUrl,
		QRCode:      resp.QrCode,
		OrderCode:   resp.OrderCode,
	}, nil
}

// GetPaymentLink retrieves information about an order's payment link.
func (c *Client) GetPaymentLink(ctx context.Context, orderCode int64) (*PaymentLink, error) {
	link, err := c.sdk.PaymentRequests.Get(ctx, orderCode)
	if err != nil {
		return nil, fmt.Errorf("payos: get payment link: %w", err)
	}
	return link, nil
}

// CancelPaymentLink cancels an active payment link.
func (c *Client) CancelPaymentLink(ctx context.Context, orderCode int64, cancellationReason *string) (*PaymentLink, error) {
	link, err := c.sdk.PaymentRequests.Cancel(ctx, orderCode, cancellationReason)
	if err != nil {
		return nil, fmt.Errorf("payos: cancel payment link: %w", err)
	}
	return link, nil
}

// ConfirmWebhook registers/confirms a webhook URL with PayOS.
func (c *Client) ConfirmWebhook(ctx context.Context, webhookURL string) (string, error) {
	result, err := c.sdk.Webhooks.Confirm(ctx, webhookURL)
	if err != nil {
		return "", fmt.Errorf("payos: confirm webhook: %w", err)
	}
	return result, nil
}

// VerifyWebhookData cryptographically verifies webhook signature and decodes payload.
func (c *Client) VerifyWebhookData(body []byte) (*WebhookData, error) {
	var rawMap map[string]interface{}
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return nil, fmt.Errorf("payos: decode webhook body: %w", err)
	}

	_, err := c.sdk.Webhooks.VerifyData(context.Background(), rawMap)
	if err != nil {
		return nil, fmt.Errorf("payos: invalid webhook signature: %w", err)
	}

	var payload WebhookData
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("payos: decode webhook payload: %w", err)
	}

	return &payload, nil
}
