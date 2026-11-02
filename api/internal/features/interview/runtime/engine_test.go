package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gorilla/websocket"
)

func TestFullPracticeInterviewDrivesEveryStateInOrder(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Tell me about yourself.", "Why this role?")
	h := newHarness(t, sessions)

	c, sync, turn := h.connect()
	if sync.State != StateInterviewerSpeaking || sync.Position == nil || *sync.Position != 1 || sync.QuestionCount != 2 ||
		sync.AnsweredCount != 0 || sync.Resumed || sync.SessionType != "practice" || sync.EvaluationStatus != nil {
		t.Fatalf("sync = %+v", sync)
	}
	if turn.Position != 1 || turn.Kind != "core" || turn.Text != "Tell me about yourself." || turn.QuestionCount != 2 {
		t.Fatalf("turn = %+v", turn)
	}
	if turn.AudioStatus != AudioReady || turn.Audio == nil || turn.Audio.MimeType != "audio/wav" || len(turn.Audio.Data) == 0 || len(turn.Visemes) == 0 {
		t.Fatalf("turn audio = %s, %d visemes", turn.AudioStatus, len(turn.Visemes))
	}
	if len(turn.BlendShapes) == 0 || len(turn.BlendShapes[0].Values) != tts.BlendShapeCount || len(turn.Words) == 0 {
		t.Fatalf("turn timelines = %d blend-shape frames, %d words", len(turn.BlendShapes), len(turn.Words))
	}

	// A recording sent in two chunks.
	c.listen("played-1", 1)
	recording := webm(strings.Repeat("a", 1000))
	c.send(TypeAnswerAudio, "answer-1", AnswerAudio{Position: 1, Chunk: 0, MimeType: "audio/webm;codecs=opus", Audio: recording[:500]})
	c.send(TypeAnswerAudio, "answer-1", AnswerAudio{Position: 1, Chunk: 1, Final: true, Audio: recording[500:]})
	c.expectState(StateProcessing)
	var accepted AnswerAccepted
	msg := c.expect(TypeAnswerAccepted, &accepted)
	if msg.ReplyTo != "answer-1" || accepted.Position != 1 || accepted.AnsweredCount != 1 || accepted.Duplicate ||
		accepted.Transcript == nil || *accepted.Transcript != FakeTranscript {
		t.Fatalf("accepted = %+v reply_to %q", accepted, msg.ReplyTo)
	}
	c.expectState(StateTransitioning)
	c.expectState(StateInterviewerSpeaking)
	c.expect(TypeInterviewerTurn, &turn)
	if turn.Position != 2 || turn.Text != "Why this role?" {
		t.Fatalf("second turn = %+v", turn)
	}

	// A typed answer for the last question completes the session.
	c.listen("played-2", 2)
	c.send(TypeAnswerText, "answer-2", AnswerText{Position: 2, Text: "  Because I like it.  "})
	c.expectState(StateProcessing)
	c.expect(TypeAnswerAccepted, &accepted)
	if *accepted.Transcript != "Because I like it." {
		t.Fatalf("transcript = %q", *accepted.Transcript)
	}
	c.expectState(StateTransitioning)
	c.expectState(StateCompleted)
	var completed SessionCompleted
	c.expect(TypeSessionCompleted, &completed)
	if completed.EvaluationStatus != "pending" {
		t.Fatalf("completed = %+v", completed)
	}
	c.expectClose(websocket.CloseNormalClosure)

	// Ordering: one stream, and seq grows by exactly one per message.
	for i, m := range c.seen {
		if m.V != ProtocolVersion || m.Stream != c.seen[0].Stream || m.Seq != int64(i+1) {
			t.Fatalf("message %d (%s): v=%d stream=%s seq=%d", i, m.Type, m.V, m.Stream, m.Seq)
		}
	}
	if answers, appends, completes := sessions.counts(); answers != 2 || appends != 2 || completes != 1 {
		t.Fatalf("answers=%d appends=%d completes=%d", answers, appends, completes)
	}
	if got := sessions.answers[0]; got != FakeTranscript {
		t.Fatalf("stored answer = %q", got)
	}
}

func TestRecruitmentAnswerNeverCarriesTheTranscript(t *testing.T) {
	h := newHarness(t, newFakeSessions(domain.TypeRecruitment, "Q1", "Q2"))
	c, sync, _ := h.connect()
	if sync.SessionType != "recruitment" {
		t.Fatalf("sync = %+v", sync)
	}
	c.listen("p1", 1)
	c.send(TypeAnswerText, "a1", AnswerText{Position: 1, Text: "secret recruitment answer"})
	c.expectState(StateProcessing)
	msg := c.expect(TypeAnswerAccepted, nil)
	if bytes.Contains(msg.Data, []byte("transcript")) || bytes.Contains(msg.Data, []byte("secret")) {
		t.Fatalf("recruitment answer.accepted leaks the transcript: %s", msg.Data)
	}
}

