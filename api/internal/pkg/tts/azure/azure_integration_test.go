//go:build tts_integration

package azure_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
)

// Runs against the real Azure AI Speech service and spends a few dozen characters of the monthly quota.
// It skips unless AZURE_SPEECH_KEY and AZURE_SPEECH_REGION are set. Audio stays in memory, never on disk.
func liveClient(t *testing.T) *azure.Client {
	t.Helper()
	key, region := os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION")
	if key == "" || region == "" {
		t.Skip("AZURE_SPEECH_KEY and AZURE_SPEECH_REGION are not set")
	}
	c, err := azure.New(azure.Config{Key: key, Region: region, Timeout: 20 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLiveSynthesize(t *testing.T) {
	c := liveClient(t)
	text := "Tell me about a project you are proud of."

	res, err := c.Synthesize(context.Background(), tts.Request{Text: text, ProviderVoiceID: "en-US-JennyNeural"})
	if err != nil {
		t.Fatal(err)
	}
	if res.MimeType != "audio/mpeg" || res.Characters != len(text) {
		t.Fatalf("mime=%q characters=%d", res.MimeType, res.Characters)
	}
	// A few seconds of 48 kbps MP3 is tens of kilobytes; an ID3 tag or an MPEG frame sync starts the file.
	if len(res.Audio) < 10_000 {
		t.Fatalf("audio is only %d bytes", len(res.Audio))
	}
	if head := res.Audio[:3]; string(head) != "ID3" && !(head[0] == 0xFF && head[1]&0xE0 == 0xE0) {
		t.Fatalf("audio does not start like an MP3: % x", head)
	}
	if len(res.Visemes) != 0 {
		t.Fatalf("REST adapter returned visemes: %v", res.Visemes)
	}
}

func TestLiveUnknownVoiceIsRejected(t *testing.T) {
	c := liveClient(t)
	_, err := c.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: "en-US-NoSuchVoiceNeural"})
	if !errors.Is(err, tts.ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected", err)
	}
}

func TestLiveBadKeyIsUnavailable(t *testing.T) {
	region := os.Getenv("AZURE_SPEECH_REGION")
	if region == "" {
		t.Skip("AZURE_SPEECH_REGION is not set")
	}
	c, err := azure.New(azure.Config{Key: strings.Repeat("0", 32), Region: region, Timeout: 20 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: "en-US-JennyNeural"})
	if !errors.Is(err, tts.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}
