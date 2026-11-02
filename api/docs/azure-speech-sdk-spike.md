# Azure Speech SDK spike (KAN-248)

Run on 2026-10-30. **Result: the pinned Go Speech SDK returns viseme ids, `FacialExpression` blend shapes
and word boundaries for `en-US-JennyNeural` and `en-US-GuyNeural`, together with 48 kHz mono MP3 audio.**
It works natively on CachyOS (not an officially supported distribution) and in Debian 12 and Ubuntu 24.04
containers. The adapter is `internal/pkg/tts/sdk`; this page is the install recipe and the event behaviour
it relies on.

## Versions

| Part | Value |
| --- | --- |
| Go module | `github.com/Microsoft/cognitive-services-speech-sdk-go v1.53.0` (capital `M`: the lowercase path fails with "module declares its path as ...") |
| Native SDK | `SpeechSDK-Linux-1.53.0.tar.gz`, 8,099,137 bytes |
| Download URL | `https://csspeechstorage.blob.core.windows.net/drop/1.53.0/SpeechSDK-Linux-1.53.0.tar.gz` |
| SHA-256 | `9f88b77c003c7dabe08623e7c4b41ce3f3bad109c79bcfea62f5d4a32861bc6f` |
| Build tag | `azure_sdk` |

`https://aka.ms/csspeech/linuxbinary` redirected to the same file on the spike date, but it follows the
latest release. Always download the versioned URL and check the hash, so the native library matches the Go
module.

## Install recipe (Linux x64)

The SDK goes outside the repository. Nothing here is committed.

```sh
SPEECHSDK_ROOT="$HOME/.cache/kan248-dev/speechsdk"   # any directory outside the repo
mkdir -p "$SPEECHSDK_ROOT"
curl -fsSL -o /tmp/SpeechSDK-Linux-1.53.0.tar.gz \
  https://csspeechstorage.blob.core.windows.net/drop/1.53.0/SpeechSDK-Linux-1.53.0.tar.gz
echo "9f88b77c003c7dabe08623e7c4b41ce3f3bad109c79bcfea62f5d4a32861bc6f  /tmp/SpeechSDK-Linux-1.53.0.tar.gz" \
  | sha256sum -c -
tar xzf /tmp/SpeechSDK-Linux-1.53.0.tar.gz -C "$SPEECHSDK_ROOT" --strip-components=1
```

The archive holds `include/c_api` (headers) and `lib/{x64,arm64,arm32}` (shared libraries).

Build and run with the tag, as the SDK README describes:

```sh
export CGO_ENABLED=1
export CGO_CFLAGS="-I$SPEECHSDK_ROOT/include/c_api"
export CGO_LDFLAGS="-L$SPEECHSDK_ROOT/lib/x64 -lMicrosoft.CognitiveServices.Speech.core"
export LD_LIBRARY_PATH="$SPEECHSDK_ROOT/lib/x64${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

cd api
set -a; . ../.env; set +a
go run -tags azure_sdk ./cmd/api
go test -tags azure_sdk ./internal/pkg/tts/...
go test -tags "azure_sdk tts_integration" -run TestLive -v ./internal/pkg/tts/sdk/   # real Azure calls
```

Keep these variables out of `.env`: `make run` exports that file, and the default build must stay free of
cgo.

### Packages

| Stage | Needs |
| --- | --- |
| Build | a C compiler (`gcc`); `golang:1.27-bookworm` already has it |
| Run | `libssl3` (Ubuntu 24.04: `libssl3t64`) and `ca-certificates`. glibc 2.29 or newer |

`libasound2` and GStreamer are **not** needed: they back the microphone, speaker and compressed-input
extensions, and the adapter synthesizes to memory. Verified with `ldd` and a real call on a minimal
`ubuntu:24.04` with only `ca-certificates libssl3t64`.

Of the eight libraries in `lib/x64` (7.2 MB), synthesis needs two:
`libMicrosoft.CognitiveServices.Speech.core.so` and `libpal_azure_c_shared_openssl3.so`. Copying the whole
`lib/x64` directory is simplest.

### Tested hosts

| Host | glibc | OpenSSL | Result |
| --- | --- | --- | --- |
| CachyOS, kernel 7.2.11, GCC 16.2.1 (native) | 2.44 | 3.6.5 | pass |
| `golang:1.27-bookworm` build, `debian:12-slim` run | 2.36 | 3 | pass |
| `golang:1.27-bookworm` build, `ubuntu:24.04` run | 2.39 | 3 | pass |

The container rows built the spike program as below (shown here for the API) and ran it with the listed
packages only:

```sh
# build
docker run --rm -v "$SPEECHSDK_ROOT":/opt/speechsdk:ro -v "$PWD":/src -w /src golang:1.27-bookworm sh -c '
  CGO_ENABLED=1 CGO_CFLAGS=-I/opt/speechsdk/include/c_api \
  CGO_LDFLAGS="-L/opt/speechsdk/lib/x64 -lMicrosoft.CognitiveServices.Speech.core" \
  go build -tags azure_sdk -o /src/out/app ./cmd/api'
# run
apt-get install -y --no-install-recommends ca-certificates libssl3t64   # libssl3 on Debian 12
LD_LIBRARY_PATH=/opt/speechsdk/lib/x64 /app/app
```

## What the SDK returns

SSML used (the adapter builds the same document, with the text escaped):

```xml
<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis"
       xmlns:mstts="http://www.w3.org/2001/mstts" xml:lang="en-US">
  <voice name="en-US-JennyNeural"><mstts:viseme type="FacialExpression"/>TEXT</voice>
</speak>
```

