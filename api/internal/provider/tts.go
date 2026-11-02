package provider

import (
	"errors"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/sdk"
)

// ProvideSynthesizer wires Azure AI Speech when its credentials are configured: the Speech SDK in a binary
// built with the azure_sdk tag, the REST endpoint (audio only) otherwise. Without credentials, or when the
// synthesizer cannot be created, it wires one that reports tts.ErrUnavailable so the interview shows the
// question as text; the API itself always starts.
func ProvideSynthesizer(cfg *config.Config, log *logger.Logger) tts.Synthesizer {
	return provideSynthesizer(cfg, log, newSDKSynthesizer)
}

type sdkFactory func(sdk.Config, *logger.Logger) (tts.Synthesizer, error)

func newSDKSynthesizer(cfg sdk.Config, log *logger.Logger) (tts.Synthesizer, error) {
	s, err := sdk.New(cfg, sdk.WithLogger(log))
	if err != nil {
		return nil, err
	}
	return s, nil
}

func provideSynthesizer(cfg *config.Config, log *logger.Logger, newSDK sdkFactory) tts.Synthesizer {
	if cfg.TTS.Key == "" && cfg.TTS.Region == "" {
		log.Warn("Text-to-speech disabled: AZURE_SPEECH_KEY and AZURE_SPEECH_REGION are not set")
		return tts.NewUnavailable()
	}

	engine := "sdk"
	s, err := newSDK(sdk.Config{Key: cfg.TTS.Key, Region: cfg.TTS.Region, Timeout: cfg.TTS.Timeout}, log)
	if errors.Is(err, sdk.ErrNotBuilt) {
		engine = "rest"
		s, err = newRESTSynthesizer(cfg.TTS, log)
	}
	if err != nil {
		// The error names the faulty setting, never its value.
		log.Error("Text-to-speech disabled", logger.String("engine", engine), logger.Cause(err))
		return tts.NewUnavailable()
	}
	log.Info("Text-to-speech enabled", logger.String("provider", "azure"), logger.String("engine", engine),
		logger.String("region", cfg.TTS.Region))
	return s
}

func newRESTSynthesizer(cfg config.TTSConfig, log *logger.Logger) (tts.Synthesizer, error) {
	c, err := azure.New(azure.Config{Key: cfg.Key, Region: cfg.Region, Timeout: cfg.Timeout}, azure.WithLogger(log))
	if err != nil {
		return nil, err
	}
	return c, nil
}
