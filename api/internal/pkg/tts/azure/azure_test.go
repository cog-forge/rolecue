package azure_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
)

const (
	testKey = "test-key-DO-NOT-LOG"
	voice   = "en-US-AvaMultilingualNeural"
)

var audioBytes = []byte("ID3-fake-mp3-bytes")

// stub serves handler and returns a Client pointed at it with instant retries.
func stub(t *testing.T, handler http.HandlerFunc, opts ...azure.Option) (*azure.Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	opts = append([]azure.Option{azure.WithEndpoint(srv.URL), azure.WithBackoff(time.Millisecond)}, opts...)
	c, err := azure.New(azure.Config{Key: testKey, Region: "koreacentral", Timeout: 2 * time.Second}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "audio/mpeg")
	_, _ = w.Write(audioBytes)
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func req(text string) tts.Request { return tts.Request{Text: text, ProviderVoiceID: voice} }

func TestSynthesizeSuccess(t *testing.T) {
	var gotHeaders http.Header
	var gotBody string
	c, calls := stub(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		ok(w, r)
	})

	res, err := c.Synthesize(context.Background(), req("Tell me about yourself."))
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Audio) != string(audioBytes) || res.MimeType != "audio/mpeg" {
		t.Fatalf("audio=%q mime=%q", res.Audio, res.MimeType)
	}
	if res.Characters != len("Tell me about yourself.") {
		t.Fatalf("characters = %d", res.Characters)
	}
	if len(res.Visemes) != 0 || c.Capabilities().Visemes {
		t.Fatal("REST adapter must report audio only")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if gotHeaders.Get("Ocp-Apim-Subscription-Key") != testKey ||
		gotHeaders.Get("Content-Type") != "application/ssml+xml" ||
		gotHeaders.Get("X-Microsoft-OutputFormat") != "audio-24khz-48kbitrate-mono-mp3" ||
		gotHeaders.Get("User-Agent") == "" {
		t.Fatalf("headers = %v", gotHeaders)
	}
	want := `<speak version="1.0" xml:lang="en-US"><voice name="` + voice + `">Tell me about yourself.</voice></speak>`
	if gotBody != want {
		t.Fatalf("ssml = %s\nwant  %s", gotBody, want)
	}
}

func TestSynthesizeEscapesSSML(t *testing.T) {
	var gotBody string
	c, _ := stub(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		ok(w, r)
	})

	text := `</voice><voice name="evil">R&D <b>"quoted"</b> 'single'`
	if _, err := c.Synthesize(context.Background(), req(text)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`<voice name="evil">`, `<b>`, `R&D`} {
		if strings.Contains(gotBody, bad) {
			t.Fatalf("unescaped %q in %s", bad, gotBody)
		}
	}
	if !strings.Contains(gotBody, "R&amp;D &lt;b&gt;") || strings.Count(gotBody, "<voice") != 1 {
		t.Fatalf("ssml = %s", gotBody)
	}
}

func TestSynthesizeValidatesWithoutCallingProvider(t *testing.T) {
	c, calls := stub(t, ok)

	cases := []struct {
		name string
		req  tts.Request
		want error
	}{
		{"empty text", req(""), tts.ErrInvalidRequest},
		{"long text", req(strings.Repeat("a", tts.MaxTextChars+1)), tts.ErrInvalidRequest},
		{"bad voice", tts.Request{Text: "Hi", ProviderVoiceID: `x"><y`}, tts.ErrInvalidVoice},
	}
	for _, tc := range cases {
		if _, err := c.Synthesize(context.Background(), tc.req); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("provider was called %d times for invalid requests", calls.Load())
	}
}

func TestSynthesizeAcceptsTextAtTheLimit(t *testing.T) {
	c, _ := stub(t, ok)
	res, err := c.Synthesize(context.Background(), req(strings.Repeat("a", tts.MaxTextChars)))
	if err != nil || res.Characters != tts.MaxTextChars {
		t.Fatalf("res.Characters=%d err=%v", res.Characters, err)
	}
}