Output format `Audio48Khz192KBitRateMonoMp3` (`audio-48khz-192kbitrate-mono-mp3`, the same format the REST
adapter uses). Audio starts with an MPEG frame sync (`ff fb`). The full sentence "Tell me about a project you
are proud of, and what you would do differently today." (82 characters) gave:

| | Jenny | Guy |
| --- | --- | --- |
| Audio | 111,168 B, 4.587 s | 114,048 B, 4.725 s |
| Wall time, request to result (new connection) | 1.8 s | 1.2 s |
| `VisemeReceived` with a viseme id | 58 | 58 |
| `VisemeReceived` with animation JSON | 6 batches, 273 frames (4.55 s) | 6 batches, 283 frames (4.72 s) |
| `WordBoundary` | 19 (16 words, 2 punctuation, 1 sentence) | 19 |

### `VisemeReceived`

With `FacialExpression`, one event carries **either** a viseme or a blend-shape batch:

- **Viseme:** `VisemeID` 0-21 and `AudioOffset` in 100 ns ticks (`ms = ticks / 10_000`, e.g. `9812500` is
  981.25 ms), `Animation` empty. The first is id 0 (silence) at 50 ms, the last id 0 when speech ends.
  They arrive in offset order, in bursts, before the blend shapes.
- **Blend-shape batch:** `AudioOffset` 0 and `VisemeID` 0 (do not treat it as a silence viseme) with
  `Animation` set to `{"FrameIndex":N,"BlendShapes":[[55 floats], ...]}`. Batches are consecutive: frame
  indexes were 0, 62, 136, 211, 220, 261 (each starts where the previous ended). Frame `i` is at
  `i * 1000 / 60` ms from the start of the audio. Every frame had 55 values in 0-1. The values follow
  Azure's documented ARKit order: indexes 0 and 7 blinked (eyeBlinkLeft/Right) and 17 (jawOpen) moved
  most during speech.

The adapter merges the batches by `FrameIndex`, keeps the first pose for a repeated index, drops frames
that do not have 55 values, and logs how many it dropped.

### `WordBoundary`

`BoundaryType` 0 is a word, 1 punctuation, 2 a sentence. `AudioOffset` is in ticks, `Duration` already a
Go `time.Duration`. Words arrive in order, but the sentence boundary arrived after the last-but-one word and
the final "." after the blend shapes, so arrival order is not offset order. The adapter keeps words only
and sorts by offset.

### End and empty events

- `SynthesisCompleted` (or `SynthesisCanceled`) comes last; every viseme, batch and word arrives before it.
  Callbacks run synchronously on the SDK's thread, in order.
- **The end event can arrive after `SpeakSsmlAsync` returns its result** (seen in the "Hi." run: 27
  events had arrived with the result, 28 a moment later). The adapter waits up to one second for it before reading the timelines,
  and only then closes the native synthesizer, which unregisters the callbacks.
- Text with nothing to speak ("...") completes with 0 bytes of audio and no events except start and end.
  The adapter reports it as `tts.ErrRejected`.
- "Hi." gave 4 visemes, 3 batches (79 frames) and 1 word.

### Errors

| Case | Result | Adapter error |
| --- | --- | --- |
| Unknown voice | `Canceled`, `BadRequest` ("Unsupported voice ...") | `ErrRejected`, no retry |
| Wrong key | `Canceled`, `AuthenticationFailure` (WebSocket 401) | `ErrUnavailable`, no retry |
| Only the core library on the loader path | `Canceled`, `ConnectionFailure` ("failed to establish USP connection") | `ErrUnavailable` after retries |
| Core library missing | the process does not start: `error while loading shared libraries` | none: the dynamic linker fails before `main` |
| Rate limit (not reproduced) | `TooManyRequests` per the SDK | `ErrQuota`, retried with backoff |

The cancellation detail text is not copied into errors or logs because it can quote the request.

## Behaviour notes for KAN-252

- The tagged binary links `libMicrosoft.CognitiveServices.Speech.core.so` at load time. If it cannot be
  found, the API **does not start**. Either set
  `LD_LIBRARY_PATH`, or install the libraries in a directory the loader already searches (for example
  `/usr/local/lib` plus `ldconfig`), or add `-Wl,-rpath,<dir>` to `CGO_LDFLAGS`.
- The SDK loads `libpal_azure_c_shared_openssl3.so` only when it connects. Without it, startup succeeds and
  every SDK call fails with `ConnectionFailure`, so ship the whole `lib/x64` directory.
- There is no fallback to REST in a tagged binary. If the SDK cannot initialize, the API logs
  `Text-to-speech disabled` with `engine=sdk` and interviews show questions as text.
- A `nil` audio config keeps the audio in memory and never opens a sound device, so no ALSA device is needed
  in a container.
- The production image is Wolfi (glibc); it needs OpenSSL 3 and CA certificates for the tagged build.

## SDK notes

- Each call creates its own `SpeechConfig` and `SpeechSynthesizer`. The Go SDK routes callbacks per
  synthesizer handle, so a shared synthesizer would mix events between concurrent calls.
- Event args hold native handles; every handler calls `Close()`.
- The Go SDK has no context support. On timeout the adapter calls `StopSpeakingAsync` and frees the
  synthesizer in the background once the pending call returns.
- In v1.53.0 the viseme bridge releases the synthesizer handle instead of the event handle when it cannot
  read an event (`synthesizerFireEventVisemeReceived`). That path was not hit in the spike.
