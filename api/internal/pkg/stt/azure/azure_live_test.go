//go:build azure_stt_live

package azure

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/pkg/stt"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	azuretts "github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
)

// TestLiveGeneratedSpeech sends synthetic speech only. Credentials, audio, and text stay out of output.
func TestLiveGeneratedSpeech(t *testing.T) {
	key, region := os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION")
	if key == "" || region == "" {
		t.Skip("Azure Speech credentials are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	synth, err := azuretts.New(azuretts.Config{Key: key, Region: region, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal("TTS configuration failed")
	}
	clip, err := synth.Synthesize(ctx, tts.Request{Text: "I build reliable software with automated tests.", ProviderVoiceID: "en-US-JennyNeural"})
	if err != nil {
		t.Fatal("Synthetic clip generation failed")
	}
	c, err := New(Config{Key: key, Region: region, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal("STT configuration failed")
	}
	// The existing REST TTS returns MP3; use the same fast API, which also accepts MP3.
	result, err := c.Transcribe(ctx, stt.Audio{Data: clip.Audio, MimeType: clip.MimeType})
	if err != nil {
		t.Fatalf("STT failed: %v", err)
	}
	if result.Duration <= 0 || !strings.Contains(strings.ToLower(result.Text), "reliable software") {
		t.Fatal("Generated speech was not recognized as expected")
	}
}
