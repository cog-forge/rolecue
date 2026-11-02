//go:build azure_sdk && tts_integration

package sdk_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/sdk"
)

// Calls the real service (about 150 characters); skips without credentials.
func liveConfig(t *testing.T) sdk.Config {
	t.Helper()
	key, region := os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION")
	if key == "" || region == "" {
		t.Skip("AZURE_SPEECH_KEY and AZURE_SPEECH_REGION are not set")
	}
	return sdk.Config{Key: key, Region: region, Timeout: 30 * time.Second}
}

func TestLiveSynthesizeTimelines(t *testing.T) {
	s, err := sdk.New(liveConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	text := "Tell me about a project you are proud of."

	for _, voice := range []string{"en-US-JennyNeural", "en-US-GuyNeural"} {
		t.Run(voice, func(t *testing.T) {
			res, err := s.Synthesize(context.Background(), tts.Request{Text: text, ProviderVoiceID: voice})
			if err != nil {
				t.Fatal(err)
			}
			if res.MimeType != "audio/mpeg" || res.Characters != len(text) || len(res.Audio) < 10_000 {
				t.Fatalf("mime=%q characters=%d audio=%d bytes", res.MimeType, res.Characters, len(res.Audio))
			}
			if head := res.Audio[:3]; string(head) != "ID3" && (head[0] != 0xFF || head[1]&0xE0 != 0xE0) {
				t.Fatalf("audio does not start like an MP3: % x", head)
			}
			// 192 kbps = 24 KB/s.
			audioMs := len(res.Audio) * 1000 / 24_000

			if len(res.Visemes) < 10 {
				t.Fatalf("visemes = %d", len(res.Visemes))
			}
			for i, v := range res.Visemes {
				if v.ID < 0 || v.ID > 21 || (i > 0 && v.OffsetMs < res.Visemes[i-1].OffsetMs) {
					t.Fatalf("viseme %d = %+v after %+v", i, v, res.Visemes[max(i-1, 0)])
				}
			}

			frames := res.BlendShapes
			if len(frames) < 60 {
				t.Fatalf("blend-shape frames = %d", len(frames))
			}
			for i, f := range frames {
				if len(f.Values) != tts.BlendShapeCount || (i > 0 && f.OffsetMs <= frames[i-1].OffsetMs) {
					t.Fatalf("frame %d: offset %d, %d values", i, f.OffsetMs, len(f.Values))
				}
			}
			if last := frames[len(frames)-1].OffsetMs; last < audioMs/2 || last > audioMs+500 {
				t.Fatalf("last frame at %d ms, audio is about %d ms", last, audioMs)
			}

			var words []string
			for i, w := range res.Words {
				if w.DurationMs <= 0 || (i > 0 && w.OffsetMs < res.Words[i-1].OffsetMs) {
					t.Fatalf("word %d = %+v", i, w)
				}
				words = append(words, w.Text)
			}
			if got := strings.Join(words, " "); got != "Tell me about a project you are proud of" {
				t.Fatalf("words = %q", got)
			}
			t.Logf("audio=%d bytes (~%d ms) visemes=%d frames=%d words=%d", len(res.Audio), audioMs,
				len(res.Visemes), len(frames), len(res.Words))
		})
	}
}

func TestLiveTimeoutAbandonsSynthesis(t *testing.T) {
	cfg := liveConfig(t)
	cfg.Timeout = 50 * time.Millisecond
	s, err := sdk.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = s.Synthesize(context.Background(), tts.Request{Text: "Walk me through your last release.", ProviderVoiceID: "en-US-GuyNeural"})
	if !errors.Is(err, tts.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("returned after %v, the timeout is 50ms", elapsed)
	}
	// Let the abandoned call be freed in the background, then check the SDK still works.
	time.Sleep(3 * time.Second)
	cfg.Timeout = 30 * time.Second
	s, err = sdk.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Synthesize(context.Background(), tts.Request{Text: "Thanks.", ProviderVoiceID: "en-US-GuyNeural"}); err != nil {
		t.Fatal(err)
	}
}

func TestLiveUnknownVoiceIsRejected(t *testing.T) {
	s, err := sdk.New(liveConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: "en-US-NoSuchVoiceNeural"})
	if !errors.Is(err, tts.ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected", err)
	}
}

func TestLiveWrongKeyIsUnavailable(t *testing.T) {
	cfg := liveConfig(t)
	cfg.Key = strings.Repeat("0", 32)
	s, err := sdk.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: "en-US-JennyNeural"})
	if !errors.Is(err, tts.ErrUnavailable) || !strings.Contains(err.Error(), "credentials rejected") {
		t.Fatalf("err = %v, want rejected credentials", err)
	}
}
