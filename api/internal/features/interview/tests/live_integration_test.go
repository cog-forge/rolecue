//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const liveTestOrigin = "http://localhost:3250"

// headerAuth authenticates the Candidate named by the X-Test-User header, standing in for Better Auth.
type headerAuth struct{}

func (headerAuth) Authenticate(_ context.Context, r *http.Request) (authdomain.User, []string, error) {
	id, err := uuid.Parse(r.Header.Get("X-Test-User"))
	if err != nil {
		return authdomain.User{}, nil, authdomain.ErrInvalidSession
	}
	return authdomain.User{ID: id, Role: "candidate", OnboardingCompleted: true}, nil, nil
}

// liveStack is the real service and repository behind the real live handler and engine. Only speech
// (tts.Fake, runtime.FakeTranscriber), authentication and the evaluator are fakes.
type liveStack struct {
	svc       *service.Service
	synth     *tts.Fake
	evaluator *countingEvaluator
	server    *httptest.Server
}

func newLiveStack(t *testing.T, pool *pgxpool.Pool, settings service.Settings) *liveStack {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), settings)
	if err != nil {
		t.Fatal(err)
	}
	evaluator := newCountingEvaluator()
	svc.SetEvaluator(evaluator)
	synth := tts.NewFake(tts.WithFakeVisemes())
	engine, err := runtime.NewEngine(svc, synth, runtime.NewFakeTranscriber(), runtime.NewHub(), runtime.Settings{CacheEntries: 0})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	live := handler.NewInterviewLiveHandler(svc, engine, []string{liveTestOrigin})
	r.GET("/interviews/:id/live", middleware.RequireAuth(headerAuth{}, config.CORSConfig{AllowOrigins: []string{liveTestOrigin}}), middleware.RequireOnboarding(), live.Live)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	return &liveStack{svc: svc, synth: synth, evaluator: evaluator, server: server}
}

// liveMessage is a server message as the client decodes it.
type liveMessage struct {
	Stream string          `json:"stream"`
	Seq    int64           `json:"seq"`
	Type   string          `json:"type"`
	Data   json.RawMessage `json:"data"`
}

type liveClient struct {
	t  *testing.T
	ws *websocket.Conn
	// frames is every message data read, so a test can check what reached the Candidate.
	frames []json.RawMessage
}

func (l *liveStack) dial(t *testing.T, candidate, sessionID uuid.UUID) (*liveClient, *http.Response, error) {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", liveTestOrigin)
	header.Set("X-Test-User", candidate.String())
	dialer := websocket.Dialer{Subprotocols: []string{runtime.Subprotocol}, HandshakeTimeout: 5 * time.Second}
	ws, resp, err := dialer.Dial("ws"+strings.TrimPrefix(l.server.URL, "http")+"/interviews/"+sessionID.String()+"/live", header)
	if err != nil {
		return nil, resp, err
	}
	t.Cleanup(func() { _ = ws.Close() })
	return &liveClient{t: t, ws: ws}, resp, nil
}

func (c *liveClient) expect(messageType string, out any) liveMessage {
	c.t.Helper()
	_ = c.ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := c.ws.ReadMessage()
	if err != nil {
		c.t.Fatalf("read (want %s): %v", messageType, err)
	}
	var msg liveMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.t.Fatal(err)
	}
	c.frames = append(c.frames, msg.Data)
	if msg.Type != messageType {
		c.t.Fatalf("got %s %s, want %s", msg.Type, msg.Data, messageType)
	}
	if out != nil {
		if err := json.Unmarshal(msg.Data, out); err != nil {
			c.t.Fatal(err)
		}
	}
	return msg
}

func (c *liveClient) expectState(state runtime.State) {
	c.t.Helper()
	var change runtime.StateChange
	c.expect(runtime.TypeState, &change)
	if change.State != state {
		c.t.Fatalf("state = %s, want %s", change.State, state)
	}
}

func (c *liveClient) expectClose(code int) {
	c.t.Helper()
	_ = c.ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := c.ws.ReadMessage()
	var closeErr *websocket.CloseError
	if err == nil || !errors.As(err, &closeErr) || closeErr.Code != code {
		c.t.Fatalf("got %s / %v, want close %d", data, err, code)
	}
}

