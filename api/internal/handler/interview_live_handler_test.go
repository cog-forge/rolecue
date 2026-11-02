package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	authservice "github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const liveOrigin = "http://localhost:3000"

// liveSessions answers Get with the configured error, recording the caller.
type liveSessions struct {
	err       error
	calls     int
	candidate uuid.UUID
	role      string
}

func (s *liveSessions) Get(_ context.Context, candidateID uuid.UUID, role string, _ uuid.UUID) (domain.Session, error) {
	s.calls++
	s.candidate, s.role = candidateID, role
	return domain.Session{}, s.err
}

// liveEngine records the participant of each served socket and closes it normally.
type liveEngine struct {
	mu     sync.Mutex
	served []runtime.Participant
}

func (e *liveEngine) Serve(_ context.Context, ws *websocket.Conn, p runtime.Participant) {
	e.mu.Lock()
	e.served = append(e.served, p)
	e.mu.Unlock()
	_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))
	_ = ws.Close()
}

// failingAuth refuses every request like an absent Better Auth session.
type failingAuth struct{}

func (failingAuth) Authenticate(context.Context, *http.Request) (authdomain.User, []string, error) {
	return authdomain.User{}, nil, authdomain.ErrInvalidSession
}

func liveServer(t *testing.T, auth authservice.Authenticator, sessions *liveSessions, engine *liveEngine) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewInterviewLiveHandler(sessions, engine, []string{liveOrigin, "*"})
	r.GET("/interviews/:id/live", middleware.RequireAuth(auth, config.CORSConfig{AllowOrigins: []string{liveOrigin}}), middleware.RequireOnboarding(), h.Live)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	return server
}

func dialLive(t *testing.T, server *httptest.Server, path, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	dialer := websocket.Dialer{Subprotocols: []string{runtime.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	return dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, header)
}

func envelopeCode(t *testing.T, resp *http.Response) apperror.Code {
	t.Helper()
	var envelope response.Envelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Error == nil {
		t.Fatal("no error in envelope")
	}
	return envelope.Error.Code
}

func TestLiveHandshakeBindsTheSocketToTheCandidatesSession(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate", OnboardingCompleted: true}
	sessions, engine := &liveSessions{}, &liveEngine{}
	server := liveServer(t, onboardingSession{user}, sessions, engine)
	sessionID := uuid.New()

	ws, resp, err := dialLive(t, server, "/interviews/"+sessionID.String()+"/live", liveOrigin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = ws.Close() }()
	if resp.StatusCode != http.StatusSwitchingProtocols || ws.Subprotocol() != runtime.Subprotocol {
		t.Fatalf("status %d subprotocol %q", resp.StatusCode, ws.Subprotocol())
	}
	_, _, err = ws.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("read = %v", err)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	want := runtime.Participant{CandidateID: user.ID, Role: "candidate", SessionID: sessionID}
	if len(engine.served) != 1 || engine.served[0] != want || sessions.candidate != user.ID || sessions.role != "candidate" {
		t.Fatalf("served %+v, sessions read by %s/%s", engine.served, sessions.candidate, sessions.role)
	}
}

func TestLiveHandshakeRefusals(t *testing.T) {
	candidate := authdomain.User{ID: uuid.New(), Role: "candidate", OnboardingCompleted: true}
	valid := "/interviews/" + uuid.NewString() + "/live"
	for _, tc := range []struct {
		name     string
		auth     authservice.Authenticator
		sessions *liveSessions
		path     string
		origin   string
		status   int
		code     apperror.Code
	}{
		{"unauthenticated", failingAuth{}, &liveSessions{}, valid, liveOrigin, http.StatusUnauthorized, apperror.CodeInvalidToken},
		{"untrusted origin", onboardingSession{candidate}, &liveSessions{}, valid, "https://evil.example", http.StatusForbidden, apperror.CodeForbidden},
		{"missing origin", onboardingSession{candidate}, &liveSessions{}, valid, "", http.StatusForbidden, apperror.CodeForbidden},
		{"wildcard is never trusted", onboardingSession{candidate}, &liveSessions{}, valid, "*", http.StatusForbidden, apperror.CodeForbidden},
		{"onboarding pending", onboardingSession{authdomain.User{ID: uuid.New(), Role: "candidate"}}, &liveSessions{}, valid, liveOrigin, http.StatusForbidden, apperror.CodeForbidden},
		{"foreign session", onboardingSession{candidate}, &liveSessions{err: domain.ErrInterviewNotFound}, valid, liveOrigin, http.StatusNotFound, apperror.CodeInterviewNotFound},
		{"not a candidate", onboardingSession{candidate}, &liveSessions{err: apperror.New(apperror.CodeForbidden, "only candidates")}, valid, liveOrigin, http.StatusForbidden, apperror.CodeForbidden},
		{"bad id", onboardingSession{candidate}, &liveSessions{}, "/interviews/not-a-uuid/live", liveOrigin, http.StatusBadRequest, apperror.CodeValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &liveEngine{}
			server := liveServer(t, tc.auth, tc.sessions, engine)
			ws, resp, err := dialLive(t, server, tc.path, tc.origin)
			if err == nil {
				_ = ws.Close()
				t.Fatal("the handshake succeeded")
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			if code := envelopeCode(t, resp); code != tc.code {
				t.Fatalf("code = %s, want %s", code, tc.code)
			}
			if len(engine.served) != 0 {
				t.Fatal("a refused handshake reached the engine")
			}
		})
	}
}

func TestLiveRequiresAWebSocketUpgrade(t *testing.T) {
	user := authdomain.User{ID: uuid.New(), Role: "candidate", OnboardingCompleted: true}
	sessions := &liveSessions{}
	server := liveServer(t, onboardingSession{user}, sessions, &liveEngine{})
	req, err := http.NewRequest(http.MethodGet, server.URL+"/interviews/"+uuid.NewString()+"/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", liveOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest || envelopeCode(t, resp) != apperror.CodeValidation || sessions.calls != 0 {
		t.Fatalf("status = %d, sessions read %d times", resp.StatusCode, sessions.calls)
	}
}