func TestTTSFailureSendsTheTurnAsText(t *testing.T) {
	synth := tts.NewFake(tts.WithFakeError(tts.ErrUnavailable))
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Explain caching."), withSynthesizer(synth))
	_, _, turn := h.connect()
	if turn.Text != "Explain caching." || turn.Audio != nil || turn.AudioStatus != AudioUnavailable {
		t.Fatalf("turn = %+v", turn)
	}
	if len(synth.Calls()) != 1 {
		t.Fatalf("synthesizer calls = %d", len(synth.Calls()))
	}
}

func TestSessionWithoutVoiceGetsTextOnlyTurnsAndEmptyArrays(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1")
	sessions.voice = ""
	synth := tts.NewFake()
	h := newHarness(t, sessions, withSynthesizer(synth))
	c := h.dial()
	c.expect(TypeSessionSync, nil)
	msg := c.expect(TypeInterviewerTurn, nil)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg.Data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"visemes", "blend_shapes", "words", "cues"} {
		if string(raw[key]) != "[]" {
			t.Fatalf("%s = %s, want []", key, raw[key])
		}
	}
	if string(raw["audio"]) != "null" || string(raw["audio_status"]) != `"no_voice"` || len(synth.Calls()) != 0 {
		t.Fatalf("audio = %s status = %s calls = %d", raw["audio"], raw["audio_status"], len(synth.Calls()))
	}
}

func TestDuplicateAndReplayedEventsNeverAdvanceTwice(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1", "Q2", "Q3")
	stt := NewFakeTranscriber()
	h := newHarness(t, sessions, withTranscriber(stt))
	c, _, _ := h.connect()

	c.listen("played-1", 1)
	// The same turn.played again: acknowledged, no state change.
	c.send(TypeTurnPlayed, "played-1", TurnPlayed{Position: 1})
	var ack Ack
	if msg := c.expect(TypeAck, &ack); !ack.Duplicate || msg.ReplyTo != "played-1" {
		t.Fatalf("ack = %+v", ack)
	}

	c.send(TypeAnswerAudio, "answer-1", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("x")})
	c.expectState(StateProcessing)
	c.expect(TypeAnswerAccepted, nil)
	c.expectState(StateTransitioning)
	c.expectState(StateInterviewerSpeaking)
	c.expect(TypeInterviewerTurn, nil)

	// The same message id replayed: acknowledged without STT or storage.
	c.send(TypeAnswerAudio, "answer-1", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("x")})
	c.expect(TypeAck, &ack)
	// The same answer replayed under a new id: the stored position makes it a duplicate.
	c.send(TypeAnswerAudio, "answer-1-retry", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("y")})
	var accepted AnswerAccepted
	c.expect(TypeAnswerAccepted, &accepted)
	if !accepted.Duplicate || accepted.Position != 1 || accepted.AnsweredCount != 1 || accepted.Transcript != nil {
		t.Fatalf("replayed answer = %+v", accepted)
	}
	c.send(TypeAnswerText, "answer-1-text", AnswerText{Position: 1, Text: "different"})
	c.expect(TypeAnswerAccepted, &accepted)
	if !accepted.Duplicate {
		t.Fatalf("typed replay = %+v", accepted)
	}
	// An old turn.played is a duplicate too.
	c.send(TypeTurnPlayed, "played-1-again", TurnPlayed{Position: 1})
	c.expect(TypeAck, &ack)

	c.send(TypePing, "", nil)
	c.expect(TypePong, nil)
	if answers, appends, _ := sessions.counts(); answers != 1 || appends != 1 || len(stt.Calls()) != 1 {
		t.Fatalf("answers=%d appends=%d stt calls=%d", answers, appends, len(stt.Calls()))
	}
}

