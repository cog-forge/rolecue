package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	authservice "github.com/cog-forge/rolecue/api/internal/features/auth/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/stt"
	azurestt "github.com/cog-forge/rolecue/api/internal/pkg/stt/azure"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

// The live engine and handler drive the interview service. These assertions make the build fail if a
// signature drifts from the interfaces they declare.
var (
	_ runtime.Sessions               = (*service.Service)(nil)
	_ handler.InterviewSessionReader = (*service.Service)(nil)
	_ handler.InterviewLiveEngine    = (*runtime.Engine)(nil)
	_ service.EndNotifier            = (*runtime.Hub)(nil)
)

// ProvideTranscriber selects the live interview speech-to-text adapter. Azure credentials are shared
// with TTS; an invalid Azure configuration fails startup instead of silently selecting the fake.
func ProvideTranscriber(cfg *config.Config, log *logger.Logger) (runtime.Transcriber, error) {
	switch cfg.Interview.STTProvider {
	case config.STTProviderAzure:
		timeout := cfg.Interview.STTTimeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		client, err := azurestt.New(azurestt.Config{Key: cfg.TTS.Key, Region: cfg.TTS.Region, Timeout: timeout})
		if err != nil {
			return nil, err
		}
		return sttAdapter{provider: client}, nil
	case "", config.STTProviderFake:
		log.Warn("Live interview speech-to-text uses the development fake: every spoken answer is stored as a fixed sentence")
		return runtime.NewFakeTranscriber(), nil
	}
	return nil, fmt.Errorf("unknown interview.stt_provider %q", cfg.Interview.STTProvider)
}

// ProvideInterviewLiveEngine wires the live interview engine over the interview service, the shared
// synthesizer and the transcriber. Its hub hears every session the service ends, so an open socket
// reports a leave or an abandonment at once.
func ProvideInterviewLiveEngine(svc *service.Service, synthesizer tts.Synthesizer, transcriber runtime.Transcriber, cfg *config.Config) (*runtime.Engine, error) {
	hub := runtime.NewHub()
	engine, err := runtime.NewEngine(svc, synthesizer, transcriber, hub, runtime.Settings{
		STTTimeout:         cfg.Interview.STTTimeout,
		CacheEntries:       cfg.Interview.TTSCacheEntries,
		RevalidateInterval: cfg.Interview.LiveRecheckInterval,
		MessageRate:        cfg.Interview.LiveMessageRate,
		MessageBurst:       cfg.Interview.LiveMessageBurst,
	})
	if err != nil {
		return nil, err
	}
	svc.SetEndNotifier(hub)
	return engine, nil
}

// ProvideInterviewLiveHandler trusts the same browser origins as CORS and re-checks each open socket's
// login through the auth service.
func ProvideInterviewLiveHandler(svc *service.Service, engine *runtime.Engine, cfg *config.Config, auth authservice.AuthService) *handler.InterviewLiveHandler {
	return handler.NewInterviewLiveHandler(svc, engine, cfg.CORS.AllowOrigins).WithAuthenticator(auth)
}

// sttAdapter keeps feature errors at the composition boundary; pkg/stt never imports a feature.
type sttAdapter struct{ provider stt.Transcriber }

func (a sttAdapter) Transcribe(ctx context.Context, audio runtime.Audio) (runtime.Transcript, error) {
	result, err := a.provider.Transcribe(ctx, stt.Audio{Data: audio.Data, MimeType: audio.MimeType})
	switch {
	case errors.Is(err, stt.ErrNoSpeech):
		err = runtime.ErrNoSpeech
	case errors.Is(err, stt.ErrUnsupportedAudio):
		err = runtime.ErrUnsupportedAudio
	}
	return runtime.Transcript{Text: result.Text, Duration: result.Duration}, err
}
