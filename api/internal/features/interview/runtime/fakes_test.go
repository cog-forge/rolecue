package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	testCandidate = uuid.MustParse("00000000-0000-4000-8000-0000000000c1")
	testSession   = uuid.MustParse("00000000-0000-4000-8000-0000000000e1")
)

// fakeSessions is an in-memory interview service with the same turn rules as the real one: answers go
// to the next position only, a repeated answer with the same text is unchanged, and completion is
// idempotent. It counts every call so tests can prove nothing ran twice.
type fakeSessions struct {
	mu            sync.Mutex
	kind          domain.Type
	questions     []string
	spoken        map[int32]string
	answers       []string
	voice         string
	completed     bool
	notEligible   bool
	appendErr     error
	deliverCalls  int
	appendCalls   int
	completeCalls int
	// appended receives the answered count after each stored answer.
	appended chan int32
}

func newFakeSessions(kind domain.Type, questions ...string) *fakeSessions {
	return &fakeSessions{kind: kind, questions: questions, spoken: map[int32]string{}, voice: "en-US-AvaNeural", appended: make(chan int32, 16)}
}

func (f *fakeSessions) session() domain.Session {
	status := domain.EvaluationPending
	session := domain.Session{
		ID: testSession, Type: f.kind, QuestionCount: int32(len(f.questions)), AnsweredCount: int32(len(f.answers)),
		AnsweredTurns: int32(len(f.answers)), EvaluationStatus: status,
	}
	if f.completed {
		now := time.Now()
		session.CompletedAt = &now
	}
	return session
}

func (f *fakeSessions) Get(_ context.Context, candidateID uuid.UUID, _ string, id uuid.UUID) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if candidateID != testCandidate || id != testSession {
		return domain.Session{}, domain.ErrInterviewNotFound
	}
	return f.session(), nil
}

func (f *fakeSessions) Deliver(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (domain.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliverCalls++
	if f.completed {
		return domain.Delivery{}, domain.ErrInterviewCompleted
	}
	if f.notEligible {
		return domain.Delivery{}, domain.ErrApplicationNotEligible
	}
	d := domain.Delivery{Type: f.kind, QuestionCount: int32(len(f.questions)), AnsweredCount: int32(len(f.answers)), ProviderVoiceID: f.voice}
	if len(f.answers) >= len(f.questions) {
		return d, nil
	}
	position := int32(len(f.answers)) + 1
	if _, ok := f.spoken[position]; !ok {
		f.spoken[position] = f.questions[position-1]
	}
	d.Question = &domain.AskedQuestion{Position: position, QuestionNumber: position, Kind: domain.TurnCore, Text: f.spoken[position], AskedAt: time.Now()}
	return d, nil
}

func (f *fakeSessions) AppendTurn(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, position int32, answer string) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.appendCalls++
	if f.appendErr != nil {
		return domain.Session{}, f.appendErr
	}
	if f.completed {
		return domain.Session{}, domain.ErrInterviewCompleted
	}
	if f.notEligible {
		return domain.Session{}, domain.ErrApplicationNotEligible
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return domain.Session{}, apperror.New(apperror.CodeValidation, "answer is required")
	}
	turns := make([]domain.Turn, 0, len(f.answers))
	for i, a := range f.answers {
		turns = append(turns, domain.Turn{Position: int32(i + 1), QuestionNumber: int32(i + 1), Kind: domain.TurnCore, Answer: a})
	}
	_, unchanged, err := domain.AcceptAnswer(turns, int32(len(f.questions)), position, answer)
	if err != nil {
		return domain.Session{}, err
	}
	if !unchanged {
		f.answers = append(f.answers, answer)
		f.appended <- int32(len(f.answers))
	}
	return f.session(), nil
}

// FollowUp never asks one: these sessions have follow-ups off (followup_test.go covers them).
func (f *fakeSessions) FollowUp(context.Context, uuid.UUID, string, uuid.UUID) (domain.Delivery, error) {
	return domain.Delivery{}, nil
}

