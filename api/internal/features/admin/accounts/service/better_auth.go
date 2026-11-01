package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/authcookie"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type BetterAuthAdmin struct {
	cfg    config.AuthConfig
	client *http.Client
}

func NewBetterAuthAdmin(cfg config.AuthConfig) *BetterAuthAdmin {
	// The plugin response also waits for notification submission (up to 10 seconds).
	return &BetterAuthAdmin{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout + 11*time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (a *BetterAuthAdmin) SetLock(ctx context.Context, cookie string, id uuid.UUID, locking, notify bool, input domain.LockRequest) (result domain.Transition, err error) {
	if a.cfg.AdminSecret == "" {
		return domain.Transition{}, apperror.New(apperror.CodeAuthUnavailable, "account management is not configured")
	}
	action := "unlock"
	path := "/admin/unban-user"
	body := map[string]any{"userId": id.String()}
	if locking {
		action = "lock"
		path = "/admin/ban-user"
		body["banReason"] = input.Reason
		if input.ExpiresIn != nil {
			body["banExpiresIn"] = *input.ExpiresIn
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return domain.Transition{}, err
	}
	done := logger.Start(ctx, "accounts.set_lock", logger.String("action", action), logger.String("user_id", id.String()), zap.Bool("notify", notify))
	var failure []zap.Field
	defer func() {
		if err != nil {
			done(err, failure...)
			return
		}
		done(nil)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.URL+"/api/auth"+path, bytes.NewReader(data))
	if err != nil {
		failure = []zap.Field{logger.String("reason", "build_request")}
		return domain.Transition{}, apperror.Wrap(apperror.CodeAuthUnavailable, "account access change could not be confirmed", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", a.cfg.PublicURL)
	incoming := &http.Request{Header: http.Header{"Cookie": []string{cookie}}}
	for _, authCookie := range incoming.Cookies() {
		if authcookie.Matches(authCookie.Name, a.cfg.CookiePrefix, a.cfg.SecureCookies) {
			req.AddCookie(authCookie)
		}
	}
	req.Header.Set("X-RoleCue-Admin-Secret", a.cfg.AdminSecret)
	req.Header.Set("X-RoleCue-Account-Notification", "skip")
	if notify {
		req.Header.Set("X-RoleCue-Account-Notification", "send")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		failure = []zap.Field{logger.String("reason", lockTransportReason(err))}
		return domain.Transition{}, apperror.Wrap(apperror.CodeAuthUnavailable, "account access change could not be confirmed", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		code := apperror.CodeAuthUnavailable
		if resp.StatusCode == http.StatusUnauthorized {
			code = apperror.CodeInvalidToken
		}
		if resp.StatusCode == http.StatusForbidden {
			code = apperror.CodeForbidden
		}
		failure = []zap.Field{logger.String("reason", lockStatusReason(resp.StatusCode)), logger.Int("status_code", resp.StatusCode)}
		return domain.Transition{}, apperror.New(code, "account access change could not be confirmed")
	}
	var payload struct {
		Notification domain.Notification `json:"notification"`
		User         struct {
			ID            uuid.UUID  `json:"id"`
			Name          string     `json:"name"`
			Email         string     `json:"email"`
			Image         *string    `json:"image"`
			Role          string     `json:"role"`
			EmailVerified bool       `json:"emailVerified"`
			Banned        bool       `json:"banned"`
			BanReason     *string    `json:"banReason"`
			BanExpires    *time.Time `json:"banExpires"`
			CreatedAt     time.Time  `json:"createdAt"`
			UpdatedAt     time.Time  `json:"updatedAt"`
		} `json:"user"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload); err != nil {
		failure = []zap.Field{logger.String("reason", "decode_error")}
		return domain.Transition{}, apperror.New(apperror.CodeAuthUnavailable, "account access change could not be confirmed")
	}
	if payload.User.ID != id || payload.User.Banned != locking {
		failure = []zap.Field{logger.String("reason", "unexpected_response")}
		return domain.Transition{}, apperror.New(apperror.CodeAuthUnavailable, "account access change could not be confirmed")
	}
	user := payload.User
	account := domain.Account{ID: user.ID, Name: user.Name, Email: user.Email, Image: user.Image, Role: user.Role,
		EmailVerified: user.EmailVerified, IsLocked: user.Banned, LockReason: user.BanReason, LockExpiresAt: user.BanExpires,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
	// A mail failure does not turn a confirmed plugin write into a failed transition.
	if !notify {
		payload.Notification.Status = "not-needed"
	} else if payload.Notification.Status != "accepted" && payload.Notification.Status != "failed" {
		payload.Notification.Status = "failed"
	}
	if payload.Notification.Status == "failed" {
		logger.FromContext(ctx).Warn("account change confirmed but notification failed",
			logger.Op("accounts.set_lock"), logger.String("action", action), logger.String("user_id", id.String()), logger.String("reason", "notification_failed"))
	}
	return domain.Transition{User: account, Changed: true, Notification: payload.Notification}, nil
}

// lockTransportReason classifies a request that never produced a response.
func lockTransportReason(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	}
	return "unreachable"
}

// lockStatusReason classifies a non-200 answer by its HTTP status class.
func lockStatusReason(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "auth_rejected"
	case code == http.StatusForbidden:
		return "forbidden"
	case code >= 500:
		return "http_5xx"
	}
	return "http_4xx"
}
