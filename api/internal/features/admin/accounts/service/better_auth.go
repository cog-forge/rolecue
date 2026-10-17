package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/authcookie"
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

func (a *BetterAuthAdmin) SetLock(ctx context.Context, cookie string, id uuid.UUID, locking, notify bool, input domain.LockRequest) (domain.Transition, error) {
	if a.cfg.AdminSecret == "" {
		return domain.Transition{}, apperror.New(apperror.CodeAuthUnavailable, "account management is not configured")
	}
	path := "/admin/unban-user"
	body := map[string]any{"userId": id.String()}
	if locking {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.URL+"/api/auth"+path, bytes.NewReader(data))
	if err != nil {
		return domain.Transition{}, apperror.Wrap(apperror.CodeAuthUnavailable, "account access change could not be confirmed", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", a.cfg.URL)
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload); err != nil || payload.User.ID != id || payload.User.Banned != locking {
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
	return domain.Transition{User: account, Changed: true, Notification: payload.Notification}, nil
}
