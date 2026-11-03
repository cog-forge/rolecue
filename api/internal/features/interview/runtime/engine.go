package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Sessions is the part of the interview service the engine drives. The service owns every rule:
// ownership, the turn order, the recruitment window and completion. The engine never charges and never
// consumes a slot: those happen only when the session is started over REST.
type Sessions interface {
	Get(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error)
	Deliver(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Delivery, error)
	AppendTurn(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID, position int32, answer string) (domain.Session, error)
	Complete(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Session, error)
	// FollowUp decides after a new answer whether a follow-up is asked next; the Delivery has no
	// Question when none is. AppendFollowUp stores the answer to a follow-up this engine asked.
	FollowUp(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID) (domain.Delivery, error)
	AppendFollowUp(ctx context.Context, candidateID uuid.UUID, role string, id uuid.UUID, followUp domain.AskedQuestion, answer string) (domain.Session, error)
}

// Participant is the authenticated Candidate and the session one socket is bound to.
type Participant struct {
	CandidateID uuid.UUID
	Role        string
	SessionID   uuid.UUID
}

// Settings tune the engine. STTTimeout bounds one transcription (zero uses 30s); CacheEntries is the
// size of the synthesis cache (zero disables it).
type Settings struct {
	STTTimeout   time.Duration
	CacheEntries int
}

// Engine runs live interview sockets. One Engine serves every session of the process.
type Engine struct {
	sessions    Sessions
	synthesizer tts.Synthesizer
	transcriber Transcriber
	hub         *Hub
	cache       *synthesisCache
	sttTimeout  time.Duration
}

// NewEngine wires the engine. Every dependency is required.
func NewEngine(sessions Sessions, synthesizer tts.Synthesizer, transcriber Transcriber, hub *Hub, settings Settings) (*Engine, error) {
	if sessions == nil || synthesizer == nil || transcriber == nil || hub == nil {
		return nil, errors.New("live interview engine needs sessions, a synthesizer, a transcriber and a hub")
	}
	timeout := settings.STTTimeout
	if timeout <= 0 {
		timeout = defaultSTTTimeout
	}
	return &Engine{
		sessions: sessions, synthesizer: synthesizer, transcriber: transcriber, hub: hub,
		cache: newSynthesisCache(settings.CacheEntries), sttTimeout: timeout,
	}, nil
}

// Hub returns the engine's hub, so the process can shut live sockets down.
func (e *Engine) Hub() *Hub { return e.hub }

// Serve drives one upgraded socket until the interview completes, the socket drops, a newer socket of
// the same session takes over, or the API shuts down. It closes the socket before it returns.
func (e *Engine) Serve(ctx context.Context, ws *websocket.Conn, p Participant) {
	defer func() { _ = ws.Close() }()
	if ws.Subprotocol() != Subprotocol {
		writeClose(ws, CloseProtocolViolation, "offer the "+Subprotocol+" subprotocol")
		return
	}
	l, err := e.hub.attach(p.SessionID)
	if err != nil {
		writeClose(ws, websocket.CloseGoingAway, "the server is restarting")
		return
	}
	defer e.hub.release(l)

	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel()

	c := &conn{
		engine: e, ws: ws, p: p, lease: l, stream: l.stream,
		log: slog.With("interview_id", p.SessionID.String()),
	}
	ws.SetReadLimit(MaxMessageBytes)
	incoming := make(chan inbound)
	workers.Add(2)
	go func() {
		defer workers.Done()
		c.read(ctx, incoming)
	}()
	go func() {
		defer workers.Done()
		c.keepAlive(ctx)
	}()
	c.run(ctx, incoming)
	// Unblock the reader; the deferred Close also does, but the workers are waited for first.
	_ = ws.NetConn().Close()
}

// inbound is one frame read from the socket, or the read error that ended it.
type inbound struct {
	kind int
	data []byte
	err  error
}

// conn is the state of one socket. Only the run goroutine touches it; the reader and the pinger talk to
// it through the incoming channel and the concurrency-safe control writes.
type conn struct {
	engine *Engine
	ws     *websocket.Conn
	p      Participant
	lease  *lease
	stream *sessionStream
	log    *slog.Logger

	state          State
	sessionType    domain.Type
	questionCount  int32
	answered       int32
	current        *domain.AskedQuestion
	buffer         *answerBuffer
	protocolErrors int
	broken         bool
}

