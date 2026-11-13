package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/stt"
	"go.uber.org/zap"
)

type stubSTT struct{ err error }

func (s stubSTT) Transcribe(_ context.Context, a stt.Audio) (stt.Transcript, error) {
	return stt.Transcript{Text: a.MimeType, Duration: 181 * time.Second}, s.err
}
func TestSTTAdapter(t *testing.T) {
	for _, tc := range []struct{ source, target error }{{nil, nil}, {stt.ErrNoSpeech, runtime.ErrNoSpeech}, {stt.ErrUnsupportedAudio, runtime.ErrUnsupportedAudio}, {stt.ErrUnavailable, stt.ErrUnavailable}, {context.DeadlineExceeded, context.DeadlineExceeded}} {
		tr, err := (sttAdapter{provider: stubSTT{tc.source}}).Transcribe(context.Background(), runtime.Audio{MimeType: "audio/webm"})
		if !errors.Is(err, tc.target) {
			t.Error("wrong runtime error")
		}
		if tr.Duration != 181*time.Second || tr.Text != "audio/webm" {
			t.Error("adapter lost transcript fields")
		}
	}
}
func TestProvideAzureSTT(t *testing.T) {
	cfg := &config.Config{Interview: config.InterviewConfig{STTProvider: "azure", STTTimeout: time.Second}}
	log := &logger.Logger{Logger: zap.NewNop()}
	if _, err := ProvideTranscriber(cfg, log); err == nil {
		t.Fatal("missing credentials accepted")
	}
	cfg.TTS.Key = "test"
	cfg.TTS.Region = "koreacentral"
	if _, err := ProvideTranscriber(cfg, log); err != nil {
		t.Fatal(err)
	}
}
