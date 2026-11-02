# Live Interview WebSocket Protocol (v1)

This is the authoritative contract between the live interview room (KAN-249) and the interview engine
(KAN-52). The server is authoritative: it decides which question is asked, when the session listens,
when an answer counts and when the interview ends. The client renders what the server says and reports
two facts only: that the interviewer turn finished playing, and the Candidate's answer.

Code: `api/internal/features/interview/runtime` (engine, hub, STT boundary) and
`api/internal/handler/interview_live_handler.go` (upgrade). `docs/IMPLEMENTATION_PLAN.md` §2 is stale and
superseded by this document.

## 1. Connection

```
GET {NEXT_PUBLIC_API_BASE_URL}/interviews/{id}/live
Sec-WebSocket-Protocol: rolecue.interview.v1
```

Browser: `new WebSocket(url.replace(/^http/, "ws"), ["rolecue.interview.v1"])`. The session cookie is
sent automatically; there is no token in the URL.

The session must already exist: it is started (and charged, or its recruitment slot consumed) only by
`POST /interviews`. The socket never charges, never consumes a slot and never creates a session, so
connecting, reconnecting and resuming are always free.

Checks before the upgrade, answered as plain HTTP (the browser only sees a failed connection, close code
1006 without an `open` event; call `GET /interviews/{id}` to learn why):

| Status | Cause |
|---|---|
| 400 | The id is not a UUID. |
| 401 | No valid Better Auth session (the same `RequireAuth` middleware as every API route). |
| 403 | `Origin` missing or not in `CORS_ALLOW_ORIGINS` (cross-site WebSocket hijacking guard), onboarding not finished, or the user is not a Candidate. |
| 404 | The session does not exist or belongs to another Candidate. |

After the upgrade the server checks the subprotocol. A client that did not offer
`rolecue.interview.v1` is closed with `4000`.

**One socket per session.** A newer connection to the same session takes over: the older socket is
closed with `4001` (for example a second browser tab). Single API instance only; scaling out needs a
shared hub (KAN-67).

## 2. Envelopes

All frames are UTF-8 JSON text frames. Binary frames are a protocol error.

Server to client:

```json
{ "v": 1, "stream": "8d0b8e2c-…", "seq": 12, "type": "interviewer.turn", "reply_to": "c-17", "data": { } }
```

- `v`: protocol version, always `1`.
- `stream`, `seq`: ordering. `seq` starts at 1 and grows by exactly one for each message the server
  sends for the session. It continues across reconnects for as long as the API process keeps the
  session's stream (30 minutes after the last socket closes). A new `stream` value (API restart, or a
  stream that expired) restarts `seq` at 1.
  - Ignore a message whose `seq` is not greater than the last one seen on the same `stream`.
  - A gap across a reconnect is normal: messages written to a dead socket are lost. The `session.sync`
    that opens every connection is the full state, so nothing has to be replayed.
- `reply_to`: the `id` of the client message this one answers, when there is one.

Client to server:

```json
{ "v": 1, "id": "c-17", "type": "answer.audio", "data": { } }
```

- `v` must be `1`. Unknown fields, unknown types and malformed JSON are protocol errors.
- `id`: client message id, required on `turn.played`, `answer.text` and `answer.audio`; optional on
  `ping`. 1 to 64 characters of `A-Z a-z 0-9 _ -`. Use a fresh id per command (a UUID is fine).

### Idempotency

Two independent guards; neither can advance the session twice.

1. **Message id.** The server remembers the ids of the last 256 commands it accepted for the session
   (across reconnects, for the life of the stream). Repeating one gets `ack` with `duplicate: true` and
   has no effect. A command that failed is not remembered, so it may be retried with the same id.
2. **Position (durable).** Every answer names the `position` it answers. An answer for a position that is
   already stored is acknowledged with `answer.accepted` and `duplicate: true`, without running STT or
   writing anything. This holds after an API restart too, because it reads the stored turns.
   Repository-level: one stored turn per `(interview_id, position)` (unique constraint).

## 3. States

The server announces each state change with a `state` message, before any content for the new state.
`session.sync` carries the state a connection starts in.

