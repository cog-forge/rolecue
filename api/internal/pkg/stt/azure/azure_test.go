package azure

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/stt"
)

func TestTranscribe(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
		reason string
	}{
		{"success", 200, `{"durationMilliseconds":180001,"combinedPhrases":[{"text":"A synthetic answer."}]}`, nil, ""},
		{"silence", 200, `{"durationMilliseconds":10000,"combinedPhrases":[]}`, stt.ErrNoSpeech, "no_speech"},
		{"whitespace", 200, `{"durationMilliseconds":10,"combinedPhrases":[{"text":"  "}]}`, stt.ErrNoSpeech, "no_speech"},
		{"unsupported", 400, `private provider body`, stt.ErrUnsupportedAudio, "unsupported_audio"},
		{"media", 415, `private provider body`, stt.ErrUnsupportedAudio, "unsupported_audio"},
		{"auth", 401, `private provider body`, stt.ErrUnavailable, "auth_rejected"},
		{"forbidden", 403, `private provider body`, stt.ErrUnavailable, "auth_rejected"},
		{"quota", 429, `private provider body`, stt.ErrUnavailable, "rate_limited"},
		{"server", 503, `private provider body`, stt.ErrUnavailable, "http_5xx"},
		{"malformed", 200, `invalid private response`, stt.ErrUnavailable, "invalid_response"},
		{"missing duration", 200, `{"combinedPhrases":[{"text":"A synthetic answer."}]}`, stt.ErrUnavailable, "invalid_response"},
		{"negative duration", 200, `{"durationMilliseconds":-1}`, stt.ErrUnavailable, "invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" {
					t.Error("incorrect request")
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
				}
				if r.FormValue("definition") != `{"locales":["en-US"]}` {
					t.Error("incorrect locale")
				}
				f, h, err := r.FormFile("audio")
				if err != nil {
					t.Error(err)
				} else {
					defer func() { _ = f.Close() }()
					if h.Header.Get("Content-Type") != "audio/webm" {
						t.Error("incorrect MIME")
					}
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c, err := New(Config{Key: "test-key", Region: "koreacentral", Timeout: time.Second}, WithEndpoint(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			result, err := c.Transcribe(context.Background(), stt.Audio{Data: []byte("synthetic audio"), MimeType: "audio/webm"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("unexpected error class: %v", err)
			}
			if tc.reason != "" && failureReason(err) != tc.reason {
				t.Error("wrong reason")
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Error("provider data leaked")
			}
			if tc.want == nil && (result.Text != "A synthetic answer." || result.Duration != 180001*time.Millisecond) {
				t.Error("incorrect transcript or duration mapping")
			}
			if calls.Load() != 1 {
				t.Error("unexpected retry")
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(60 * time.Millisecond) }))
	defer server.Close()
	c, err := New(Config{Key: "test", Region: "koreacentral", Timeout: 20 * time.Millisecond}, WithEndpoint(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Transcribe(context.Background(), stt.Audio{Data: []byte("audio"), MimeType: "audio/wav"})
	if !errors.Is(err, context.DeadlineExceeded) || failureReason(err) != "timeout" {
		t.Fatalf("unexpected timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Transcribe(ctx, stt.Audio{Data: []byte("audio"), MimeType: "audio/wav"})
	if !errors.Is(err, context.Canceled) {
		t.Error("cancellation ignored")
	}
}

func TestConfig(t *testing.T) {
	for _, cfg := range []Config{{}, {Key: "test", Region: "invalid/region", Timeout: time.Second}, {Key: "test\n", Region: "koreacentral", Timeout: time.Second}, {Key: "test", Region: "koreacentral"}} {
		if _, err := New(cfg); err == nil {
			t.Error("invalid config accepted")
		}
	}
}

func TestParentDeadlineAndInput(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); time.Sleep(60 * time.Millisecond) }))
	defer server.Close()
	c, err := New(Config{Key: "test", Region: "koreacentral", Timeout: time.Second}, WithEndpoint(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = c.Transcribe(ctx, stt.Audio{Data: []byte("audio"), MimeType: "audio/webm"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("parent deadline ignored")
	}
	_, err = c.Transcribe(context.Background(), stt.Audio{})
	if !errors.Is(err, stt.ErrNoSpeech) {
		t.Error("empty audio not classified")
	}
	_, err = c.Transcribe(context.Background(), stt.Audio{Data: []byte("audio"), MimeType: "application/octet-stream"})
	if !errors.Is(err, stt.ErrUnsupportedAudio) {
		t.Error("unsupported MIME not rejected")
	}
	if calls.Load() != 1 {
		t.Error("invalid input reached provider")
	}
}

func TestLoggingDoesNotExposeEvidence(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	for _, status := range []int{200, 401} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"durationMilliseconds":10,"combinedPhrases":[{"text":"private-evidence"}]}`))
		}))
		c, err := New(Config{Key: "private-key", Region: "koreacentral", Timeout: time.Second}, WithEndpoint(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = c.Transcribe(ctx, stt.Audio{Data: []byte("private-audio"), MimeType: "audio/webm"})
		server.Close()
	}
	for _, entry := range logs.All() {
		encoded, err := json.Marshal(entry.ContextMap())
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"private-evidence", "private-key", "private-audio"} {
			if strings.Contains(string(encoded), secret) {
				t.Error("sensitive data leaked in logs")
			}
		}
		fields := entry.ContextMap()
		if fields["provider"] != "azure" || fields["audio_bytes"] == nil || fields["audio_duration"] == nil || fields["reason"] == nil {
			t.Error("required diagnostic fields missing")
		}
	}
	if logs.Len() != 2 {
		t.Error("missing operation logs")
	}
}