func (c *liveClient) send(messageType, id string, data any) {
	c.t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		c.t.Fatal(err)
	}
	payload, err := json.Marshal(runtime.ClientMessage{V: runtime.ProtocolVersion, ID: id, Type: messageType, Data: raw})
	if err != nil {
		c.t.Fatal(err)
	}
	if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
		c.t.Fatal(err)
	}
}

// answer plays the current turn and answers it, reading through to the next turn boundary.
func (c *liveClient) answer(position int32, id string, audio bool) runtime.AnswerAccepted {
	c.t.Helper()
	c.send(runtime.TypeTurnPlayed, id+"-played", runtime.TurnPlayed{Position: position})
	c.expectState(runtime.StateListening)
	if audio {
		recording := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, "opus frames"...)
		c.send(runtime.TypeAnswerAudio, id, runtime.AnswerAudio{Position: position, Final: true, MimeType: "audio/webm;codecs=opus", Audio: recording})
	} else {
		c.send(runtime.TypeAnswerText, id, runtime.AnswerText{Position: position, Text: "typed answer " + id})
	}
	c.expectState(runtime.StateProcessing)
	var accepted runtime.AnswerAccepted
	c.expect(runtime.TypeAnswerAccepted, &accepted)
	c.expectState(runtime.StateTransitioning)
	return accepted
}

// providerVoice gives a seeded voice a provider voice id, as the Admin voice sync would.
func (s *seed) providerVoice(providerVoiceID string) uuid.UUID {
	s.t.Helper()
	id := s.voice(true)
	s.exec(`UPDATE public.voices SET provider = 'azure', provider_voice_id = $2, locale = 'en-US' WHERE id = $1`, id, providerVoiceID)
	return id
}

// liveTurnRow is one conversation_turns row with the live columns.
type liveTurnRow struct {
	position   int32
	question   string
	answer     string
	kind       string
	askedAt    pgtype.Timestamptz
	answeredAt pgtype.Timestamptz
}

