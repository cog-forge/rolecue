package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// liveHandshakeTimeout bounds the WebSocket handshake after the HTTP checks passed.
const liveHandshakeTimeout = 10 * time.Second

// InterviewSessionReader reads one of the Candidate's sessions. The interview service implements it; the
// live handler uses it to refuse a foreign or missing session before upgrading.
type InterviewSessionReader interface {
	Get(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error)
}

// InterviewLiveEngine drives one upgraded live interview socket. runtime.Engine implements it.
type InterviewLiveEngine interface {
	Serve(ctx context.Context, ws *websocket.Conn, p runtime.Participant)
}

// InterviewLiveHandler upgrades the live interview WebSocket.
type InterviewLiveHandler struct {
	sessions InterviewSessionReader
	engine   InterviewLiveEngine
	origins  map[string]struct{}
	upgrader websocket.Upgrader
}

// NewInterviewLiveHandler takes the trusted browser origins (CORS_ALLOW_ORIGINS). A wildcard is never
// trusted for a credentialed socket.
func NewInterviewLiveHandler(sessions InterviewSessionReader, engine InterviewLiveEngine, allowOrigins []string) *InterviewLiveHandler {
	h := &InterviewLiveHandler{sessions: sessions, engine: engine, origins: make(map[string]struct{})}
	for _, origin := range allowOrigins {
		if origin != "" && origin != "*" {
			h.origins[origin] = struct{}{}
		}
	}
	h.upgrader = websocket.Upgrader{
		HandshakeTimeout: liveHandshakeTimeout,
		Subprotocols:     []string{runtime.Subprotocol},
		CheckOrigin:      func(r *http.Request) bool { return h.trusted(r.Header.Get("Origin")) },
	}
	return h
}

func (h *InterviewLiveHandler) trusted(origin string) bool {
	_, ok := h.origins[origin]
	return ok
}

// Live godoc
// @Summary Open the live interview WebSocket
// @Description Candidate only, your own session. Upgrades to the live interview protocol (subprotocol rolecue.interview.v1); the contract is docs/interview-ws-protocol.md. The socket never charges and never consumes a slot: start the session with POST /interviews first. Before the upgrade the request needs the session cookie and an Origin from CORS_ALLOW_ORIGINS (cross-site WebSocket guard). Errors before the upgrade: VALIDATION_ERROR (bad id, not a WebSocket request), INVALID_TOKEN, FORBIDDEN (untrusted Origin, onboarding, not a Candidate), INTERVIEW_NOT_FOUND (missing or another Candidate's).
// @Tags interviews
// @Security SessionCookie
// @Param Origin header string true "Trusted frontend origin"
// @Param Sec-WebSocket-Protocol header string true "rolecue.interview.v1"
// @Param id path string true "Interview session ID" format(uuid)
// @Success 101 "Switching Protocols"
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 403 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /interviews/{id}/live [get]
func (h *InterviewLiveHandler) Live(c *gin.Context) {
	user, id, ok := interviewTarget(c)
	if !ok {
		return
	}
	// Browsers always send Origin on a WebSocket handshake; RequireAuth checks it only for writes, so the
	// socket checks it here. A missing Origin is refused too.
	if !h.trusted(c.GetHeader("Origin")) {
		response.Error(c, apperror.New(apperror.CodeForbidden, "a trusted Origin is required"))
		return
	}
	if !websocket.IsWebSocketUpgrade(c.Request) {
		response.Error(c, apperror.New(apperror.CodeValidation, "a WebSocket upgrade is required"))
		return
	}
	// Ownership and role are the service's rules: another Candidate's session is not found.
	if _, err := h.sessions.Get(c.Request.Context(), user.ID, user.Role, id); err != nil {
		response.Error(c, err)
		return
	}
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// The upgrader has already answered the HTTP request.
		return
	}
	h.engine.Serve(c.Request.Context(), ws, runtime.Participant{CandidateID: user.ID, Role: user.Role, SessionID: id})
}