func TestReconnectRestoresStateAndSequence(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1", "Q2", "Q3")
	h := newHarness(t, sessions)
	first, _, _ := h.connect()
	first.listen("p1", 1)
	first.send(TypeAnswerText, "a1", AnswerText{Position: 1, Text: "one"})
	first.expectState(StateProcessing)
	first.expect(TypeAnswerAccepted, nil)
	first.expectState(StateTransitioning)
	first.expectState(StateInterviewerSpeaking)
	var before InterviewerTurn
	last := first.expect(TypeInterviewerTurn, &before)
	first.listen("p2", 2)
	// Abnormal drop: no close frame.
	_ = first.ws.NetConn().Close()

	second := h.dial()
	var sync SessionSync
	msg := second.expect(TypeSessionSync, &sync)
	if msg.Stream != last.Stream || msg.Seq <= last.Seq {
		t.Fatalf("sync stream/seq = %s/%d, last before drop %s/%d", msg.Stream, msg.Seq, last.Stream, last.Seq)
	}
	if !sync.Resumed || sync.AnsweredCount != 1 || sync.Position == nil || *sync.Position != 2 || sync.State != StateInterviewerSpeaking {
		t.Fatalf("sync = %+v", sync)
	}
	var again InterviewerTurn
	turnMsg := second.expect(TypeInterviewerTurn, &again)
	if again.Position != 2 || again.Text != before.Text || turnMsg.Seq != msg.Seq+1 {
		t.Fatalf("re-sent turn = %+v seq %d", again, turnMsg.Seq)
	}
	// The command id accepted before the drop is still remembered.
	second.send(TypeAnswerText, "a1", AnswerText{Position: 1, Text: "one"})
	var ack Ack
	second.expect(TypeAck, &ack)
	if !ack.Duplicate {
		t.Fatal("remembered id not acknowledged as duplicate")
	}
}

