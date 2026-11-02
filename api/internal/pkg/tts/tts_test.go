package tts_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

const voice = "en-US-AvaMultilingualNeural"

func TestRequestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  tts.Request
		want error
	}{
		{"valid", tts.Request{Text: "Tell me about yourself.", ProviderVoiceID: voice}, nil},
		{"hd voice with colon", tts.Request{Text: "Hi", ProviderVoiceID: "en-US-Ava:DragonHDLatestNeural"}, nil},
		{"empty voice", tts.Request{Text: "Hi"}, tts.ErrInvalidVoice},
		{"voice with quote", tts.Request{Text: "Hi", ProviderVoiceID: `a"b`}, tts.ErrInvalidVoice},
		{"voice with space", tts.Request{Text: "Hi", ProviderVoiceID: "en US"}, tts.ErrInvalidVoice},
		{"empty text", tts.Request{ProviderVoiceID: voice}, tts.ErrInvalidRequest},
		{"blank text", tts.Request{Text: " \n\t", ProviderVoiceID: voice}, tts.ErrInvalidRequest},
		{"at the limit", tts.Request{Text: strings.Repeat("a", tts.MaxTextChars), ProviderVoiceID: voice}, nil},
		{"over the limit", tts.Request{Text: strings.Repeat("a", tts.MaxTextChars+1), ProviderVoiceID: voice}, tts.ErrInvalidRequest},
		{"limit counts characters not bytes", tts.Request{Text: strings.Repeat("é", tts.MaxTextChars), ProviderVoiceID: voice}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.want == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFakeIsDeterministicPlayableAudio(t *testing.T) {
	f := tts.NewFake()
	req := tts.Request{Text: "Describe a hard bug you fixed.", ProviderVoiceID: voice}

	a, err := f.Synthesize(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := f.Synthesize(context.Background(), req)
	if !bytes.Equal(a.Audio, b.Audio) {
		t.Fatal("same request produced different audio")
	}
	if a.MimeType != "audio/wav" || a.Characters != len(req.Text) {
		t.Fatalf("mime=%q characters=%d", a.MimeType, a.Characters)
	}
	if string(a.Audio[:4]) != "RIFF" || string(a.Audio[8:12]) != "WAVE" {
		t.Fatal("audio is not a WAV file")
	}
	if got := int(binary.LittleEndian.Uint32(a.Audio[40:])); got != len(a.Audio)-44 {
		t.Fatalf("data chunk length %d, want %d", got, len(a.Audio)-44)
	}
	if len(a.Visemes) != 0 || f.Capabilities().Visemes {
		t.Fatal("default fake must mirror an audio-only provider")
	}

	long, _ := f.Synthesize(context.Background(), tts.Request{Text: strings.Repeat("word ", 40), ProviderVoiceID: voice})
	if len(long.Audio) <= len(a.Audio) {
		t.Fatal("longer text should give longer audio")
	}
}

func TestFakeVisemes(t *testing.T) {
	f := tts.NewFake(tts.WithFakeVisemes())
	res, err := f.Synthesize(context.Background(), tts.Request{Text: "Hello there", ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	if !f.Capabilities().Visemes || len(res.Visemes) < 2 {
		t.Fatalf("visemes = %v", res.Visemes)
	}
	for i := 1; i < len(res.Visemes); i++ {
		if res.Visemes[i].OffsetMs <= res.Visemes[i-1].OffsetMs {
			t.Fatalf("offsets not increasing at %d: %v", i, res.Visemes)
		}
	}
	if last := res.Visemes[len(res.Visemes)-1]; last.ID != 0 {
		t.Fatalf("last viseme = %+v, want silence", last)
	}
}

func TestFakeBlendShapesAndWords(t *testing.T) {
	plain := tts.NewFake()
	if c := plain.Capabilities(); c.BlendShapes || c.WordBoundaries {
		t.Fatalf("plain fake capabilities = %+v", c)
	}
	res, _ := plain.Synthesize(context.Background(), tts.Request{Text: "Hello there", ProviderVoiceID: voice})
	if res.BlendShapes != nil || res.Words != nil {
		t.Fatalf("plain fake returned timelines: %d frames, %d words", len(res.BlendShapes), len(res.Words))
	}

	f := tts.NewFake(tts.WithFakeBlendShapes(), tts.WithFakeWordBoundaries())
	if c := f.Capabilities(); !c.BlendShapes || !c.WordBoundaries || c.Visemes {
		t.Fatalf("capabilities = %+v", c)
	}
	res, err := f.Synthesize(context.Background(), tts.Request{Text: "Tell me  about it", ProviderVoiceID: voice})
	if err != nil {
		t.Fatal(err)
	}
	// 17 characters at 60 ms each is 1020 ms of audio: 61 frames at 60 FPS.
	if len(res.BlendShapes) != 61 {
		t.Fatalf("frames = %d, want 61", len(res.BlendShapes))
	}
	for i, fr := range res.BlendShapes {
		if len(fr.Values) != tts.BlendShapeCount {
			t.Fatalf("frame %d has %d values", i, len(fr.Values))
		}
		if i > 0 && fr.OffsetMs <= res.BlendShapes[i-1].OffsetMs {
			t.Fatalf("frame offsets not increasing at %d", i)
		}
	}
	want := []tts.WordBoundary{
		{Text: "Tell", OffsetMs: 0, DurationMs: 255},
		{Text: "me", OffsetMs: 255, DurationMs: 255},
		{Text: "about", OffsetMs: 510, DurationMs: 255},
		{Text: "it", OffsetMs: 765, DurationMs: 255},
	}
	if !reflect.DeepEqual(res.Words, want) {
		t.Fatalf("words = %+v", res.Words)
	}
}

func TestFakeValidatesRecordsAndFails(t *testing.T) {
	boom := errors.New("boom")
	f := tts.NewFake(tts.WithFakeError(boom))

	if _, err := f.Synthesize(context.Background(), tts.Request{ProviderVoiceID: voice}); !errors.Is(err, tts.ErrInvalidRequest) {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := f.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: voice}); !errors.Is(err, boom) {
		t.Fatalf("injected error: %v", err)
	}
	if calls := f.Calls(); len(calls) != 1 || calls[0].Text != "Hi" {
		t.Fatalf("calls = %+v, want only the valid request", calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tts.NewFake().Synthesize(ctx, tts.Request{Text: "Hi", ProviderVoiceID: voice}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
}

func TestUnavailable(t *testing.T) {
	u := tts.NewUnavailable()
	if _, err := u.Synthesize(context.Background(), tts.Request{Text: "Hi", ProviderVoiceID: voice}); !errors.Is(err, tts.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if _, err := u.Synthesize(context.Background(), tts.Request{ProviderVoiceID: voice}); !errors.Is(err, tts.ErrInvalidRequest) {
		t.Fatalf("invalid request should be reported as such: %v", err)
	}
	if u.Capabilities().Visemes {
		t.Fatal("unavailable synthesizer reports visemes")
	}
}
