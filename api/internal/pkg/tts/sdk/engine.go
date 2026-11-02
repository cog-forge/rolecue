package sdk

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotBuilt is returned by New when the binary was built without the azure_sdk tag.
var ErrNotBuilt = errors.New("azure speech sdk: binary built without the azure_sdk tag")

// engine is the cgo part: it speaks one SSML document through the Speech SDK.
type engine interface {
	// speak delivers the SDK events to sink and returns the audio, a *canceledError or the context error.
	// The end event can reach sink after speak returns, so on success the caller waits for it and then
	// calls release to free the native synthesizer.
	speak(ctx context.Context, ssml string, sink eventSink) (audio []byte, release func(), err error)
}

// eventSink receives synthesizer events on the SDK's thread.
type eventSink interface {
	// viseme carries either a viseme id (animation empty) or a blend-shape batch as JSON (offset and id 0).
	viseme(offsetTicks uint64, id uint32, animation string)
	word(offsetTicks uint64, duration time.Duration, text string, kind boundaryKind)
	// finished is the completed or canceled event; nothing follows it.
	finished()
}

// boundaryKind mirrors the SDK's SpeechSynthesisBoundaryType.
type boundaryKind int

const (
	boundaryWord boundaryKind = iota
	boundaryPunctuation
	boundarySentence
)

// cancelCode mirrors the SDK's CancellationErrorCode, so error mapping needs no cgo.
type cancelCode int

const (
	codeNoError cancelCode = iota
	codeAuthenticationFailure
	codeBadRequest
	codeTooManyRequests
	codeForbidden
	codeConnectionFailure
	codeServiceTimeout
	codeServiceError
	codeServiceUnavailable
	codeRuntimeError
)

var cancelCodeNames = map[cancelCode]string{
	codeNoError:               "NoError",
	codeAuthenticationFailure: "AuthenticationFailure",
	codeBadRequest:            "BadRequest",
	codeTooManyRequests:       "TooManyRequests",
	codeForbidden:             "Forbidden",
	codeConnectionFailure:     "ConnectionFailure",
	codeServiceTimeout:        "ServiceTimeout",
	codeServiceError:          "ServiceError",
	codeServiceUnavailable:    "ServiceUnavailable",
	codeRuntimeError:          "RuntimeError",
}

func (c cancelCode) String() string {
	if name, ok := cancelCodeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("CancellationErrorCode(%d)", int(c))
}

// canceledError drops the SDK's detail text, which can quote the request.
type canceledError struct{ code cancelCode }

func (e *canceledError) Error() string { return "speech synthesis canceled: " + e.code.String() }
