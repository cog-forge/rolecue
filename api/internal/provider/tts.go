package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts/azure"
)

// ProvideSynthesizer wires Azure AI Speech when its credentials are configured. Without them, or with an
// unusable setting, it wires a synthesizer that reports tts.ErrUnavailable so the interview shows the
// question as text; the API itself always starts.
func ProvideSynthesizer(cfg *config.Config, log *logger.Logger) tts.Synthesizer {
	if cfg.TTS.Key == "" && cfg.TTS.Region == "" {
		log.Warn("Text-to-speech disabled: AZURE_SPEECH_KEY and AZURE_SPEECH_REGION are not set")
		return tts.NewUnavailable()
	}
	client, err := azure.New(azure.Config{
		Key:     cfg.TTS.Key,
		Region:  cfg.TTS.Region,
		Timeout: cfg.TTS.Timeout,
	}, azure.WithLogger(log))
	if err != nil {
		// The error names the faulty setting, never its value.
		log.Error("Text-to-speech disabled: invalid Azure Speech configuration", logger.Error(err))
		return tts.NewUnavailable()
	}
	log.Info("Text-to-speech enabled", logger.String("provider", "azure"), logger.String("region", cfg.TTS.Region))
	return client
}