func TestAbnormalDisconnectDuringProcessingStillStoresTheAnswer(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1", "Q2")
	gate := &gateTranscriber{started: make(chan struct{}, 1), release: make(chan struct{})}
	h := newHarness(t, sessions, withTranscriber(gate))
	c, _, _ := h.connect()
	c.listen("p1", 1)
	c.send(TypeAnswerAudio, "a1", AnswerAudio{Position: 1, Final: true, MimeType: "audio/ogg", Audio: []byte("OggS-voice")})
	c.expectState(StateProcessing)
	<-gate.started
	_ = c.ws.NetConn().Close()
	close(gate.release)

	select {
	case got := <-sessions.appended:
		if got != 1 {
			t.Fatalf("answered = %d", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the answer that fully arrived was not stored after the drop")
	}
	next := h.dial()
	var sync SessionSync
	next.expect(TypeSessionSync, &sync)
	if sync.AnsweredCount != 1 || *sync.Position != 2 {
		t.Fatalf("sync after drop = %+v", sync)
	}
}

func TestPartialRecordingIsDiscardedOnDrop(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1")
	h := newHarness(t, sessions)
	c, _, _ := h.connect()
	c.listen("p1", 1)
	c.send(TypeAnswerAudio, "a1", AnswerAudio{Position: 1, MimeType: "audio/webm", Audio: webm("part")})
	c.send(TypePing, "", nil)
	c.expect(TypePong, nil)
	_ = c.ws.NetConn().Close()

	next := h.dial()
	var sync SessionSync
	next.expect(TypeSessionSync, &sync)
	if sync.AnsweredCount != 0 || *sync.Position != 1 {
		t.Fatalf("sync = %+v", sync)
	}
	if answers, appends, _ := sessions.counts(); answers != 0 || appends != 0 {
		t.Fatalf("answers=%d appends=%d", answers, appends)
	}
}

func TestNewConnectionSupersedesTheOldOne(t *testing.T) {
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"))
	old, _, _ := h.connect()
	fresh := h.dial()
	var sync SessionSync
	fresh.expect(TypeSessionSync, &sync)
	if !sync.Resumed {
		t.Fatalf("sync = %+v", sync)
	}
	old.expectClose(CloseSuperseded)
}

func TestRecruitmentDeadlineClosesTheSocket(t *testing.T) {
	t.Run("on connect", func(t *testing.T) {
		sessions := newFakeSessions(domain.TypeRecruitment, "Q1")
		sessions.notEligible = true
		c := newHarness(t, sessions).dial()
		if data := c.expectError(string(apperror.CodeApplicationNotEligible)); !data.Fatal || data.Retryable {
			t.Fatalf("error = %+v", data)
		}
		c.expectClose(CloseNotEligible)
	})
	t.Run("while answering", func(t *testing.T) {
		sessions := newFakeSessions(domain.TypeRecruitment, "Q1", "Q2")
		h := newHarness(t, sessions)
		c, _, _ := h.connect()
		c.listen("p1", 1)
		sessions.set(func(f *fakeSessions) { f.notEligible = true })
		c.send(TypeAnswerText, "a1", AnswerText{Position: 1, Text: "late"})
		c.expectState(StateProcessing)
		c.expectError(string(apperror.CodeApplicationNotEligible))
		c.expectClose(CloseNotEligible)
		if answers, _, completes := sessions.counts(); answers != 0 || completes != 0 {
			t.Fatalf("answers=%d completes=%d", answers, completes)
		}
	})
}

func TestCompletedSessionSyncsAndClosesWithoutCompletingAgain(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1")
	sessions.answers = []string{"done"}
	sessions.completed = true
	c := newHarness(t, sessions).dial()
	var sync SessionSync
	c.expect(TypeSessionSync, &sync)
	if sync.State != StateCompleted || sync.Position != nil || sync.EvaluationStatus == nil || *sync.EvaluationStatus != "pending" {
		t.Fatalf("sync = %+v", sync)
	}
	c.expect(TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)
	if _, _, completes := sessions.counts(); completes != 0 {
		t.Fatalf("completes = %d", completes)
	}
}

func TestFullyAnsweredSessionCompletesOnConnect(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1")
	sessions.answers = []string{"stored before the drop"}
	c := newHarness(t, sessions).dial()
	var sync SessionSync
	c.expect(TypeSessionSync, &sync)
	if sync.State != StateTransitioning || sync.Position != nil {
		t.Fatalf("sync = %+v", sync)
	}
	c.expectState(StateCompleted)
	c.expect(TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)
	if _, _, completes := sessions.counts(); completes != 1 {
		t.Fatalf("completes = %d", completes)
	}
}

func TestAnswerRules(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1", "Q2")
	stt := NewFakeTranscriber()
	h := newHarness(t, sessions, withTranscriber(stt))
	c, _, _ := h.connect()

	// Not listening yet.
	c.send(TypeAnswerText, "early", AnswerText{Position: 1, Text: "too soon"})
	c.expectError(CodeInvalidState)
	c.listen("p1", 1)

	// A later position than the current one.
	c.send(TypeAnswerText, "ahead", AnswerText{Position: 2, Text: "ahead"})
	c.expectError(string(apperror.CodeInterviewTurnConflict))
	var sync SessionSync
	c.expect(TypeSessionSync, &sync)
	if sync.State != StateListening || *sync.Position != 1 {
		t.Fatalf("resync = %+v", sync)
	}

	for _, tc := range []struct {
		name  string
		chunk AnswerAudio
		code  string
	}{
		{"unknown type", AnswerAudio{Position: 1, Final: true, MimeType: "audio/flac", Audio: webm("x")}, CodeAudioFormatUnsupported},
		{"missing type", AnswerAudio{Position: 1, Final: true, Audio: webm("x")}, CodeAudioChunkOrder},
		{"starts late", AnswerAudio{Position: 1, Chunk: 1, MimeType: "audio/webm", Audio: webm("x")}, CodeAudioChunkOrder},
		{"wrong bytes", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: []byte("OggS")}, CodeAudioFormatUnsupported},
		{"chunk too large", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm(strings.Repeat("z", MaxAudioChunkBytes))}, CodeAudioTooLarge},
	} {
		c.send(TypeAnswerAudio, "bad-"+strings.ReplaceAll(tc.name, " ", "-"), tc.chunk)
		if data := c.expectError(tc.code); !data.Retryable {
			t.Fatalf("%s: error = %+v", tc.name, data)
		}
	}

	// Silence, then a failing transcriber: both keep listening.
	stt.SetText("")
	c.send(TypeAnswerAudio, "silent", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("x")})
	c.expectState(StateProcessing)
	c.expectError(CodeSTTNoSpeech)
	c.expectState(StateListening)
	stt.SetText(FakeTranscript)
	stt.SetError(errors.New("whisper crashed"))
	c.send(TypeAnswerAudio, "broken", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("x")})
	c.expectState(StateProcessing)
	c.expectError(CodeSTTFailed)
	c.expectState(StateListening)
	stt.SetError(nil)

	// A recording longer than the 3-minute limit (plus grace) is refused; one at the limit is not.
	stt.SetDuration(MaxAnswerDuration + answerDurationGrace + time.Second)
	c.send(TypeAnswerAudio, "long", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("x")})
	c.expectState(StateProcessing)
	if data := c.expectError(CodeAnswerTooLong); !data.Retryable {
		t.Fatalf("too long: error = %+v", data)
	}
	c.expectState(StateListening)
	stt.SetDuration(MaxAnswerDuration)

	// An empty typed answer is a validation error from the service.
	c.send(TypeAnswerText, "blank", AnswerText{Position: 1, Text: "   "})
	c.expectState(StateProcessing)
	c.expectError(string(apperror.CodeValidation))
	c.expectState(StateListening)

	// A failed command may be retried with the same id.
	c.send(TypeAnswerAudio, "broken", AnswerAudio{Position: 1, Final: true, MimeType: "audio/webm", Audio: webm("x")})
	c.expectState(StateProcessing)
	var accepted AnswerAccepted
	c.expect(TypeAnswerAccepted, &accepted)
	if accepted.Duplicate || accepted.AnsweredCount != 1 {
		t.Fatalf("accepted = %+v", accepted)
	}
	if answers, _, _ := sessions.counts(); answers != 1 {
		t.Fatalf("answers = %d", answers)
	}
}