| Server state | Meaning | Frontend machine (`interview-machine.ts`) |
|---|---|---|
| (socket opening) | Handshake in progress. | `CONNECTING` |
| `interviewer_speaking` | A question is out; the client plays it (or shows it as text). | `ASSET_LOADING` on first sync, then `INTERVIEWER_SPEAKING` |
| `listening` | The server accepts an answer for the current position. | `LISTENING` |
| `processing` | The answer arrived; STT and persistence are running. | `PROCESSING_ANSWER` |
| `transitioning` | The answer is stored; the next turn is being prepared, or the session is completing. | `TRANSITIONING` |
| `completed` | The session is completed and evaluation has started. | `COMPLETED` |
| `paused` | Reserved for KAN-215 (pause and exit). Not sent in v1. | `PAUSED` |
| (socket lost) | Abnormal close or ping timeout. | `RECONNECTING` |
| (close 4001 / 4003 / 4000) | Not recoverable by reconnecting. | `TERMINATED_EARLY` / `FATAL_ERROR` |

```
connect ─► session.sync(interviewer_speaking) ─► interviewer.turn
        ─► client: turn.played ─► state(listening)
        ─► client: answer.audio … final ─► state(processing) ─► answer.accepted
        ─► state(transitioning) ─► state(interviewer_speaking) ─► interviewer.turn   (next position)
        … last answer ─► state(transitioning) ─► state(completed) ─► session.completed ─► close 1000
```

## 4. Server messages

### `session.sync`
First message of every connection.

```json
{
  "session_id": "uuid", "session_type": "practice",
  "state": "interviewer_speaking",
  "question_count": 8, "answered_count": 3, "position": 4,
  "resumed": true, "evaluation_status": null,
  "server_time": "2026-10-30T10:00:00Z"
}
```

- `position`: the current question, or `null` when none is open (`transitioning`, `completed`).
- `resumed`: an earlier connection of this session existed on the same stream.
- `evaluation_status`: only set when `state` is `completed`.

### `state`
`{ "state": "listening", "position": 4 }`

### `interviewer.turn`
Sent after `state(interviewer_speaking)`. On every reconnect the current turn is sent again with the
same text, because the text is frozen when it is first delivered.

```json
{
  "position": 4, "kind": "core", "question_count": 8,
  "text": "Walk me through how you would design a rate limiter.",
  "audio": { "mime_type": "audio/mpeg", "data": "<base64>" },
  "audio_status": "ready",
  "visemes": [ { "offset_ms": 0, "id": 0 }, { "offset_ms": 85, "id": 12 } ],
  "blend_shapes": [],
  "words": [],
  "cues": []
}
```

- `kind`: `core` or `follow_up`. Only `core` exists until KAN-56.
- `audio`: `null` when `audio_status` is not `ready`.
- `audio_status`:
  - `ready`: play `audio`.
  - `unavailable`: TTS failed or timed out. Show `text`; the interview continues (Integrations §3.3).
  - `no_voice`: the session has no provider voice and no default is configured. Show `text`.
- `visemes`: Azure viseme ids with their offsets from the audio start. Empty when the synthesizer gives
  none (the REST adapter); the client then lip-syncs from the audio.
- `blend_shapes`, `words`: always present, empty until KAN-248 merges. The engine then fills them from
  `tts.Result`:
  - `blend_shapes`: `[{ "offset_ms": n, "values": [55 numbers] }]` at 60 FPS;
  - `words`: `[{ "offset_ms": n, "duration_ms": n, "text": "…" }]`.
- `cues`: always present, empty until KAN-250 (emotion and gesture cues).
- Never sent: the question bank, `core_question_id`, other positions' questions, evaluation data, and
  (for recruitment) any transcript.

**Why audio is inline base64, not a URL:** the turn arrives atomically, so the text, the audio and the
viseme timeline can never disagree or arrive out of order. There is no second authenticated endpoint or
presigned object to expire during a reconnect, and no link to a recruitment interviewer's audio that
could be shared. The size is small: interviewer audio is 48 kHz mono MP3, about 180 KB for a 15-second
question (about 240 KB as base64). Audio over 3 MiB is not sent (`audio_status: unavailable`). Revisit
with short-lived URLs only if turns become long or audio must be cached by the browser.

### `answer.accepted`
`{ "position": 4, "answered_count": 4, "duplicate": false, "transcript": "…" }`

- `transcript` is present only for **practice** sessions. A recruitment transcript is never sent to the
  Candidate (Interview §5.6).
- `duplicate: true` means the position was already stored; nothing changed.

### `session.completed`
`{ "evaluation_status": "pending" }`, then the server closes with `1000`.

### `ack`
`{ "duplicate": true }` with `reply_to`: a repeated command id; nothing changed.

### `error`
`{ "code": "STT_NO_SPEECH", "message": "…", "retryable": true, "fatal": false }`

`fatal: true` is followed by a close. The `message` is for logs; show localized copy keyed by `code`.