func (c *conn) read(ctx context.Context, out chan<- inbound) {
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { return c.ws.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		kind, data, err := c.ws.ReadMessage()
		if err == nil {
			_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
		}
		select {
		case out <- inbound{kind: kind, data: data, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *conn) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		}
	}
}

func (c *conn) run(ctx context.Context, incoming <-chan inbound) {
	if c.start(ctx) {
		return
	}
	for {
		select {
		case <-c.lease.superseded:
			c.close(CloseSuperseded, "another connection took over this interview")
			return
		case <-c.engine.hub.shutdown:
			c.close(websocket.CloseGoingAway, "the server is restarting")
			return
		case in := <-incoming:
			if in.err != nil {
				// A drop keeps everything already stored; a partial recording is discarded and the
				// reconnect asks the same question again.
				if !websocket.IsCloseError(in.err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					c.log.Info("live interview socket ended", "state", string(c.state), "error", in.err)
				}
				return
			}
			if c.handle(ctx, in) {
				return
			}
		}
	}
}

// start syncs a new socket and delivers the current turn. It reports whether the socket was closed.
func (c *conn) start(ctx context.Context) bool {
	c.stream.turn.Lock()
	defer c.stream.turn.Unlock()
	return c.syncAndDeliver(ctx, "")
}

// syncAndDeliver reads the session at its turn boundary, sends session.sync and the current turn, or
// completes a session whose answers are all stored. The caller holds the turn lock.
func (c *conn) syncAndDeliver(ctx context.Context, replyTo string) bool {
	delivery, err := c.engine.sessions.Deliver(ctx, c.p.CandidateID, c.p.Role, c.p.SessionID)
	if err != nil {
		if codeOf(err) == apperror.CodeInterviewCompleted {
			return c.finishCompleted(ctx, replyTo)
		}
		return c.fatal(err, replyTo)
	}
	c.apply(delivery)
	if delivery.Question == nil {
		c.state = StateTransitioning
		c.sendSync(replyTo, nil)
		return c.complete(ctx)
	}
	c.state = StateInterviewerSpeaking
	c.sendSync(replyTo, nil)
	c.send(TypeInterviewerTurn, "", c.performTurn(ctx, delivery))
	return false
}

// apply takes the session's position from a delivery.
func (c *conn) apply(d domain.Delivery) {
	c.sessionType, c.questionCount, c.answered, c.current = d.Type, d.QuestionCount, d.AnsweredCount, d.Question
}

// handle runs one client frame. It reports whether the socket was closed.
func (c *conn) handle(ctx context.Context, in inbound) bool {
	if in.kind != websocket.TextMessage {
		return c.protocolError("", "only JSON text frames are accepted")
	}
	var msg ClientMessage
	if err := decodeStrict(in.data, &msg); err != nil || msg.V != ProtocolVersion || msg.Type == "" {
		return c.protocolError("", "malformed message or unsupported version")
	}
	if msg.ID != "" && !commandIDPattern.MatchString(msg.ID) {
		return c.protocolError("", "invalid message id")
	}
	switch msg.Type {
	case TypePing:
		if !emptyData(msg.Data) {
			return c.protocolError(msg.ID, "ping carries no data")
		}
		c.send(TypePong, msg.ID, empty{})
		return false
	case TypeTurnPlayed, TypeAnswerText, TypeAnswerAudio:
	default:
		return c.protocolError(msg.ID, "unknown message type")
	}
	if msg.ID == "" {
		return c.protocolError("", msg.Type+" needs an id")
	}
	if c.stream.accepted(msg.ID) {
		c.send(TypeAck, msg.ID, Ack{Duplicate: true})
		return false
	}
	switch msg.Type {
	case TypeTurnPlayed:
		var played TurnPlayed
		if decodeStrict(msg.Data, &played) != nil {
			return c.protocolError(msg.ID, "invalid turn.played data")
		}
		c.turnPlayed(msg.ID, played)
		return false
	case TypeAnswerText:
		var answer AnswerText
		if decodeStrict(msg.Data, &answer) != nil {
			return c.protocolError(msg.ID, "invalid answer.text data")
		}
		if !c.answerAllowed(msg.ID, answer.Position) {
			return false
		}
		return c.processAnswer(ctx, msg.ID, answer.Position, nil, answer.Text)
	default:
		var chunk AnswerAudio
		if decodeStrict(msg.Data, &chunk) != nil {
			return c.protocolError(msg.ID, "invalid answer.audio data")
		}
		return c.answerChunk(ctx, msg.ID, chunk)
	}
}