func (f *fakeSessions) AppendFollowUp(context.Context, uuid.UUID, string, uuid.UUID, domain.AskedQuestion, string) (domain.Session, error) {
	return domain.Session{}, errors.New("follow-ups are off")
}

func (f *fakeSessions) Complete(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.completed {
		if f.notEligible {
			return domain.Session{}, domain.ErrApplicationNotEligible
		}
		f.completeCalls++
		f.completed = true
	}
	return f.session(), nil
}

func (f *fakeSessions) set(update func(*fakeSessions)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	update(f)
}

func (f *fakeSessions) counts() (answers, appends, completes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.answers), f.appendCalls, f.completeCalls
}

// gateTranscriber blocks each transcription until the test releases it, to hold the engine in
// processing.
type gateTranscriber struct {
	started chan struct{}
	release chan struct{}
}

func (g *gateTranscriber) Transcribe(ctx context.Context, _ Audio) (Transcript, error) {
	g.started <- struct{}{}
	select {
	case <-g.release:
	case <-ctx.Done():
		return Transcript{}, ctx.Err()
	}
	return Transcript{Text: "an answer given while the socket dropped"}, nil
}

// harness runs the engine behind a real WebSocket server.
type harness struct {
	t        *testing.T
	sessions *fakeSessions
	engine   *Engine
	server   *httptest.Server
}

type harnessOption func(*harnessConfig)

type harnessConfig struct {
	synthesizer tts.Synthesizer
	transcriber Transcriber
	cache       int
	settings    Settings
	verify      func(context.Context) error
}

func withGuards(s Settings) harnessOption {
	return func(c *harnessConfig) { c.settings = s }
}

func withVerify(f func(context.Context) error) harnessOption {
	return func(c *harnessConfig) { c.verify = f }
}

func withSynthesizer(s tts.Synthesizer) harnessOption {
	return func(c *harnessConfig) { c.synthesizer = s }
}

func withTranscriber(t Transcriber) harnessOption {
	return func(c *harnessConfig) { c.transcriber = t }
}

func withCache(entries int) harnessOption {
	return func(c *harnessConfig) { c.cache = entries }
}

func newHarness(t *testing.T, sessions *fakeSessions, opts ...harnessOption) *harness {
	t.Helper()
	h := serve(t, sessions, opts...)
	h.sessions = sessions
	return h
}

// serve runs the engine over any Sessions, such as the real service over an in-memory repository.
func serve(t *testing.T, sessions Sessions, opts ...harnessOption) *harness {
	t.Helper()
	cfg := harnessConfig{synthesizer: tts.NewFake(tts.WithFakeVisemes(), tts.WithFakeBlendShapes(), tts.WithFakeWordBoundaries()), transcriber: NewFakeTranscriber()}
	for _, opt := range opts {
		opt(&cfg)
	}
	settings := cfg.settings
	settings.STTTimeout, settings.CacheEntries = 5*time.Second, cfg.cache
	engine, err := NewEngine(sessions, cfg.synthesizer, cfg.transcriber, NewHub(), settings)
	if err != nil {
		t.Fatal(err)
	}
	upgrader := websocket.Upgrader{Subprotocols: []string{Subprotocol}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		engine.Serve(r.Context(), ws, Participant{CandidateID: testCandidate, Role: "candidate", SessionID: testSession, Verify: cfg.verify})
	}))
	t.Cleanup(server.Close)
	return &harness{t: t, engine: engine, server: server}
}

// received is a server message as the client decodes it.
type received struct {
	V       int             `json:"v"`
	Stream  string          `json:"stream"`
	Seq     int64           `json:"seq"`
	Type    string          `json:"type"`
	ReplyTo string          `json:"reply_to"`
	Data    json.RawMessage `json:"data"`
}

