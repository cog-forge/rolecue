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

// mp3Header reads the sample rate and bitrate of the first MPEG-1 Layer III frame, after an optional ID3v2 tag.
func mp3Header(t *testing.T, audio []byte) (hz, kbps int) {
	t.Helper()
	i := 0
	if len(audio) > 10 && string(audio[:3]) == "ID3" {
		i = 10 + int(audio[6]&0x7f)<<21 + int(audio[7]&0x7f)<<14 + int(audio[8]&0x7f)<<7 + int(audio[9]&0x7f)
	}
	if len(audio) < i+4 || audio[i] != 0xFF || audio[i+1]&0xFE != 0xFA {
		t.Fatalf("audio does not start with an MPEG-1 Layer III frame: % x", audio[:min(len(audio), 8)])
	}
	rates := [...]int{44100, 48000, 32000}
	bitrates := [...]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	rateIndex := int(audio[i+2]>>2) & 3
	if rateIndex == 3 {
		t.Fatalf("reserved sample rate index in frame header % x", audio[i:i+4])
	}
	return rates[rateIndex], bitrates[audio[i+2]>>4]
}

// Jenny is recorded at 48 kHz, Kai at 24 kHz (the list reports SampleRateHertz); both must come back as 48 kHz.
func TestLiveSynthesize(t *testing.T) {
	for _, voice := range []string{"en-US-JennyNeural", "en-US-KaiNeural"} {
		t.Run(voice, func(t *testing.T) { liveSynthesize(t, voice) })
	}
}

func liveSynthesize(t *testing.T, voice string) {
	c := liveClient(t)
	text := "Tell me about a project you are proud of."

	res, err := c.Synthesize(context.Background(), tts.Request{Text: text, ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	if res.MimeType != "audio/mpeg" || res.Characters != len(text) {
		t.Fatalf("mime=%q characters=%d", res.MimeType, res.Characters)
	}
	// A few seconds of 192 kbps MP3 is around a hundred kilobytes.
	if len(res.Audio) < 10_000 {
		t.Fatalf("audio is only %d bytes", len(res.Audio))
	}
	rate, kbps := mp3Header(t, res.Audio)
	if rate != 48000 {
		t.Fatalf("sample rate = %d Hz, want 48000", rate)
	}
	t.Logf("%d bytes, MPEG-1 Layer III, %d Hz, %d kbps: %.1f s of audio, %d bytes per second",
		len(res.Audio), rate, kbps, float64(len(res.Audio))*8/float64(kbps*1000), kbps*1000/8)
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