func (c *conn) turnPlayed(id string, played TurnPlayed) {
	switch {
	case c.current != nil && played.Position < c.current.Position, c.current == nil && played.Position <= c.answered:
		c.send(TypeAck, id, Ack{Duplicate: true})
	case c.current == nil || played.Position != c.current.Position:
		c.sendError(id, CodeInvalidState, "no interviewer turn is playing at that position", false, false)
	case c.state == StateListening:
		c.send(TypeAck, id, Ack{Duplicate: true})
	case c.state == StateInterviewerSpeaking:
		c.stream.accept(id)
		c.setState(StateListening, id)
	default:
		c.sendError(id, CodeInvalidState, "the interviewer turn cannot end now", false, false)
	}
}

// answerAllowed applies the durable position guard and the state rule to a new answer. An answer for a
// stored position is acknowledged as a duplicate without any work.
func (c *conn) answerAllowed(id string, position int32) bool {
	if position >= 1 && position <= c.answered {
		c.send(TypeAnswerAccepted, id, AnswerAccepted{Position: position, AnsweredCount: c.answered, Duplicate: true})
		return false
	}
	if c.current == nil || position != c.current.Position {
		c.sendError(id, string(apperror.CodeInterviewTurnConflict), "the answer is not for the current question", false, false)
		c.sendSync("", nil)
		return false
	}
	if c.state != StateListening {
		c.sendError(id, CodeInvalidState, "answers are accepted only while listening", false, false)
		return false
	}
	return true
}

// answerChunk adds one chunk of a recorded answer and processes the answer on its final chunk.
func (c *conn) answerChunk(ctx context.Context, id string, chunk AnswerAudio) bool {
	if c.buffer == nil || c.buffer.id != id {
		c.buffer = nil
		if !c.answerAllowed(id, chunk.Position) {
			return false
		}
		mediaType, ok := normalizeAudioType(chunk.MimeType)
		switch {
		case chunk.Chunk != 0 || chunk.MimeType == "":
			c.sendError(id, CodeAudioChunkOrder, "a recording starts with chunk 0 and its mime_type", true, false)
			return false
		case !ok:
			c.sendError(id, CodeAudioFormatUnsupported, "the recording format is not supported", true, false)
			return false
		}
		c.buffer = &answerBuffer{id: id, position: chunk.Position, mediaType: mediaType}
	}
	result, code := c.buffer.add(chunk)
	switch {
	case code != "":
		c.buffer = nil
		c.sendError(id, code, "the recording was discarded; record the answer again", true, false)
		return false
	case result != chunkComplete:
		return false
	}
	audio := Audio{Data: c.buffer.data, MimeType: c.buffer.mediaType}
	c.buffer = nil
	return c.processAnswer(ctx, id, chunk.Position, &audio, "")
}