| Code | Retryable | Fatal | When |
|---|---|---|---|
| `PROTOCOL_ERROR` | no | after 8 | Malformed JSON, unknown type or field, bad `v` or `id`, binary frame. The 8th closes with `4000`. |
| `INVALID_STATE` | no | no | A command not allowed now, such as an answer while `interviewer_speaking`. |
| `INTERVIEW_TURN_CONFLICT` | no | no | An answer for a later position than the current one. Followed by a fresh `session.sync`. |
| `AUDIO_FORMAT_UNSUPPORTED` | yes | no | Unknown `mime_type`, or bytes that do not match it. |
| `AUDIO_TOO_LARGE` | yes | no | The answer exceeds the audio limits (§6). |
| `AUDIO_CHUNK_ORDER` | yes | no | A chunk out of order, or a first chunk without `mime_type`. The partial answer is discarded. |
| `STT_NO_SPEECH` | yes | no | The transcript is empty. The state returns to `listening`; ask the Candidate to answer again. |
| `STT_FAILED` | yes | no | Speech-to-text failed or timed out. The state returns to `listening` (Integrations §3.2). |
| `ANSWER_TOO_LONG` | yes | no | The transcriber reports a recording longer than 3 minutes (plus 5 s grace). The state returns to `listening`; ask for a shorter answer. |
| `VALIDATION_ERROR` | yes | no | The answer text is empty or longer than 8000 characters. |
| `APPLICATION_NOT_ELIGIBLE` | no | yes | Recruitment window closed (24-hour deadline) or the Application was decided. Close `4003`. |
| `INTERNAL_ERROR` | yes | no / yes | Storage or engine failure. Non-fatal: resend the answer. Fatal: close `1011`, reconnect. |

## 5. Client messages

### `turn.played`
`{ "position": 4 }`. The interviewer turn finished playing (or the text was shown). Moves
`interviewer_speaking` to `listening`. A repeat for the same position, or an older one, is acknowledged
as a duplicate.

### `answer.audio`
One answer is one recording, sent as one message or as consecutive chunks that share the same `id`:

```json
{ "position": 4, "chunk": 0, "final": false, "mime_type": "audio/webm;codecs=opus", "audio": "<base64>" }
```

- Accepted only in `listening`, for the current position.
- `chunk` starts at 0 and increases by one. `mime_type` is required on chunk 0 and ignored after. A
  repeated chunk number is ignored (retransmit). A new `id` while a recording is open discards the open
  one.
- `final: true` on the last chunk; the server then moves to `processing`.
- Formats: `audio/webm` (Opus; Chrome, Edge, Firefox `MediaRecorder` default), `audio/ogg` (Opus),
  `audio/mp4` (Safari `MediaRecorder`), `audio/wav` (tests). Codec parameters are allowed. The first
  bytes must match the container.
- Recommended recorder settings: `new MediaRecorder(stream, { mimeType: "audio/webm;codecs=opus",
  audioBitsPerSecond: 32000 })`, collect the blobs, then send the whole recording in chunks of up to
  256 KiB.

**Why one recording per answer, not a live stream:** the STT boundary (KAN-55, local Whisper) transcribes a
whole utterance, and a complete file is a valid container that any decoder accepts. The browser already
detects the end of speech (`voice-activity.ts`). Streaming partial transcripts can be added in a later
version without changing turn semantics.

### `answer.text`
`{ "position": 4, "text": "…" }`. A typed answer (accessibility fallback, or no microphone). Same rules as
`answer.audio`, without STT. 1 to 8000 characters after trimming.

### `ping`
`{}`. The server answers `pong` with `reply_to`. Optional: the server also sends WebSocket ping frames,
which browsers answer automatically.

## 6. Limits

| Limit | Value | Why |
|---|---|---|
| Inbound message | 512 KiB | One 256 KiB chunk as base64 (about 342 KiB) plus the envelope. Larger frames close with `1009`. |
| Audio chunk | 256 KiB decoded | Keeps each frame small enough to parse without buffering a whole answer twice. |
| Answer audio | 4 MiB decoded, at most 1024 chunks | Opus at 32 kbps is about 4 KB/s, so 4 MiB is about 17 minutes, far longer than an answer. At 128 kbps it is still about 4 minutes. |
| Answer text | 8000 characters | Same as `POST /interviews/{id}/turns`. |
| Interviewer audio | 3 MiB decoded | Over that, the turn is sent as text (`audio_status: unavailable`). |
| Keepalive | ping every 25 s; closed after 60 s without any frame | Detects dead peers (laptop sleep, network loss) without app-level timers. |
| Write timeout | 10 s per message | A stalled client cannot block the engine. |
| STT timeout | `INTERVIEW_STT_TIMEOUT` (30 s) | Bounds `processing`. |
| Protocol errors | 8 per connection | Then close `4000`. |
| Stream memory | 30 min after the last socket closes | Keeps `seq` and remembered ids for reconnects. |

