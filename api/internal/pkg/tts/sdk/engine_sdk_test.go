//go:build azure_sdk

package sdk

import (
	"testing"
	"time"

	"github.com/Microsoft/cognitive-services-speech-sdk-go/common"
)

// Needs the native libraries on the loader path; does not call the service.
func TestNewInitializesNativeSDK(t *testing.T) {
	s, err := New(Config{Key: "not-a-real-key", Region: "koreacentral", Timeout: time.Second})
	if err != nil {
		t.Fatalf("New = %v; is LD_LIBRARY_PATH set to the native Speech SDK?", err)
	}
	if _, ok := s.engine.(*nativeEngine); !ok {
		t.Fatalf("engine = %T, want the native engine", s.engine)
	}
}

func TestCancelCodeMirrorsSDK(t *testing.T) {
	for sdkCode, want := range map[common.CancellationErrorCode]cancelCode{
		common.NoError:               codeNoError,
		common.AuthenticationFailure: codeAuthenticationFailure,
		common.BadRequest:            codeBadRequest,
		common.TooManyRequests:       codeTooManyRequests,
		common.Forbidden:             codeForbidden,
		common.ConnectionFailure:     codeConnectionFailure,
		common.ServiceTimeout:        codeServiceTimeout,
		common.ServiceError:          codeServiceError,
		common.ServiceUnavailable:    codeServiceUnavailable,
		common.RuntimeError:          codeRuntimeError,
	} {
		if got := cancelCodeOf(sdkCode); got != want {
			t.Errorf("cancelCodeOf(%v) = %v, want %v", sdkCode, got, want)
		}
	}
}

func TestBoundaryKindMirrorsSDK(t *testing.T) {
	for sdkKind, want := range map[common.SpeechSynthesisBoundaryType]boundaryKind{
		common.WordBoundary:        boundaryWord,
		common.PunctuationBoundary: boundaryPunctuation,
		common.SentenceBoundary:    boundarySentence,
	} {
		if got := boundaryKindOf(sdkKind); got != want {
			t.Errorf("boundaryKindOf(%v) = %v, want %v", sdkKind, got, want)
		}
	}
}
