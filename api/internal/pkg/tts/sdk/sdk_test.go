package sdk

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

const voice = "en-US-JennyNeural"

var testCfg = Config{Key: "test-key-123", Region: "koreacentral", Timeout: 5 * time.Second}

// step scripts one speak call. lateEnd fires the end event after speak returns, as the SDK sometimes does.
type step struct {
	events  func(eventSink)
	audio   []byte
	err     error
	lateEnd bool
	noEnd   bool
	block   bool
}

type fakeEngine struct {
	mu       sync.Mutex
	steps    []step
	ssml     []string
	released int
}

func (f *fakeEngine) speak(ctx context.Context, ssml string, sink eventSink) ([]byte, func(), error) {
	f.mu.Lock()
	n := len(f.ssml)
	f.ssml = append(f.ssml, ssml)
	st := f.steps[min(n, len(f.steps)-1)]
	f.mu.Unlock()

	if st.block {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	if st.events != nil {
		st.events(sink)
	}
	if st.err != nil {
		sink.finished()
		return nil, nil, st.err
	}
	switch {
	case st.lateEnd:
		go func() {
			time.Sleep(20 * time.Millisecond)
			sink.viseme(2_000_000, 7, "")
			sink.finished()
		}()
	case !st.noEnd:
		sink.finished()
	}
	release := func() {
		f.mu.Lock()
		f.released++
		f.mu.Unlock()
	}
	return st.audio, release, nil
}

func (f *fakeEngine) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ssml)
}