func TestStatusMappingAndRetryPolicy(t *testing.T) {
	cases := []struct {
		name      string
		code      int
		want      error
		wantCalls int32 // with the default 2 retries
	}{
		{"400 is rejected, not retried", 400, tts.ErrRejected, 1},
		{"415 is rejected, not retried", 415, tts.ErrRejected, 1},
		{"401 is unavailable, not retried", 401, tts.ErrUnavailable, 1},
		{"403 is unavailable, not retried", 403, tts.ErrUnavailable, 1},
		{"429 is quota, retried", 429, tts.ErrQuota, 3},
		{"500 is unavailable, retried", 500, tts.ErrUnavailable, 3},
		{"502 is unavailable, retried", 502, tts.ErrUnavailable, 3},
		{"503 is unavailable, retried", 503, tts.ErrUnavailable, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, calls := stub(t, status(tc.code))
			_, err := c.Synthesize(context.Background(), req("Hi"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if calls.Load() != tc.wantCalls {
				t.Fatalf("calls = %d, want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

func TestRetryRecoversFromTransientFailure(t *testing.T) {
	var n atomic.Int32
	c, calls := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ok(w, r)
	})

	res, err := c.Synthesize(context.Background(), req("Hi"))
	if err != nil || string(res.Audio) != string(audioBytes) {
		t.Fatalf("res=%q err=%v", res.Audio, err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3", calls.Load())
	}
}

func TestRetriesAreBounded(t *testing.T) {
	c, calls := stub(t, status(503), azure.WithRetries(1))
	if _, err := c.Synthesize(context.Background(), req("Hi")); !errors.Is(err, tts.ErrUnavailable) {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}

	c, calls = stub(t, status(503), azure.WithRetries(0))
	_, _ = c.Synthesize(context.Background(), req("Hi"))
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestRetryAfterIsHonouredAndCapped(t *testing.T) {
	var n atomic.Int32
	c, _ := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		ok(w, r)
	})

	start := time.Now()
	if _, err := c.Synthesize(context.Background(), req("Hi")); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("retried after %v, want to wait for Retry-After: 1", elapsed)
	}
}

func TestTimeout(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	// Cleanups run last-in first-out: release the handler before Close waits for it.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	c, err := azure.New(azure.Config{Key: testKey, Region: "koreacentral", Timeout: 100 * time.Millisecond},
		azure.WithEndpoint(srv.URL), azure.WithBackoff(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = c.Synthesize(context.Background(), req("Hi"))
	if !errors.Is(err, tts.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %v, the budget is 100ms", time.Since(start))
	}
	if calls.Load() != 1 {
		t.Fatalf("a timed-out budget must not be retried, calls = %d", calls.Load())
	}
}

func TestCallerCancellationIsNotATimeout(t *testing.T) {
	c, _ := stub(t, func(http.ResponseWriter, *http.Request) { time.Sleep(200 * time.Millisecond) })
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)

	_, err := c.Synthesize(ctx, req("Hi"))
	if !errors.Is(err, context.Canceled) || errors.Is(err, tts.ErrTimeout) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestEmptyAudioIsUnavailable(t *testing.T) {
	c, calls := stub(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	if _, err := c.Synthesize(context.Background(), req("Hi")); !errors.Is(err, tts.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestNetworkFailureIsRetriedThenUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more
	c, err := azure.New(azure.Config{Key: testKey, Region: "koreacentral", Timeout: 2 * time.Second},
		azure.WithEndpoint(url), azure.WithBackoff(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Synthesize(context.Background(), req("Hi")); !errors.Is(err, tts.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		ok(w, r)
	}))
	t.Cleanup(target.Close)
	c, _ := stub(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) })

	if _, err := c.Synthesize(context.Background(), req("Hi")); !errors.Is(err, tts.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if targetHits.Load() != 0 {
		t.Fatal("redirect was followed; the key would have been forwarded")
	}
}

func TestLogsCharacterCountButNeverTextOrKey(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	log := &logger.Logger{Logger: zap.New(core)}

	const secretText = "Confidential salary band for the Staff Engineer role"
	var n atomic.Int32
	c, _ := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ok(w, r)
	}, azure.WithLogger(log))

	if _, err := c.Synthesize(context.Background(), req(secretText)); err != nil {
		t.Fatal(err)
	}

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("log entries = %d, want a warning for the failed attempt and one info", len(entries))
	}
	info := entries[1]
	if info.Message != "tts synthesized" || info.ContextMap()["characters"] != int64(len(secretText)) {
		t.Fatalf("info entry = %+v", info)
	}
	for _, e := range entries {
		dump := fmt.Sprint(e.Message, e.ContextMap())
		if strings.Contains(dump, secretText) || strings.Contains(dump, testKey) {
			t.Fatalf("log leaks text or key: %s", dump)
		}
	}
}

func TestNewValidatesConfig(t *testing.T) {
	good := azure.Config{Key: testKey, Region: "koreacentral", Timeout: time.Second}
	if _, err := azure.New(good); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*azure.Config){
		"empty key":        func(c *azure.Config) { c.Key = "  " },
		"newline in key":   func(c *azure.Config) { c.Key = "abc\nInjected: 1" },
		"empty region":     func(c *azure.Config) { c.Region = "" },
		"region with path": func(c *azure.Config) { c.Region = "evil.example.com/x" },
		"region uppercase": func(c *azure.Config) { c.Region = "KoreaCentral" },
		"zero timeout":     func(c *azure.Config) { c.Timeout = 0 },
	} {
		cfg := good
		mutate(&cfg)
		if _, err := azure.New(cfg); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