func (s *seed) liveTurns(interviewID uuid.UUID) []liveTurnRow {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx, `SELECT position, question, COALESCE(answer, ''), kind, asked_at, answered_at
		FROM public.conversation_turns WHERE interview_id = $1 ORDER BY position`, interviewID)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []liveTurnRow
	for rows.Next() {
		var r liveTurnRow
		if err := rows.Scan(&r.position, &r.question, &r.answer, &r.kind, &r.askedAt, &r.answeredAt); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

func (s *seed) frozenVoice(interviewID uuid.UUID) *string {
	s.t.Helper()
	var voice *string
	if err := s.pool.QueryRow(s.ctx, `SELECT provider_voice_id FROM public.interviews WHERE id = $1`, interviewID).Scan(&voice); err != nil {
		s.t.Fatal(err)
	}
	return voice
}

func TestLivePracticeInterviewEndToEnd(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	stack := newLiveStack(t, pool, service.Settings{QuestionLimit: 2})
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	providerVoiceID := "en-US-Kan52Live" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	voice := s.providerVoice(providerVoiceID)
	profile, _ := s.confirmedProfile(candidate, 3)
	started, created, err := stack.svc.Create(ctx, candidate, "candidate", domain.CreateInput{
		Type: domain.TypePractice, RoleProfileID: &profile, VoiceID: &voice,
	})
	if err != nil || !created {
		t.Fatalf("Create: created=%v err=%v", created, err)
	}
	if got := s.balance(candidate); got != 15 {
		t.Fatalf("balance after start = %d, want 15", got)
	}
	questions := s.snapshot(started.ID)
	firstText := s.contentOf(questions[0])

	c, _, err := stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	var sync runtime.SessionSync
	c.expect(runtime.TypeSessionSync, &sync)
	var turn runtime.InterviewerTurn
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if sync.QuestionCount != 2 || turn.Position != 1 || turn.Text != firstText || turn.AudioStatus != runtime.AudioReady {
		t.Fatalf("sync = %+v turn = %d %q %s", sync, turn.Position, turn.Text, turn.AudioStatus)
	}
	if calls := stack.synth.Calls(); len(calls) != 1 || calls[0].ProviderVoiceID != providerVoiceID {
		t.Fatalf("synthesizer calls = %+v", calls)
	}

	// The bank and the voice catalog change after the first delivery; the session keeps what it used.
	s.exec(`UPDATE public.core_questions SET content = 'rewritten bank text' WHERE id = $1`, questions[0])
	s.exec(`UPDATE public.voices SET provider_voice_id = $2 WHERE id = $1`, voice, providerVoiceID+"x")

	// Drop and reconnect before answering: same words, no charge.
	_ = c.ws.NetConn().Close()
	c, _, err = stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("redial: %v", err)
	}
	c.expect(runtime.TypeSessionSync, &sync)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if !sync.Resumed || turn.Text != firstText {
		t.Fatalf("after reconnect resumed=%v text=%q, want the frozen %q", sync.Resumed, turn.Text, firstText)
	}
	accepted := c.answer(1, "answer-1", true)
	if accepted.Transcript == nil || *accepted.Transcript != runtime.FakeTranscript {
		t.Fatalf("accepted = %+v", accepted)
	}
	c.expectState(runtime.StateInterviewerSpeaking)
	c.expect(runtime.TypeInterviewerTurn, &turn)

	// The same answer replayed under a new id after the turn moved on: a duplicate, nothing stored.
	c.send(runtime.TypeAnswerText, "answer-1-replay", runtime.AnswerText{Position: 1, Text: "a different text"})
	c.expect(runtime.TypeAnswerAccepted, &accepted)
	if !accepted.Duplicate {
		t.Fatalf("replay = %+v", accepted)
	}

	c.answer(2, "answer-2", false)
	c.expectState(runtime.StateCompleted)
	var completed runtime.SessionCompleted
	c.expect(runtime.TypeSessionCompleted, &completed)
	c.expectClose(websocket.CloseNormalClosure)

	select {
	case id := <-stack.evaluator.done:
		if id != started.ID {
			t.Fatalf("evaluated %s", id)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the evaluation did not start")
	}

	// A reconnect to the completed session reports it and changes nothing.
	c, _, err = stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("dial completed: %v", err)
	}
	c.expect(runtime.TypeSessionSync, &sync)
	if sync.State != runtime.StateCompleted {
		t.Fatalf("sync = %+v", sync)
	}
	c.expect(runtime.TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)

	// Charged exactly once, whatever the sockets did.
	if got := s.balance(candidate); got != 15 {
		t.Fatalf("balance = %d, want 15", got)
	}
	if rows := s.coinRows(candidate); len(rows) != 1 || rows[0].amount != 5 {
		t.Fatalf("ledger rows = %+v", rows)
	}
	turns := s.liveTurns(started.ID)
	if len(turns) != 2 {
		t.Fatalf("turns = %+v", turns)
	}
	if turns[0].question != firstText || turns[0].answer != runtime.FakeTranscript || turns[1].answer != "typed answer answer-2" {
		t.Fatalf("turn text = %+v", turns)
	}
	for _, turn := range turns {
		if turn.kind != "core" || !turn.askedAt.Valid || !turn.answeredAt.Valid || turn.answeredAt.Time.Before(turn.askedAt.Time) {
			t.Fatalf("turn %d kind=%s asked=%v answered=%v", turn.position, turn.kind, turn.askedAt, turn.answeredAt)
		}
	}
	if voice := s.frozenVoice(started.ID); voice == nil || *voice != providerVoiceID {
		t.Fatalf("frozen voice = %v, want %s", voice, providerVoiceID)
	}
	for _, call := range stack.synth.Calls() {
		if call.ProviderVoiceID != providerVoiceID {
			t.Fatalf("a turn was spoken with %s after the catalog changed, want the frozen %s", call.ProviderVoiceID, providerVoiceID)
		}
	}
	if n := s.count(`SELECT count(*) FROM public.interview_questions WHERE interview_id = $1 AND spoken_text IS NOT NULL AND asked_at IS NOT NULL`, started.ID); n != 2 {
		t.Fatalf("delivered questions = %d", n)
	}
	session, err := stack.svc.Get(ctx, candidate, "candidate", started.ID)
	if err != nil || session.CompletedAt == nil || session.AnsweredCount != 2 {
		t.Fatalf("session = %+v err = %v", session, err)
	}
}

func TestLiveRefusesAnotherCandidatesSession(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	stack := newLiveStack(t, pool, service.Settings{QuestionLimit: 2})
	owner := s.user("candidate")
	s.wallet(owner, 10)
	started, _ := practiceSession(t, ctx, s, stack.svc, owner, 2)
	stranger := s.user("candidate")

	_, resp, err := stack.dial(t, stranger, started.ID)
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("stranger dial err=%v resp=%v", err, resp)
	}
	if n := s.count(`SELECT count(*) FROM public.interview_questions WHERE interview_id = $1 AND asked_at IS NOT NULL`, started.ID); n != 0 {
		t.Fatalf("a refused socket delivered %d questions", n)
	}
	if voice := s.frozenVoice(started.ID); voice != nil {
		t.Fatalf("a refused socket froze voice %s", *voice)
	}
}

