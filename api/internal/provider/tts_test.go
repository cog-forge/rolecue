package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/sdk"
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
		switch s.(type) {
		case *azure.Client, *sdk.Synthesizer:
		default:
			t.Fatalf("synthesizer = %T, want an Azure synthesizer", s)
		}
	})
}

func TestProvideSynthesizerEngine(t *testing.T) {
	cfg := &config.Config{TTS: config.TTSConfig{Key: "k", Region: "koreacentral", Timeout: time.Second}}
	sdkSynth := tts.NewFake(tts.WithFakeVisemes())

	cases := map[string]struct {
		err        error
		wantEngine string
		check      func(tts.Synthesizer) bool
	}{
		"sdk": {
			wantEngine: "sdk",
			check:      func(s tts.Synthesizer) bool { return s == tts.Synthesizer(sdkSynth) },
		},
		"built without the tag": {
			err:        sdk.ErrNotBuilt,
			wantEngine: "rest",
			check:      func(s tts.Synthesizer) bool { _, ok := s.(*azure.Client); return ok },
		},
		"sdk initialization fails": {
			err:        errors.New("initialize azure speech sdk: SPXERR_RUNTIME_ERROR"),
			wantEngine: "sdk",
			check:      func(s tts.Synthesizer) bool { _, ok := s.(tts.Unavailable); return ok },
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			var got sdk.Config
			factory := func(c sdk.Config, _ *logger.Logger) (tts.Synthesizer, error) {
				got = c
				if tc.err != nil {
					return nil, tc.err
				}
				return sdkSynth, nil
			}

			s := provideSynthesizer(cfg, &logger.Logger{Logger: zap.New(core)}, factory)

			if got != (sdk.Config{Key: "k", Region: "koreacentral", Timeout: time.Second}) {
				t.Fatalf("sdk config = %+v", got)
			}
			if !tc.check(s) {
				t.Fatalf("synthesizer = %T", s)
			}
			entries := logs.All()
			if len(entries) != 1 || entries[0].ContextMap()["engine"] != tc.wantEngine {
				t.Fatalf("logs = %+v, want one entry with engine %q", entries, tc.wantEngine)
			}
		})
	}
}