// processAnswer transcribes and stores one answer, then moves to the next turn. The work runs on a
// context detached from the socket: an answer that fully arrived is stored even if the socket drops
// meanwhile, so the reconnect continues after it.
func (c *conn) processAnswer(ctx context.Context, id string, position int32, audio *Audio, text string) bool {
	c.stream.turn.Lock()
	defer c.stream.turn.Unlock()
	c.setState(StateProcessing, id)

	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.engine.sttTimeout+persistTimeout)
	defer cancel()
	if audio != nil {
		transcript, err := c.transcribe(work, *audio)
		if err != nil {
			code, message := CodeSTTFailed, "the answer could not be transcribed; answer again"
			switch {
			case errors.Is(err, ErrNoSpeech):
				code, message = CodeSTTNoSpeech, "no speech was recognized; answer again"
			case errors.Is(err, ErrUnsupportedAudio):
				code, message = CodeAudioFormatUnsupported, "the recording could not be decoded; record the answer again"
			case errors.Is(err, ErrAnswerTooLong):
				code, message = CodeAnswerTooLong, "the answer is longer than 3 minutes; answer again more briefly"
			default:
				c.log.Warn("live interview transcription failed", "error", err)
			}
			c.retryAnswer(id, code, message)
			return false
		}
		text = transcript
	}

	var session domain.Session
	var err error
	if c.current.Kind == domain.TurnFollowUp {
		// The follow-up exists only in this socket; it is stored with its answer.
		session, err = c.engine.sessions.AppendFollowUp(work, c.p.CandidateID, c.p.Role, c.p.SessionID, *c.current, text)
	} else {
		session, err = c.engine.sessions.AppendTurn(work, c.p.CandidateID, c.p.Role, c.p.SessionID, position, text)
	}
	if err != nil {
		return c.answerFailed(work, id, err)
	}
	c.stream.accept(id)
	stored := session.AnsweredTurns > c.answered
	c.answered = session.AnsweredTurns
	accepted := AnswerAccepted{Position: position, AnsweredCount: session.AnsweredTurns}
	if session.Type == domain.TypePractice {
		// Only a practice Candidate sees their transcript; a recruitment transcript is Recruiter-only.
		stored := strings.TrimSpace(text)
		accepted.Transcript = &stored
	}
	c.send(TypeAnswerAccepted, id, accepted)
	// Only a newly stored answer can lead to a follow-up; a retry that changed nothing never does.
	return c.advance(work, stored)
}

func (c *conn) transcribe(ctx context.Context, audio Audio) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.engine.sttTimeout)
	defer cancel()
	transcript, err := c.engine.transcriber.Transcribe(ctx, audio)
	if err != nil {
		return "", err
	}
	if transcript.Duration > MaxAnswerDuration+answerDurationGrace {
		return "", ErrAnswerTooLong
	}
	if strings.TrimSpace(transcript.Text) == "" {
		return "", ErrNoSpeech
	}
	return transcript.Text, nil
}

// retryAnswer reports a recoverable answer failure and listens again.
func (c *conn) retryAnswer(id, code, message string) {
	c.sendError(id, code, message, true, false)
	c.setState(StateListening, id)
}

// answerFailed maps a storage failure of an answer. It reports whether the socket was closed.
func (c *conn) answerFailed(ctx context.Context, id string, err error) bool {
	switch codeOf(err) {
	case apperror.CodeInterviewCompleted:
		return c.finishCompleted(ctx, id)
	case apperror.CodeInterviewTurnConflict:
		// The stored session moved on (another socket answered first): resend the truth.
		c.sendError(id, string(apperror.CodeInterviewTurnConflict), "the session moved on; continuing from the stored state", false, false)
		return c.syncAndDeliver(ctx, id)
	case apperror.CodeValidation:
		c.retryAnswer(id, string(apperror.CodeValidation), appMessage(err))
		return false
	case apperror.CodeApplicationNotEligible, apperror.CodeInterviewNotFound, apperror.CodeForbidden:
		return c.fatal(err, id)
	}
	c.log.Error("live interview answer could not be stored", "error", err)
	c.retryAnswer(id, string(apperror.CodeInternal), "the answer could not be stored; send it again")
	return false
}

// advance moves past a stored answer: a follow-up when one is decided (only after a new answer), else
// the next core question, or completion after the last one. A superseded socket stops here and leaves
// the next turn to the socket that took over.
func (c *conn) advance(ctx context.Context, newAnswer bool) bool {
	select {
	case <-c.lease.superseded:
		c.close(CloseSuperseded, "another connection took over this interview")
		return true
	default:
	}
	c.setState(StateTransitioning, "")
	var delivery domain.Delivery
	var err error
	if newAnswer {
		delivery, err = c.engine.sessions.FollowUp(ctx, c.p.CandidateID, c.p.Role, c.p.SessionID)
	}
	if err == nil && delivery.Question == nil {
		delivery, err = c.engine.sessions.Deliver(ctx, c.p.CandidateID, c.p.Role, c.p.SessionID)
	}
	if err != nil {
		if codeOf(err) == apperror.CodeInterviewCompleted {
			return c.finishCompleted(ctx, "")
		}
		return c.fatal(err, "")
	}
	c.apply(delivery)
	if delivery.Question == nil {
		return c.complete(ctx)
	}
	turn := c.performTurn(ctx, delivery)
	c.setState(StateInterviewerSpeaking, "")
	c.send(TypeInterviewerTurn, "", turn)
	return false
}