func TestLiveRecruitmentAfterTheDeadlineIsRefusedWithoutConsumingAnotherSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	stack := newLiveStack(t, pool, service.Settings{QuestionLimit: 2})
	slot := int32(2)
	_, posting, _ := s.openPosting(&slot, 3)
	candidate := s.user("candidate")
	application := s.eligibleApplication(posting, candidate)
	started, created, err := stack.svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &application})
	if err != nil || !created {
		t.Fatalf("recruitment Create created=%v err=%v", created, err)
	}

	c, _, err := stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.expect(runtime.TypeSessionSync, nil)
	c.expect(runtime.TypeInterviewerTurn, nil)
	c.send(runtime.TypeTurnPlayed, "p1", runtime.TurnPlayed{Position: 1})
	c.expectState(runtime.StateListening)
	s.setDeadlineInPast(application)
	c.send(runtime.TypeAnswerText, "late", runtime.AnswerText{Position: 1, Text: "too late"})
	c.expectState(runtime.StateProcessing)
	var failure runtime.ErrorData
	c.expect(runtime.TypeError, &failure)
	if failure.Code != "APPLICATION_NOT_ELIGIBLE" || !failure.Fatal {
		t.Fatalf("error = %+v", failure)
	}
	c.expectClose(runtime.CloseNotEligible)

	// Reconnecting is refused the same way, and never starts or charges anything.
	c, _, err = stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("redial: %v", err)
	}
	c.expect(runtime.TypeError, &failure)
	c.expectClose(runtime.CloseNotEligible)
	if n := s.count(`SELECT count(*) FROM public.interviews WHERE application_id = $1`, application); n != 1 {
		t.Fatalf("recruitment sessions = %d", n)
	}
	if n := s.count(`SELECT count(*) FROM public.conversation_turns WHERE interview_id = $1`, started.ID); n != 0 {
		t.Fatalf("a late answer was stored: %d turns", n)
	}
	if n := s.count(`SELECT count(*) FROM public.transactions WHERE "from" = $1 OR "to" = $1`, candidate.String()); n != 0 {
		t.Fatalf("the recruitment Candidate has %d ledger rows", n)
	}
}

func TestLiveDefaultVoiceIsFrozenForASessionWithoutVoice(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	stack := newLiveStack(t, pool, service.Settings{QuestionLimit: 1, DefaultProviderVoiceID: "en-US-DefaultTest"})
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	started, _ := practiceSession(t, ctx, s, stack.svc, candidate, 1)

	c, _, err := stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.expect(runtime.TypeSessionSync, nil)
	var turn runtime.InterviewerTurn
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if turn.AudioStatus != runtime.AudioReady {
		t.Fatalf("audio status = %s", turn.AudioStatus)
	}
	if voice := s.frozenVoice(started.ID); voice == nil || *voice != "en-US-DefaultTest" {
		t.Fatalf("frozen voice = %v", voice)
	}

	// Without a default, the same kind of session is text only and freezes nothing.
	plain := newLiveStack(t, pool, service.Settings{QuestionLimit: 1})
	other := s.user("candidate")
	s.wallet(other, 10)
	session, _ := practiceSession(t, ctx, s, plain.svc, other, 1)
	c, _, err = plain.dial(t, other, session.ID)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.expect(runtime.TypeSessionSync, nil)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if turn.AudioStatus != runtime.AudioNoVoice || turn.Audio != nil || s.frozenVoice(session.ID) != nil {
		t.Fatalf("turn status %s audio %v", turn.AudioStatus, turn.Audio != nil)
	}
}

func TestTurnPositionIsUniquePerSession(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 2)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 2)
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "first"); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.conversation_turns (interview_id, position, question, answer) VALUES ($1, 1, 'q', 'again')`, started.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate turn insert = %v, want a unique violation", err)
	}
	// A REST answer to a question never delivered live keeps no asked time but gets an answered time.
	turns := s.liveTurns(started.ID)
	if len(turns) != 1 || turns[0].askedAt.Valid || !turns[0].answeredAt.Valid || turns[0].kind != "core" {
		t.Fatalf("turn = %+v", turns)
	}
}
