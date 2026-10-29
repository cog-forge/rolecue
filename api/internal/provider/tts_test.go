package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
)

func TestProvideSynthesizer(t *testing.T) {
	log := &logger.Logger{Logger: zap.NewNop()}
	req := tts.Request{Text: "Hi", ProviderVoiceID: "en-US-AvaNeural"}

	for name, cfg := range map[string]config.TTSConfig{
		"no credentials":     {Timeout: time.Second},
		"key without region": {Key: "k", Timeout: time.Second},
		"bad region":         {Key: "k", Region: "x/y", Timeout: time.Second},
		"zero timeout":       {Key: "k", Region: "koreacentral"},
	} {
		t.Run(name, func(t *testing.T) {
			s := ProvideSynthesizer(&config.Config{TTS: cfg}, log)
			if _, err := s.Synthesize(context.Background(), req); !errors.Is(err, tts.ErrUnavailable) {
				t.Fatalf("err = %v, want ErrUnavailable", err)
			}
		})
	}

	t.Run("configured", func(t *testing.T) {
		s := ProvideSynthesizer(&config.Config{TTS: config.TTSConfig{Key: "k", Region: "koreacentral", Timeout: time.Second}}, log)
		if _, ok := s.(*azure.Client); !ok {
			t.Fatalf("synthesizer = %T, want *azure.Client", s)
		}
	})
}
