package provider

import (
	"fmt"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

// The live engine and handler drive the interview service. These assertions make the build fail if a
// signature drifts from the interfaces they declare.
var (
	_ runtime.Sessions               = (*service.Service)(nil)
	_ handler.InterviewSessionReader = (*service.Service)(nil)
	_ handler.InterviewLiveEngine    = (*runtime.Engine)(nil)
)

// ProvideTranscriber selects the speech-to-text adapter of the live interview. Only the deterministic
// fake exists until KAN-55; a real adapter adds its case here, selected by INTERVIEW_STT_PROVIDER.
func ProvideTranscriber(cfg *config.Config, log *logger.Logger) (runtime.Transcriber, error) {
	switch cfg.Interview.STTProvider {
	case "", config.STTProviderFake:
		log.Warn("Live interview speech-to-text uses the development fake: every spoken answer is stored as a fixed sentence")
		return runtime.NewFakeTranscriber(), nil
	}
	return nil, fmt.Errorf("unknown interview.stt_provider %q", cfg.Interview.STTProvider)
}

// ProvideInterviewLiveEngine wires the live interview engine over the interview service, the shared
// synthesizer and the transcriber.
func ProvideInterviewLiveEngine(svc *service.Service, synthesizer tts.Synthesizer, transcriber runtime.Transcriber, cfg *config.Config) (*runtime.Engine, error) {
	return runtime.NewEngine(svc, synthesizer, transcriber, runtime.NewHub(), runtime.Settings{
		STTTimeout:   cfg.Interview.STTTimeout,
		CacheEntries: cfg.Interview.TTSCacheEntries,
	})
}

// ProvideInterviewLiveHandler trusts the same browser origins as CORS.
func ProvideInterviewLiveHandler(svc *service.Service, engine *runtime.Engine, cfg *config.Config) *handler.InterviewLiveHandler {
	return handler.NewInterviewLiveHandler(svc, engine, cfg.CORS.AllowOrigins)
}