func newTest(t *testing.T, e *fakeEngine, opts ...Option) *Synthesizer {
	t.Helper()
	s, err := New(testCfg, append([]Option{withEngine(e), WithBackoff(time.Millisecond)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func speakingEvents(sink eventSink) {
	sink.viseme(1_000_000, 19, "")
	sink.viseme(500_000, 0, "")
	sink.word(500_000, 212*time.Millisecond, "Hello", boundaryWord)
	sink.viseme(0, 0, `{"FrameIndex":0,"BlendShapes":[`+strings.TrimSuffix(strings.Repeat(pose+",", 3), ",")+`]}`)
}

var pose = "[" + strings.TrimSuffix(strings.Repeat("0.5,", tts.BlendShapeCount), ",") + "]"

func TestSynthesizeReturnsAudioAndTimelines(t *testing.T) {
	e := &fakeEngine{steps: []step{{events: speakingEvents, audio: []byte("mp3-bytes")}}}
	s := newTest(t, e)

	res, err := s.Synthesize(context.Background(), tts.Request{Text: "Hello <there> & you", ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Audio) != "mp3-bytes" || res.MimeType != "audio/mpeg" || res.Characters != 19 {
		t.Fatalf("result = audio %q mime %q characters %d", res.Audio, res.MimeType, res.Characters)
	}
	if len(res.Visemes) != 2 || res.Visemes[0].OffsetMs != 50 || res.Visemes[1].ID != 19 {
		t.Fatalf("visemes = %+v", res.Visemes)
	}
	if len(res.BlendShapes) != 3 || res.BlendShapes[2].OffsetMs != 33 || res.BlendShapes[0].Values[0] != 0.5 {
		t.Fatalf("blend shapes = %d frames", len(res.BlendShapes))
	}
	if len(res.Words) != 1 || res.Words[0] != (tts.WordBoundary{Text: "Hello", OffsetMs: 50, DurationMs: 212}) {
		t.Fatalf("words = %+v", res.Words)
	}
	if e.released != 1 {
		t.Fatalf("released = %d, want 1", e.released)
	}
	if c := s.Capabilities(); !c.Visemes || !c.BlendShapes || !c.WordBoundaries {
		t.Fatalf("capabilities = %+v", c)
	}
	ssml := e.ssml[0]
	for _, want := range []string{`<voice name="en-US-JennyNeural">`, `<mstts:viseme type="FacialExpression"/>`,
		`Hello &lt;there&gt; &amp; you`, `xmlns:mstts="http://www.w3.org/2001/mstts"`, `xml:lang="en-US"`} {
		if !strings.Contains(ssml, want) {
			t.Fatalf("ssml %q lacks %q", ssml, want)
		}
	}
}

func TestSynthesizeWaitsForLateEndEvent(t *testing.T) {
	e := &fakeEngine{steps: []step{{events: speakingEvents, audio: []byte("mp3"), lateEnd: true}}}
	s := newTest(t, e)

	res, err := s.Synthesize(context.Background(), tts.Request{Text: "Hello", ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Visemes) != 3 || res.Visemes[2] != (tts.Viseme{OffsetMs: 200, ID: 7}) || e.released != 1 {
		t.Fatalf("visemes = %+v released = %d", res.Visemes, e.released)
	}
}

func TestSynthesizeWithoutEndEventKeepsAudio(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	e := &fakeEngine{steps: []step{{events: speakingEvents, audio: []byte("mp3"), noEnd: true}}}
	s := newTest(t, e, withEndGrace(10*time.Millisecond), WithLogger(&logger.Logger{Logger: zap.New(core)}))

	res, err := s.Synthesize(context.Background(), tts.Request{Text: "Hello", ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Audio) != "mp3" || len(res.Visemes) != 2 || e.released != 1 {
		t.Fatalf("audio %q visemes %d released %d", res.Audio, len(res.Visemes), e.released)
	}
	if logs.FilterMessageSnippet("end event not received").Len() != 1 {
		t.Fatalf("logs = %+v", logs.All())
	}
}

func TestSynthesizeEmptyAudioIsRejected(t *testing.T) {
	e := &fakeEngine{steps: []step{{audio: nil}}}
	s := newTest(t, e)

	_, err := s.Synthesize(context.Background(), tts.Request{Text: "...", ProviderVoiceID: voice})
	if !errors.Is(err, tts.ErrRejected) || e.calls() != 1 {
		t.Fatalf("err = %v calls = %d, want ErrRejected without retry", err, e.calls())
	}
}

func TestSynthesizeErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		want       error
		wantReason string
		wantCalls  int
	}{
		{"quota", &canceledError{code: codeTooManyRequests}, tts.ErrQuota, "rate_limited", 3},
		{"connection", &canceledError{code: codeConnectionFailure}, tts.ErrUnavailable, "unreachable", 3},
		{"service timeout", &canceledError{code: codeServiceTimeout}, tts.ErrUnavailable, "timeout", 3},
		{"service error", &canceledError{code: codeServiceError}, tts.ErrUnavailable, "service_error", 3},
		{"service unavailable", &canceledError{code: codeServiceUnavailable}, tts.ErrUnavailable, "service_error", 3},
		{"bad key", &canceledError{code: codeAuthenticationFailure}, tts.ErrUnavailable, "auth_rejected", 1},
		{"forbidden", &canceledError{code: codeForbidden}, tts.ErrUnavailable, "auth_rejected", 1},
		{"unknown voice", &canceledError{code: codeBadRequest}, tts.ErrRejected, "rejected", 1},
		{"runtime", &canceledError{code: codeRuntimeError}, tts.ErrUnavailable, "sdk_error", 1},
		{"unknown code", &canceledError{code: cancelCode(42)}, tts.ErrUnavailable, "sdk_error", 1},
		{"native failure", errors.New("SPXERR_RUNTIME_ERROR"), tts.ErrUnavailable, "sdk_error", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &fakeEngine{steps: []step{{err: tc.err}}}
			s := newTest(t, e)

			_, err := s.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: voice})
			if !errors.Is(err, tc.want) || failureReason(err) != tc.wantReason {
				t.Fatalf("err = %v (reason %q), want %v (%q)", err, failureReason(err), tc.want, tc.wantReason)
			}
			if e.calls() != tc.wantCalls {
				t.Fatalf("calls = %d, want %d", e.calls(), tc.wantCalls)
			}
		})
	}
}

func TestSynthesizeRetriesQuotaThenSucceeds(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	e := &fakeEngine{steps: []step{
		{err: &canceledError{code: codeTooManyRequests}},
		{err: &canceledError{code: codeTooManyRequests}},
		{events: speakingEvents, audio: []byte("mp3")},
	}}
	s := newTest(t, e, WithLogger(&logger.Logger{Logger: zap.New(core)}))
	text := "A secret interview question"

	res, err := s.Synthesize(context.Background(), tts.Request{Text: text, ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Audio) != "mp3" || e.calls() != 3 {
		t.Fatalf("audio %q calls %d", res.Audio, e.calls())
	}
	if logs.FilterMessage("tts attempt failed").Len() != 2 || logs.FilterMessage("tts synthesized").Len() != 1 {
		t.Fatalf("logs = %+v", logs.All())
	}
	for _, entry := range logs.All() {
		for k, v := range entry.ContextMap() {
			if s, ok := v.(string); ok && (strings.Contains(s, "secret") || strings.Contains(s, testCfg.Key)) {
				t.Fatalf("log field %s leaks the text or key: %q", k, s)
			}
		}
		if strings.Contains(entry.Message, "secret") {
			t.Fatalf("log message leaks the text: %q", entry.Message)
		}
	}
	if chars := logs.FilterMessage("tts synthesized").All()[0].ContextMap()["characters"]; chars != int64(len(text)) {
		t.Fatalf("characters = %v", chars)
	}
}

func TestSynthesizeRetriesWithinTimeout(t *testing.T) {
	e := &fakeEngine{steps: []step{{err: &canceledError{code: codeServiceUnavailable}}}}
	cfg := testCfg
	cfg.Timeout = 30 * time.Millisecond
	s, err := New(cfg, withEngine(e), WithRetries(10), WithBackoff(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = s.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: voice})
	if !errors.Is(err, tts.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v, the timeout is 30ms", elapsed)
	}
	if e.calls() >= 10 {
		t.Fatalf("calls = %d, the backoff should have run out the budget first", e.calls())
	}
}

func TestSynthesizeTimeout(t *testing.T) {
	e := &fakeEngine{steps: []step{{block: true}}}
	cfg := testCfg
	cfg.Timeout = 20 * time.Millisecond
	s, err := New(cfg, withEngine(e))
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: voice})
	if !errors.Is(err, tts.ErrTimeout) || e.calls() != 1 {
		t.Fatalf("err = %v calls = %d, want one timed-out call", err, e.calls())
	}
}

func TestSynthesizeCallerCancellation(t *testing.T) {
	e := &fakeEngine{steps: []step{{block: true}}}
	s := newTest(t, e)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := s.Synthesize(ctx, tts.Request{Text: "Hi", ProviderVoiceID: voice})
	if !errors.Is(err, context.Canceled) || errors.Is(err, tts.ErrTimeout) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestSynthesizeValidatesBeforeCalling(t *testing.T) {
	e := &fakeEngine{steps: []step{{audio: []byte("mp3")}}}
	s := newTest(t, e)

	for name, tc := range map[string]struct {
		req  tts.Request
		want error
	}{
		"bad voice":     {tts.Request{Text: "Hi", ProviderVoiceID: `x"><evil`}, tts.ErrInvalidVoice},
		"empty text":    {tts.Request{Text: "  ", ProviderVoiceID: voice}, tts.ErrInvalidRequest},
		"character cap": {tts.Request{Text: strings.Repeat("a", tts.MaxTextChars+1), ProviderVoiceID: voice}, tts.ErrInvalidRequest},
	} {
		if _, err := s.Synthesize(context.Background(), tc.req); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Synthesize(ctx, tts.Request{Text: "Hi", ProviderVoiceID: voice}); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context: err = %v", err)
	}
	if e.calls() != 0 {
		t.Fatalf("engine called %d times for invalid requests", e.calls())
	}
}

func TestNewValidatesConfig(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no key":       {Region: "koreacentral", Timeout: time.Second},
		"newline key":  {Key: "k\n", Region: "koreacentral", Timeout: time.Second},
		"bad region":   {Key: "k", Region: "Korea Central", Timeout: time.Second},
		"zero timeout": {Key: "k", Region: "koreacentral"},
	} {
		if _, err := New(cfg, withEngine(&fakeEngine{})); err == nil || strings.Contains(err.Error(), "k\n") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestDelayIsCappedExponential(t *testing.T) {
	s := &Synthesizer{backoff: time.Second}
	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 4: maxBackoff} {
		if got := s.delay(attempt); got != want {
			t.Errorf("delay(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestNewWithoutTag(t *testing.T) {
	if built {
		t.Skip("built with azure_sdk")
	}
	if _, err := New(testCfg); !errors.Is(err, ErrNotBuilt) {
		t.Fatalf("err = %v, want ErrNotBuilt", err)
	}
}