// complete completes the session after its last answer and closes the socket with 1000. The service
// starts the evaluation once.
func (c *conn) complete(ctx context.Context) bool {
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	session, err := c.engine.sessions.Complete(work, c.p.CandidateID, c.p.Role, c.p.SessionID)
	if err != nil {
		return c.fatal(err, "")
	}
	c.current = nil
	c.setState(StateCompleted, "")
	c.send(TypeSessionCompleted, "", SessionCompleted{EvaluationStatus: string(session.EvaluationStatus)})
	c.close(websocket.CloseNormalClosure, "interview completed")
	return true
}

// finishCompleted reports a session that is already completed and closes with 1000.
func (c *conn) finishCompleted(ctx context.Context, replyTo string) bool {
	session, err := c.engine.sessions.Get(ctx, c.p.CandidateID, c.p.Role, c.p.SessionID)
	if err != nil {
		return c.fatal(err, replyTo)
	}
	status := string(session.EvaluationStatus)
	c.state, c.current = StateCompleted, nil
	c.sessionType, c.questionCount, c.answered = session.Type, session.QuestionCount, session.AnsweredTurns
	c.sendSync(replyTo, &status)
	c.send(TypeSessionCompleted, "", SessionCompleted{EvaluationStatus: status})
	c.close(websocket.CloseNormalClosure, "interview completed")
	return true
}

// fatal reports an error that ends the socket. A closed recruitment window or a session that is no
// longer the Candidate's closes with 4003; anything else is internal and closes with 1011, and the
// client reconnects.
func (c *conn) fatal(err error, replyTo string) bool {
	switch code := codeOf(err); code {
	case apperror.CodeApplicationNotEligible, apperror.CodeInterviewNotFound, apperror.CodeForbidden:
		c.sendError(replyTo, string(code), appMessage(err), false, true)
		c.close(CloseNotEligible, "this interview no longer accepts answers")
	default:
		c.log.Error("live interview failed", "error", err)
		c.sendError(replyTo, string(apperror.CodeInternal), "the interview could not continue; reconnect", true, true)
		c.close(websocket.CloseInternalServerErr, "internal error")
	}
	return true
}

// performTurn builds the interviewer turn with synthesized speech. A missing voice or a failed synthesis
// sends the turn as text only; the interview never stops for speech.
func (c *conn) performTurn(ctx context.Context, d domain.Delivery) InterviewerTurn {
	q := d.Question
	turn := InterviewerTurn{
		Position: q.Position, QuestionNumber: q.QuestionNumber, Kind: string(q.Kind), QuestionCount: d.QuestionCount, Text: q.Text,
		AudioStatus: AudioNoVoice, Visemes: []Viseme{}, BlendShapes: []BlendShapeFrame{}, Words: []WordTiming{}, Cues: []Cue{},
	}
	if d.ProviderVoiceID == "" {
		return turn
	}
	result, ok := c.engine.synthesize(ctx, c.log, d.ProviderVoiceID, q.Text)
	if !ok {
		turn.AudioStatus = AudioUnavailable
		return turn
	}
	turn.AudioStatus = AudioReady
	turn.Audio = &TurnAudio{MimeType: result.MimeType, Data: result.Audio}
	for _, v := range result.Visemes {
		turn.Visemes = append(turn.Visemes, Viseme{OffsetMs: v.OffsetMs, ID: v.ID})
	}
	for _, f := range result.BlendShapes {
		turn.BlendShapes = append(turn.BlendShapes, BlendShapeFrame{OffsetMs: f.OffsetMs, Values: roundWeights(f.Values)})
	}
	for _, w := range result.Words {
		turn.Words = append(turn.Words, WordTiming{OffsetMs: w.OffsetMs, DurationMs: w.DurationMs, Text: w.Text})
	}
	return turn
}

// roundWeights keeps three decimals of each blend-shape weight. The avatar cannot show finer steps, and a
// float32 widened to float64 would otherwise print about 17 digits per value in JSON, multiplying the size
// of a 60 FPS, 55-value timeline several times over.
func roundWeights(values []float32) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = math.Round(float64(v)*1000) / 1000
	}
	return out
}

