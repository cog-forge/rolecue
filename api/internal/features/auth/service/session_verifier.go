package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/authcookie"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
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
	return authcookie.Matches(name, v.cfg.CookiePrefix, v.cfg.SecureCookies)
}

// Verify asks the auth service whether the request's session is valid. Every rejection carries a reason
// in its error chain, which the response layer logs. The upstream call is also logged as one step with its
// duration, HTTP status and a classified reason. Cookie values, the URL query and the body never reach a log.
func (v *HTTPVerifier) Verify(ctx context.Context, incoming *http.Request) (result VerifiedSession, err error) {
	if v.cfg.Timeout <= 0 || v.cfg.URL == "" || v.cfg.CookiePrefix == "" {
		return result, fmt.Errorf("%w: verifier not configured", domain.ErrUnavailable)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.URL+"/api/auth/get-session?disableCookieCache=true", nil)
	if err != nil {
		return result, fmt.Errorf("%w: build session request", domain.ErrUnavailable)
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
		return result, fmt.Errorf("%w: no session cookie", domain.ErrInvalidSession)
	}
	req.Header.Set("Accept", "application/json")

	done := logger.Start(ctx, "auth.verify_session")
	var reason string
	var status int
	defer func() {
		if err == nil {
			done(nil, logger.String("user_id", result.UserID.String()))
			return
		}
		fields := []zap.Field{logger.String("reason", reason)}
		if status != 0 {
			fields = append(fields, logger.Int("status_code", status))
		}
		done(err, fields...)
	}()
	// reject records why the check failed and returns the sentinel with that reason in its chain.
	reject := func(sentinel error, why string) (VerifiedSession, error) {
		reason = why
		return result, fmt.Errorf("%w: %s", sentinel, why)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return reject(domain.ErrUnavailable, transportReason(err))
	}
	defer func() { _ = resp.Body.Close() }()
	status = resp.StatusCode
	for _, raw := range resp.Header.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(raw)
		if err != nil || !v.authCookieName(cookie.Name) || cookie.Path != "/" || strings.TrimPrefix(cookie.Domain, ".") != v.cfg.CookieDomain || cookie.Secure != v.cfg.SecureCookies || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
			reason = "malformed_set_cookie"
			return VerifiedSession{}, fmt.Errorf("%w: %s", domain.ErrUnavailable, reason)
		}
		result.SetCookies = append(result.SetCookies, raw)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return reject(domain.ErrInvalidSession, "auth_rejected")
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 500 {
			return reject(domain.ErrUnavailable, "http_5xx")
		}
		return reject(domain.ErrUnavailable, "http_4xx")
	}
	const maxBody = 64 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return reject(domain.ErrUnavailable, "read_error")
	}
	if len(body) > maxBody {
		return reject(domain.ErrUnavailable, "response_too_large")
	}
	if strings.TrimSpace(string(body)) == "null" {
		return reject(domain.ErrInvalidSession, "session_absent")
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
	if err := json.Unmarshal(body, &payload); err != nil {
		return reject(domain.ErrUnavailable, "decode_error")
	}
	if payload.User == nil || payload.Session == nil {
		return reject(domain.ErrUnavailable, "malformed_response")
	}
	userID, err := uuid.Parse(payload.User.ID)
	if err != nil || userID == uuid.Nil {
		return reject(domain.ErrUnavailable, "malformed_response")
	}
	sessionID, err := uuid.Parse(payload.Session.ID)
	if err != nil || sessionID == uuid.Nil || payload.Session.UserID != payload.User.ID || payload.Session.ExpiresAt.IsZero() {
		return reject(domain.ErrUnavailable, "malformed_response")
	}
	if !payload.Session.ExpiresAt.After(time.Now()) {
		return reject(domain.ErrInvalidSession, "session_expired")
	}
	if payload.Session.ImpersonatedBy != nil && *payload.Session.ImpersonatedBy != "" {
		return reject(domain.ErrInvalidSession, "impersonated_session")
	}
	result.UserID = userID
	return result, nil
}

// transportReason classifies a session check that never produced a response.
func transportReason(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	}
	return "unreachable"
}