**Answer duration: 3 minutes** (product rule, team decision 2026-10-30). The client shows a countdown and
stops the recorder at 3:00. The server enforces it from the transcriber's `Transcript.Duration` (browser
WebM recordings usually carry no duration): a recording longer than 3 minutes plus 5 s of grace is refused
with `ANSWER_TOO_LONG`. While the transcriber reports no duration (the development fake), only the byte
limits above apply; at the recommended 32 kbps, 4 MiB is roughly 17 minutes, so they are a technical
backstop, not the rule. Pause and abandon timeouts remain open (KAN-215, KAN-180).

## 7. Close codes

| Code | Meaning | Client action |
|---|---|---|
| 1000 | Interview completed (after `session.completed`). | Go to the result. |
| 1001 | API shutting down. | Reconnect with backoff. |
| 1006 | (Browser-side) connection lost, or refused before the upgrade. | `GET /interviews/{id}`: 2xx with `completed_at: null` means reconnect with backoff; 401, 403 or 404 means stop. |
| 1009 | Message too big. | Fix the client; do not resend the same frame. |
| 1011 | Internal error. | Reconnect with backoff. |
| 4000 | Protocol violation (wrong subprotocol or too many protocol errors). | Do not reconnect automatically. |
| 4001 | Superseded by a newer connection to the same session. | Do not reconnect; tell the Candidate the interview continues elsewhere. |
| 4003 | Session no longer accepts answers (recruitment deadline or Application decided). | Do not reconnect. |

## 8. Reconnect and recovery

- Persisted at every step, so an abnormal disconnect loses at most the recording in flight:
  - the asked question's text and time (`interview_questions.spoken_text`, `asked_at`);
  - the session's provider voice (`interviews.provider_voice_id`);
  - each answer (`conversation_turns`, with `kind`, `asked_at`, `answered_at`).
- An answer whose last chunk reached the server is stored even if the socket drops during `processing`;
  the reconnect then starts at the next question.
- A partial recording (no `final` chunk) is discarded; the reconnect asks the same question again.
- After reconnecting, the server re-sends the current `interviewer.turn` in `interviewer_speaking`. A
  client that already played it may send `turn.played` right away.
- A session whose answers are all stored but which was not completed (the socket dropped in between)
  completes on the next connection.

## 9. STT boundary (for KAN-55)

The engine depends on one consumer-side interface (`runtime.Transcriber`):

```go
type Transcriber interface {
	Transcribe(ctx context.Context, audio Audio) (Transcript, error)
}

type Audio struct {
	Data     []byte // one complete recording, exactly as the browser recorded it
	MimeType string // normalized: "audio/webm", "audio/ogg", "audio/mp4" or "audio/wav"
}

type Transcript struct {
	Text     string        // plain text; empty means no speech
	Duration time.Duration // length of the decoded recording; 0 means unknown
}
```

- Honour `ctx`: the engine applies `INTERVIEW_STT_TIMEOUT`.
- Return `runtime.ErrNoSpeech` for silence (or an empty `Text`). Return `runtime.ErrUnsupportedAudio`
  for audio you cannot decode. Any other error is reported as `STT_FAILED`, and the Candidate may answer
  again.
- Never log the audio or the transcript: recruitment answers are Recruiter-only evidence.
- Set `Duration` to the decoded length of the recording. The engine refuses answers longer than 3 minutes
  (plus 5 s grace) with `ANSWER_TOO_LONG`; without a duration it cannot enforce the rule.
- English only (the interview voices are en-US).
- Wiring: add a case for your provider in `provider.ProvideTranscriber` and select it with
  `INTERVIEW_STT_PROVIDER`. Until then the only value is `fake`, a deterministic transcriber that
  returns a fixed sentence for any non-empty audio.

## 10. Configuration (`interview.*`)

| Env | Default | Meaning |
|---|---|---|
| `INTERVIEW_STT_PROVIDER` | `fake` | Transcriber. Only `fake` exists until KAN-55. |
| `INTERVIEW_STT_TIMEOUT` | `30s` | Bound for one transcription (1s to 2m). |
| `INTERVIEW_DEFAULT_PROVIDER_VOICE_ID` | `en-US-JennyNeural` | Provider voice for sessions without a voice ("Default voice" in setup; team decision 2026-10-30). Empty: such sessions get text-only turns. |
| `INTERVIEW_TTS_CACHE_ENTRIES` | `128` | In-memory cache of synthesized turns per (provider voice, text); `0` disables. |