// synthesize speaks one text, through the cache. It reports false on any failure, which the caller turns
// into a text-only turn.
func (e *Engine) synthesize(ctx context.Context, log *slog.Logger, voice, text string) (tts.Result, bool) {
	if cached, ok := e.cache.get(voice, text); ok {
		return cached, true
	}
	ctx, cancel := context.WithTimeout(ctx, synthesisTimeout)
	defer cancel()
	result, err := e.synthesizer.Synthesize(ctx, tts.Request{Text: text, ProviderVoiceID: voice})
	if err != nil {
		log.Warn("interviewer speech unavailable; sending the turn as text", "error", err)
		return tts.Result{}, false
	}
	if len(result.Audio) == 0 || len(result.Audio) > MaxInterviewerAudio {
		log.Warn("interviewer speech unusable; sending the turn as text", "audio_bytes", len(result.Audio))
		return tts.Result{}, false
	}
	e.cache.put(voice, text, result)
	return result, true
}

func (c *conn) setState(state State, replyTo string) {
	c.state = state
	c.send(TypeState, replyTo, StateChange{State: state, Position: c.openPosition()})
}

func (c *conn) sendSync(replyTo string, evaluationStatus *string) {
	c.send(TypeSessionSync, replyTo, SessionSync{
		SessionID: c.p.SessionID.String(), SessionType: string(c.sessionType), State: c.state,
		QuestionCount: c.questionCount, AnsweredCount: c.answered, Position: c.openPosition(),
		QuestionNumber: c.openQuestionNumber(), Resumed: c.lease.resumed, EvaluationStatus: evaluationStatus,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	})
}

// openPosition is the current question's position while one is open.
func (c *conn) openPosition() *int32 {
	if c.current == nil || c.state == StateTransitioning || c.state == StateCompleted {
		return nil
	}
	position := c.current.Position
	return &position
}

// openQuestionNumber is the current question's core question number while one is open.
func (c *conn) openQuestionNumber() *int32 {
	if c.openPosition() == nil {
		return nil
	}
	number := c.current.QuestionNumber
	return &number
}

// sendError sends an error message. A fatal one is followed by a close.
func (c *conn) sendError(replyTo, code, message string, retryable, fatal bool) {
	c.send(TypeError, replyTo, ErrorData{Code: code, Message: message, Retryable: retryable, Fatal: fatal})
}

func (c *conn) protocolError(replyTo, message string) bool {
	c.protocolErrors++
	fatal := c.protocolErrors >= maxProtocolErrors
	c.sendError(replyTo, CodeProtocolError, message, false, fatal)
	if fatal {
		c.close(CloseProtocolViolation, "too many protocol errors")
	}
	return fatal
}

// send stamps the next sequence number of the session's stream and writes one message. After a failed
// write the socket is treated as gone; the reader then ends the loop.
func (c *conn) send(messageType, replyTo string, data any) {
	if c.broken {
		return
	}
	payload, err := json.Marshal(ServerMessage{
		V: ProtocolVersion, Stream: c.stream.id, Seq: c.stream.nextSeq(), Type: messageType, ReplyTo: replyTo, Data: data,
	})
	if err != nil {
		c.log.Error("live interview message could not be encoded", "type", messageType, "error", err)
		return
	}
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
		c.broken = true
	}
}

func (c *conn) close(code int, reason string) {
	if c.broken {
		return
	}
	writeClose(c.ws, code, reason)
	c.broken = true
}

// writeClose sends a close frame; the caller closes the connection afterwards.
func writeClose(ws *websocket.Conn, code int, reason string) {
	if len(reason) > closeReasonMaxBytes {
		reason = reason[:closeReasonMaxBytes]
	}
	_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(writeTimeout))
}

var commandIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,` + fmt.Sprint(maxCommandIDLength) + `}$`)

// decodeStrict decodes exactly one JSON value and refuses unknown fields.
func decodeStrict(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("empty JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data")
	}
	return nil
}

// emptyData reports whether a message carries no data: absent, null or {}.
func emptyData(data json.RawMessage) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}"))
}

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func appMessage(err error) string {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "request failed"
}
