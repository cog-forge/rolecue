package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClientID    = "test-client-id"
	testAPIKey      = "test-api-key"
	testChecksumKey = "test-checksum-key-1234567890abcdef"
)

func TestNewClient_Validation(t *testing.T) {
	t.Run("missing clientID fails", func(t *testing.T) {
		_, err := payos.NewClient("", testAPIKey, testChecksumKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "clientID is required")

		_, err = payos.NewClient("   ", testAPIKey, testChecksumKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "clientID is required")
	})

	t.Run("missing apiKey fails", func(t *testing.T) {
		_, err := payos.NewClient(testClientID, "", testChecksumKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "apiKey is required")

		_, err = payos.NewClient(testClientID, "   ", testChecksumKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "apiKey is required")
	})

	t.Run("missing checksumKey fails", func(t *testing.T) {
		_, err := payos.NewClient(testClientID, testAPIKey, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "checksumKey is required")

		_, err = payos.NewClient(testClientID, testAPIKey, "   ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "checksumKey is required")
	})

	t.Run("valid credentials succeeds", func(t *testing.T) {
		c, err := payos.NewClient(testClientID, testAPIKey, testChecksumKey)
		assert.NoError(t, err)
		assert.NotNil(t, c)
	})

	t.Run("MustNewClient panics on error", func(t *testing.T) {
		assert.Panics(t, func() {
			payos.MustNewClient("", testAPIKey, testChecksumKey)
		})
	})

	t.Run("NewClientWithOptions validation", func(t *testing.T) {
		_, err := payos.NewClientWithOptions(nil)
		assert.Error(t, err)

		_, err = payos.NewClientWithOptions(&payos.Options{
			ClientId:    "",
			ApiKey:      testAPIKey,
			ChecksumKey: testChecksumKey,
		})
		assert.Error(t, err)

		c, err := payos.NewClientWithOptions(&payos.Options{
			ClientId:    testClientID,
			ApiKey:      testAPIKey,
			ChecksumKey: testChecksumKey,
		})
		assert.NoError(t, err)
		assert.NotNil(t, c)
		assert.NotNil(t, c.SDK())
		assert.NotNil(t, c.PaymentRequests())
		assert.NotNil(t, c.Webhooks())
		assert.NotNil(t, c.Payouts())
		assert.NotNil(t, c.PayoutsAccount())
	})

	t.Run("MustNewClientWithOptions panics on error", func(t *testing.T) {
		assert.Panics(t, func() {
			payos.MustNewClientWithOptions(nil)
		})
	})
}

func hmacSHA256(message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookData(t *testing.T) {
	client, err := payos.NewClient(testClientID, testAPIKey, testChecksumKey)
	require.NoError(t, err)

	t.Run("valid signature succeeds", func(t *testing.T) {
		// Keys sorted: amount=50000&description=topup&orderCode=123456
		signedStr := "amount=50000&description=topup&orderCode=123456"
		sig := hmacSHA256(signedStr, testChecksumKey)

		payload := map[string]interface{}{
			"code":    "00",
			"desc":    "success",
			"success": true,
			"data": map[string]interface{}{
				"orderCode":   123456,
				"amount":      50000,
				"description": "topup",
			},
			"signature": sig,
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		webhookData, err := client.VerifyWebhookData(body)
		require.NoError(t, err)
		assert.Equal(t, int64(123456), webhookData.Data.OrderCode)
		assert.Equal(t, 50000, webhookData.Data.Amount)
		assert.Equal(t, "topup", webhookData.Data.Description)
	})

	t.Run("tampered data fails signature verification", func(t *testing.T) {
		signedStr := "amount=50000&description=topup&orderCode=123456"
		sig := hmacSHA256(signedStr, testChecksumKey)

		// Tamper amount to 100000
		payload := map[string]interface{}{
			"code":    "00",
			"desc":    "success",
			"success": true,
			"data": map[string]interface{}{
				"orderCode":   123456,
				"amount":      100000,
				"description": "topup",
			},
			"signature": sig,
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		_, err = client.VerifyWebhookData(body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid webhook signature")
	})

	t.Run("invalid signature fails", func(t *testing.T) {
		payload := map[string]interface{}{
			"code":    "00",
			"desc":    "success",
			"success": true,
			"data": map[string]interface{}{
				"orderCode": 123456,
				"amount":    50000,
			},
			"signature": "invalid-signature",
		}
		body, err := json.Marshal(payload)
		require.NoError(t, err)

		_, err = client.VerifyWebhookData(body)
		assert.Error(t, err)
	})

	t.Run("missing data fails", func(t *testing.T) {
		body := []byte(`{"code":"00","signature":"abc"}`)
		_, err := client.VerifyWebhookData(body)
		assert.Error(t, err)
	})

	t.Run("missing signature fails", func(t *testing.T) {
		body := []byte(`{"code":"00","data":{"orderCode":123}}`)
		_, err := client.VerifyWebhookData(body)
		assert.Error(t, err)
	})

	t.Run("malformed json fails", func(t *testing.T) {
		body := []byte(`{not valid json}`)
		_, err := client.VerifyWebhookData(body)
		assert.Error(t, err)
	})
}
