# Azure interview speech-to-text

Select `INTERVIEW_STT_PROVIDER=azure` and provide the existing `AZURE_SPEECH_KEY`
and `AZURE_SPEECH_REGION` used by TTS. The default remains `fake`. Missing or invalid
credentials fail startup when Azure STT is selected. `INTERVIEW_STT_TIMEOUT` defaults
to 30 seconds and bounds the whole request (the caller's earlier deadline wins).

The provider uses synchronous fast transcription API `2024-11-15`, with the fixed
locale `en-US`. Browser WebM/Opus is uploaded directly; no ffmpeg or Speech SDK is
required in the deployment image. Recordings up to the engine's three-minute limit
are handled in one call. The response's `durationMilliseconds` maps to
`Transcript.Duration`, preserving the engine's duration enforcement.

There is no automatic retry: keep live-turn latency within the timeout budget and
let the Candidate answer again after `STT_FAILED`. Silence maps to `STT_NO_SPEECH`;
HTTP 400/415 maps to unsupported audio. Authentication, quota, service and malformed
response errors remain generic failures. Provider response bodies and transport
error details are never returned or logged. Operation logs contain only provider,
byte count, decoded duration, elapsed time, and a classified failure reason.

## Verification

Credential-free tests: `cd api && go test ./internal/pkg/stt/... ./internal/provider ./internal/config`.
Live synthetic speech (skips without credentials):
`go test -tags=azure_stt_live -count=1 ./internal/pkg/stt/azure`.
Load the root environment before running live tests. Never use real candidate answers
as test fixtures or print transcripts/audio/credentials.

On 2026-11-03 the configured Korea Central resource accepted WebM/Opus directly:
10 seconds of generated speech took 1.240 seconds; 60 seconds took 1.231 seconds.
Both returned HTTP 200 and exact recording duration, with recognized speech.
No quota errors occurred; the resource pricing tier was not determined by this probe.

Official references:
- https://learn.microsoft.com/azure/ai-services/speech-service/fast-transcription-create
- https://learn.microsoft.com/rest/api/speechtotext/transcriptions/transcribe?view=rest-speechtotext-2024-11-15

## Coordination

Azure STT is independent of KAN-55 (local Whisper). A later Whisper provider can use
the same provider-neutral boundary or implement `runtime.Transcriber` directly.
Sync the selectable Azure provider, timeout, no-retry policy and direct WebM support
into Brain Integrations §3.2 and the provider/configuration notes in
`docs/interview-ws-protocol.md` §9–10. No WebSocket messages changed.

The isolated stack (Caddy 3310, auth 3311, Next 3312, API 8210, database clone
55420) completed a three-question Practice session using Chromium's WAV-backed
fake microphone and actual MediaRecorder WebM/Opus uploads. All three room
transcripts matched `conversation_turns.answer` exactly. Evaluation completed with
scores, summary and feedback on all three turns; report answers matched the stored
voice answers. A separate silent microphone recording returned `STT_NO_SPEECH`,
showed the retry copy, and accepted no answer. No frontend changes were required.

Final checks passed: API format/vet/tests/lint/build (50 tested packages, lint zero
issues); integration against the clone (61 tested packages); focused STT/config/
composition race tests; live Azure generated-speech test; frontend lint, format,
types, build and 1,563 tests across 195 files. The room/live/i18n Playwright specs
passed all 19 tests. Their existing CORS fixtures hardcode port 3100; temporary
copies changed that origin to this lane's permitted port 3312. Tracked frontend
and test files were unchanged. Test processes and the lane's Caddy were stopped.
