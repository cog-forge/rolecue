package payos

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const baseURL = "https://api-merchant.payos.vn"

type Config struct {
	ClientID    string
	APIKey      string
	ChecksumKey string
}

type Client struct {
	cfg        Config
	httpClient *http.Client
}

func NewClient(clientID, apiKey, checksumKey string) *Client {
	return &Client{
		cfg: Config{
			ClientID:    clientID,
			APIKey:      apiKey,
			ChecksumKey: checksumKey,
		},
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type CreateLinkRequest struct {
	OrderCode   int64  `json:"orderCode"`
	Amount      int    `json:"amount"`
	Description string `json:"description"`
	ReturnURL   string `json:"returnUrl"`
	CancelURL   string `json:"cancelUrl"`
	Signature   string `json:"signature"`
}

type CreateLinkResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
	QRCode      string `json:"qrCode"`
	OrderCode   int64  `json:"orderCode"`
}

type WebhookData struct {
	Code      string `json:"code"`
	Desc      string `json:"desc"`
	Success   bool   `json:"success"`
	Data      struct {
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

func (c *Client) CreatePaymentLink(ctx context.Context, req CreateLinkRequest) (CreateLinkResponse, error) {
	req.Signature = c.signCreateLink(req)

	body, err := json.Marshal(req)
	if err != nil {
		return CreateLinkResponse{}, fmt.Errorf("payos: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v2/payment-requests", bytes.NewReader(body))
	if err != nil {
		return CreateLinkResponse{}, fmt.Errorf("payos: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-client-id", c.cfg.ClientID)
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CreateLinkResponse{}, fmt.Errorf("payos: http call: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var wrapper struct {
		Code string             `json:"code"`
		Desc string             `json:"desc"`
		Data CreateLinkResponse `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return CreateLinkResponse{}, fmt.Errorf("payos: decode response: %w", err)
	}
	if wrapper.Code != "00" {
		return CreateLinkResponse{}, fmt.Errorf("payos: gateway error %s: %s", wrapper.Code, wrapper.Desc)
	}
	return wrapper.Data, nil
}

func (c *Client) VerifyWebhookData(body []byte) (*WebhookData, error) {
	var payload WebhookData
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("payos: decode webhook body: %w", err)
	}

	if c.cfg.ChecksumKey != "" {
		expectedSig := c.signWebhookData(payload)
		if !hmac.Equal([]byte(payload.Signature), []byte(expectedSig)) {
			return nil, fmt.Errorf("payos: invalid webhook signature")
		}
	}

	return &payload, nil
}

func (c *Client) signCreateLink(req CreateLinkRequest) string {
	raw := fmt.Sprintf("amount=%d&cancelUrl=%s&description=%s&orderCode=%d&returnUrl=%s",
		req.Amount, req.CancelURL, req.Description, req.OrderCode, req.ReturnURL)
	return c.hmac(raw)
}

func (c *Client) signWebhookData(data WebhookData) string {
	fields := map[string]string{
		"orderCode":           fmt.Sprintf("%d", data.Data.OrderCode),
		"amount":              fmt.Sprintf("%d", data.Data.Amount),
		"description":         data.Data.Description,
		"accountNumber":       data.Data.AccountNumber,
		"reference":           data.Data.Reference,
		"transactionDateTime": data.Data.TransactionTime,
		"paymentLinkId":       data.Data.PaymentLinkID,
		"code":                data.Data.Code,
		"desc":                data.Data.Desc,
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	return c.hmac(strings.Join(parts, "&"))
}

func (c *Client) hmac(message string) string {
	mac := hmac.New(sha256.New, []byte(c.cfg.ChecksumKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
