//go:build azure_sdk

package sdk

import (
	"context"
	"errors"
	"fmt"

	"github.com/Microsoft/cognitive-services-speech-sdk-go/common"
	"github.com/Microsoft/cognitive-services-speech-sdk-go/speech"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

const built = true

// nativeEngine opens a synthesizer per call: the SDK routes events per synthesizer, so a shared one would
// mix concurrent calls.
type nativeEngine struct {
	key    string
	region string
	log    *logger.Logger
}

// newEngine opens and closes one synthesizer, without contacting the service, so a broken native install
// fails at startup.
func newEngine(cfg Config, log *logger.Logger) (engine, error) {
	e := &nativeEngine{key: cfg.Key, region: cfg.Region, log: log}
	syn, conf, err := e.open()
	if err != nil {
		return nil, fmt.Errorf("initialize azure speech sdk: %w", err)
	}
	syn.Close()
	conf.Close()
	return e, nil
}

func (e *nativeEngine) open() (*speech.SpeechSynthesizer, *speech.SpeechConfig, error) {
	conf, err := speech.NewSpeechConfigFromSubscription(e.key, e.region)
	if err != nil {
		return nil, nil, err
	}
	if err := conf.SetSpeechSynthesisOutputFormat(common.Audio48Khz192KBitRateMonoMp3); err != nil {
		conf.Close()
		return nil, nil, err
	}
	// A nil audio config keeps the audio in memory instead of opening a sound device.
	syn, err := speech.NewSpeechSynthesizerFromConfig(conf, nil)
	if err != nil {
		conf.Close()
		return nil, nil, err
	}
	return syn, conf, nil
}

func (e *nativeEngine) speak(ctx context.Context, ssml string, sink eventSink) ([]byte, func(), error) {
	syn, conf, err := e.open()
	if err != nil {
		return nil, nil, err
	}
	release := func() {
		syn.Close()
		conf.Close()
	}

	syn.VisemeReceived(func(ev speech.SpeechSynthesisVisemeEventArgs) {
		defer ev.Close()
		sink.viseme(ev.AudioOffset, uint32(ev.VisemeID), ev.Animation)
	})
	syn.WordBoundary(func(ev speech.SpeechSynthesisWordBoundaryEventArgs) {
		defer ev.Close()
		sink.word(ev.AudioOffset, ev.Duration, ev.Text, boundaryKindOf(ev.BoundaryType))
	})
	syn.SynthesisCompleted(func(ev speech.SpeechSynthesisEventArgs) {
		ev.Close()
		sink.finished()
	})
	syn.SynthesisCanceled(func(ev speech.SpeechSynthesisEventArgs) {
		ev.Close()
		sink.finished()
	})

	outcomes := syn.SpeakSsmlAsync(ssml)
	select {
	case out := <-outcomes:
		audio, err := readOutcome(out)
		if err != nil {
			release()
			return nil, nil, err
		}
		return audio, release, nil
	case <-ctx.Done():
		// The SDK call cannot be abandoned: stop it and free the synthesizer once it returns.
		go func() {
			if err := <-syn.StopSpeakingAsync(); err != nil && e.log != nil {
				e.log.Warn("tts could not stop an abandoned synthesis", logger.String("engine", "sdk"), logger.Cause(err))
			}
			out := <-outcomes
			out.Close()
			release()
		}()
		return nil, nil, ctx.Err()
	}
}

func readOutcome(out speech.SpeechSynthesisOutcome) ([]byte, error) {
	defer out.Close()
	if out.Error != nil {
		return nil, out.Error
	}
	res := out.Result
	if res == nil {
		return nil, errors.New("speech sdk returned no result")
	}
	switch res.Reason {
	case common.SynthesizingAudioCompleted:
		return res.AudioData, nil
	case common.Canceled:
		details, err := speech.NewCancellationDetailsFromSpeechSynthesisResult(res)
		if err != nil {
			return nil, err
		}
		return nil, &canceledError{code: cancelCodeOf(details.ErrorCode)}
	default:
		return nil, fmt.Errorf("speech sdk result reason %d", int(res.Reason))
	}
}

func cancelCodeOf(code common.CancellationErrorCode) cancelCode {
	switch code {
	case common.NoError:
		return codeNoError
	case common.AuthenticationFailure:
		return codeAuthenticationFailure
	case common.BadRequest:
		return codeBadRequest
	case common.TooManyRequests:
		return codeTooManyRequests
	case common.Forbidden:
		return codeForbidden
	case common.ConnectionFailure:
		return codeConnectionFailure
	case common.ServiceTimeout:
		return codeServiceTimeout
	case common.ServiceError:
		return codeServiceError
	case common.ServiceUnavailable:
		return codeServiceUnavailable
	default:
		return codeRuntimeError
	}
}

func boundaryKindOf(t common.SpeechSynthesisBoundaryType) boundaryKind {
	switch t {
	case common.WordBoundary:
		return boundaryWord
	case common.PunctuationBoundary:
		return boundaryPunctuation
	default:
		return boundarySentence
	}
}