func TestStorageFailureKeepsListening(t *testing.T) {
	sessions := newFakeSessions(domain.TypePractice, "Q1")
	h := newHarness(t, sessions)
	c, _, _ := h.connect()
	c.listen("p1", 1)
	sessions.set(func(f *fakeSessions) { f.appendErr = apperror.New(apperror.CodeInternal, "database down") })
	c.send(TypeAnswerText, "a1", AnswerText{Position: 1, Text: "answer"})
	c.expectState(StateProcessing)
	if data := c.expectError(string(apperror.CodeInternal)); !data.Retryable || data.Fatal {
		t.Fatalf("error = %+v", data)
	}
	c.expectState(StateListening)
}

func TestProtocolErrors(t *testing.T) {
	t.Run("missing subprotocol", func(t *testing.T) {
		h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"))
		c := h.dial("another.protocol")
		c.expectClose(CloseProtocolViolation)
	})
	t.Run("budget", func(t *testing.T) {
		h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"))
		c, _, _ := h.connect()
		bad := [][]byte{
			[]byte("not json"),
			[]byte(`{"v":2,"type":"ping"}`),
			[]byte(`{"v":1,"type":"dance","id":"x"}`),
			[]byte(`{"v":1,"type":"turn.played","data":{"position":1}}`),
			[]byte(`{"v":1,"type":"turn.played","id":"bad id!","data":{"position":1}}`),
			[]byte(`{"v":1,"type":"turn.played","id":"x","data":{"position":1,"extra":true}}`),
			[]byte(`{"v":1,"type":"ping","extra":1}`),
		}
		for _, payload := range bad {
			c.sendRaw(payload)
			if data := c.expectError(CodeProtocolError); data.Fatal {
				t.Fatalf("%s: fatal too early", payload)
			}
		}
		if err := c.ws.WriteMessage(websocket.BinaryMessage, []byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
		if data := c.expectError(CodeProtocolError); !data.Fatal {
			t.Fatalf("8th protocol error not fatal: %+v", data)
		}
		c.expectClose(CloseProtocolViolation)
	})
	t.Run("message too big", func(t *testing.T) {
		h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"))
		c, _, _ := h.connect()
		c.sendRaw(bytes.Repeat([]byte(" "), MaxMessageBytes+1))
		c.expectClose(websocket.CloseMessageTooBig)
	})
}

func TestShutdownClosesLiveSocketsWithGoingAway(t *testing.T) {
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"))
	c, _, _ := h.connect()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.engine.Hub().Shutdown(ctx) }()
	c.expectClose(websocket.CloseGoingAway)
	if err := <-done; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if _, err := h.engine.Hub().attach(testSession); !errors.Is(err, errShuttingDown) {
		t.Fatalf("attach after shutdown = %v", err)
	}
}

func TestSynthesisIsCachedPerVoiceAndText(t *testing.T) {
	synth := tts.NewFake()
	sessions := newFakeSessions(domain.TypePractice, "Same question")
	h := newHarness(t, sessions, withSynthesizer(synth), withCache(8))
	first, _, _ := h.connect()
	_ = first.ws.NetConn().Close()
	_, _, turn := h.connect()
	if turn.AudioStatus != AudioReady || len(synth.Calls()) != 1 {
		t.Fatalf("status %s, synthesizer calls = %d", turn.AudioStatus, len(synth.Calls()))
	}
	sessions.set(func(f *fakeSessions) { f.voice = "en-US-AndrewNeural" })
	_, _, _ = h.connect()
	if len(synth.Calls()) != 2 {
		t.Fatalf("another voice reused the cache: calls = %d", len(synth.Calls()))
	}
}

func TestNewEngineNeedsEveryDependency(t *testing.T) {
	if _, err := NewEngine(nil, tts.NewFake(), NewFakeTranscriber(), NewHub(), Settings{}); err == nil {
		t.Fatal("NewEngine accepted nil sessions")
	}
}
