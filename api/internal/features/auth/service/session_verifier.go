package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/google/uuid"
)

type VerifiedSession struct {
	UserID     uuid.UUID
	SetCookies []string
}

type SessionVerifier interface {
	Verify(context.Context, *http.Request) (VerifiedSession, error)
}

type HTTPVerifier struct {
	cfg    config.AuthConfig
	client *http.Client
}

func NewHTTPVerifier(cfg config.AuthConfig) *HTTPVerifier {
	return &HTTPVerifier{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (v *HTTPVerifier) authCookieName(name string) bool {
	prefix := v.cfg.CookiePrefix + "."
	if v.cfg.SecureCookies {
		prefix = "__Secure-" + prefix
	}
	return name == prefix+"session_token" || name == prefix+"dont_remember" || name == prefix+"session_data"
}

func (v *HTTPVerifier) Verify(ctx context.Context, incoming *http.Request) (VerifiedSession, error) {
	var result VerifiedSession
	if v.cfg.Timeout <= 0 || v.cfg.URL == "" || v.cfg.CookiePrefix == "" {
		return result, domain.ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.URL+"/api/auth/get-session?disableCookieCache=true", nil)
	if err != nil {
		return result, domain.ErrUnavailable
	}
	hasToken := false
	for _, cookie := range incoming.Cookies() {
		if v.authCookieName(cookie.Name) {
			req.AddCookie(cookie)
			if strings.HasSuffix(cookie.Name, ".session_token") && cookie.Value != "" {
				hasToken = true
			}
		}
	}
	if !hasToken {
		return result, domain.ErrInvalidSession
	}
	req.Header.Set("Accept", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return result, domain.ErrUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	for _, raw := range resp.Header.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(raw)
		if err != nil || !v.authCookieName(cookie.Name) || cookie.Path != "/" || strings.TrimPrefix(cookie.Domain, ".") != v.cfg.CookieDomain || cookie.Secure != v.cfg.SecureCookies || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
			return VerifiedSession{}, domain.ErrUnavailable
		}
		result.SetCookies = append(result.SetCookies, raw)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return result, domain.ErrInvalidSession
	}
	if resp.StatusCode != http.StatusOK {
		return result, domain.ErrUnavailable
	}
	const maxBody = 64 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return result, domain.ErrUnavailable
	}
	if strings.TrimSpace(string(body)) == "null" {
		return result, domain.ErrInvalidSession
	}
	var payload struct {
		User *struct {
			ID string `json:"id"`
		} `json:"user"`
		Session *struct {
			ID             string    `json:"id"`
			UserID         string    `json:"userId"`
			ExpiresAt      time.Time `json:"expiresAt"`
			ImpersonatedBy *string   `json:"impersonatedBy"`
		} `json:"session"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.User == nil || payload.Session == nil {
		return result, domain.ErrUnavailable
	}
	userID, err := uuid.Parse(payload.User.ID)
	if err != nil || userID == uuid.Nil {
		return result, domain.ErrUnavailable
	}
	sessionID, err := uuid.Parse(payload.Session.ID)
	if err != nil || sessionID == uuid.Nil || payload.Session.UserID != payload.User.ID || payload.Session.ExpiresAt.IsZero() {
		return result, domain.ErrUnavailable
	}
	if !payload.Session.ExpiresAt.After(time.Now()) || (payload.Session.ImpersonatedBy != nil && *payload.Session.ImpersonatedBy != "") {
		return result, domain.ErrInvalidSession
	}
	result.UserID = userID
	return result, nil
}