type client struct {
	t    *testing.T
	ws   *websocket.Conn
	seen []received
}

func (h *harness) dial(protocols ...string) *client {
	h.t.Helper()
	if protocols == nil {
		protocols = []string{Subprotocol}
	}
	dialer := websocket.Dialer{Subprotocols: protocols, HandshakeTimeout: 5 * time.Second}
	ws, _, err := dialer.Dial("ws"+strings.TrimPrefix(h.server.URL, "http"), nil)
	if err != nil {
		h.t.Fatalf("dial: %v", err)
	}
	c := &client{t: h.t, ws: ws}
	h.t.Cleanup(func() { _ = ws.Close() })
	return c
}

// next reads one server message.
func (c *client) next() received {
	c.t.Helper()
	_ = c.ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ws.ReadMessage()
	if err != nil {
		c.t.Fatalf("read: %v (messages so far: %s)", err, c.types())
	}
	var msg received
	if err := json.Unmarshal(data, &msg); err != nil {
		c.t.Fatalf("decode %s: %v", data, err)
	}
	c.seen = append(c.seen, msg)
	return msg
}

// expect reads the next message, requires its type and decodes its data into out when out is not nil.
func (c *client) expect(messageType string, out any) received {
	c.t.Helper()
	msg := c.next()
	if msg.Type != messageType {
		c.t.Fatalf("got %s %s, want %s", msg.Type, msg.Data, messageType)
	}
	if out != nil {
		if err := json.Unmarshal(msg.Data, out); err != nil {
			c.t.Fatalf("decode %s data: %v", messageType, err)
		}
	}
	return msg
}

// expectState reads a state message and requires the state.
func (c *client) expectState(state State) {
	c.t.Helper()
	var change StateChange
	c.expect(TypeState, &change)
	if change.State != state {
		c.t.Fatalf("state = %s, want %s", change.State, state)
	}
}

// expectError reads an error message and requires its code.
func (c *client) expectError(code string) ErrorData {
	c.t.Helper()
	var data ErrorData
	c.expect(TypeError, &data)
	if data.Code != code {
		c.t.Fatalf("error code = %s (%s), want %s", data.Code, data.Message, code)
	}
	return data
}

// expectClose requires the server to close with the code as its next frame.
func (c *client) expectClose(code int) {
	c.t.Helper()
	_ = c.ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ws.ReadMessage()
	var closeErr *websocket.CloseError
	if err == nil || !errors.As(err, &closeErr) || closeErr.Code != code {
		c.t.Fatalf("got %s / %v, want close %d", data, err, code)
	}
}

func (c *client) send(messageType, id string, data any) {
	c.t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		c.t.Fatal(err)
	}
	c.sendRaw(mustJSON(c.t, ClientMessage{V: ProtocolVersion, ID: id, Type: messageType, Data: raw}))
}

func (c *client) sendRaw(payload []byte) {
	c.t.Helper()
	if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

func (c *client) types() string {
	names := make([]string, 0, len(c.seen))
	for _, msg := range c.seen {
		names = append(names, msg.Type)
	}
	return strings.Join(names, ",")
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// webm is a minimal recording that passes the container check.
func webm(payload string) []byte {
	return append([]byte{0x1A, 0x45, 0xDF, 0xA3}, payload...)
}

// connect dials and reads the opening sync and the first turn.
func (h *harness) connect() (*client, SessionSync, InterviewerTurn) {
	h.t.Helper()
	c := h.dial()
	var sync SessionSync
	c.expect(TypeSessionSync, &sync)
	var turn InterviewerTurn
	c.expect(TypeInterviewerTurn, &turn)
	return c, sync, turn
}

// listen ends the interviewer turn at the position.
func (c *client) listen(id string, position int32) {
	c.t.Helper()
	c.send(TypeTurnPlayed, id, TurnPlayed{Position: position})
	c.expectState(StateListening)
}
