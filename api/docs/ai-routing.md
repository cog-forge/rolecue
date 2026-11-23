# AI task routing (KAN-273)

No route is enabled by default: unset `AI_*` settings keep the existing `LLM_*`
model, optional-key behavior, service validation and retry policy. Set a task provider
explicitly to enable routing. No schema, database or WebSocket contract changes.

| Task / env prefix | Suggested primary | Ordered fallback |
| --- | --- | --- |
| JD extraction / `AI_JD_EXTRACTION` | Groq `openai/gpt-oss-120b` | Gemini |
| Question bank / `AI_QUESTION_BANK` | Groq `openai/gpt-oss-120b` | Gemini |
| Live follow-up decision + text / `AI_FOLLOW_UP` | Groq `openai/gpt-oss-20b` | Gemini, if budget remains |
| Evaluation / `AI_EVALUATION` | Groq `openai/gpt-oss-120b` | Gemini |

Each prefix accepts `_PROVIDER`, `_MODEL`, `_TIMEOUT`, `_MAX_RETRIES`, `_FALLBACK`.
Unspecified fields inherit `LLM_*`; setting provider `groq` without a model selects
the model above. `GROQ_API_KEY` is read only from the environment. Gemini uses
`LLM_API_KEY`. Neither credential is logged. Fallback is a comma-separated ordered
list, e.g. `gemini` or `gemini:gemini-3.1-flash-lite,groq:openai/gpt-oss-20b`.
Bare `gemini` inherits `LLM_MODEL`. Other entries require explicit model IDs.
Fallback entries share the task timeout and retry count. Invalid providers, missing
keys/models, invalid timeout (positive, at most 10m), and retries outside 0–3 fail startup.

`.env.example` contains the complete opt-in recipe: 30s for JD/evaluation, 90s for
bank generation, 5s for follow-ups, zero retries, Gemini fallback. YAML has matching
`ai.<task>` sections; credentials must stay in env. Bedrock is deferred. Jev is not
used; Clef (below) took its place for the follow-up decision.

## Execution and validation

A configured task has one absolute timeout, bounded further by its caller's context.
The live route is additionally capped at 5s; the existing service still skips a failed
decision. Remaining time is divided between remaining providers/retries so a slow
primary cannot consume every fallback opportunity. Max retries means additional
attempts at each entry. A 429 goes immediately to the next entry, with no sleep or
same-provider retry, regardless of Retry-After; this preserves interactive budgets.
Timeouts, network failures, 5xx and invalid structured output can retry/fall back.
Authentication and other 4xx errors stop the route. Cancellation stops immediately.

Groq uses the OpenAI-compatible `/openai/v1/chat/completions` endpoint, JSON object
mode, low reasoning effort, one HTTP request per Generate, no SDK retry and no
redirects. The existing strict decoders and domain validators decide acceptance,
including JD evidence checks, bank distinctness, follow-up rules and scoring IDs/ranges.
JSON mode ensures syntax; application validators enforce the schema and semantics.
No rejected response is replayed as an instruction. Final routing errors contain
only classified reasons, avoiding provider response bodies in upstream service logs.
Logs include task, provider, model, duration, reason/status, Retry-After and token
usage where supplied; prompts, answers, credentials and response bodies are omitted.

Groq banks use sequential batches of at most 10. Previous questions are sent in an
explicit untrusted data field to avoid repetition; prompt rules and injection
handling remain intact. Each attempt validates the combined bank with the existing
storage validator, which deduplicates case-insensitively. A cross-batch duplicate
that leaves too few questions triggers retry/fallback. Only the complete, validated
bank is returned for storage. Every batch shares the bank's absolute deadline.

## Follow-up decision model: Clef (KAN-274)

[Clef](https://developers.cloudflare.com/workers-ai/models/clef/) is Cloudflare's decision model on Workers AI.
It answers typed questions about a `state` with probabilities and **writes no text**, so it is not a chat
model and is not a route entry. It sits in front of the `follow_up` chat route behind `FollowUpDecider`:

| Clef result | What happens |
| --- | --- |
| yes probability below the threshold | No follow-up; no chat call is made |
| at or above the threshold | The chat route only **words** the question (a prompt whose reply must ask); a reply that declines or fails the validator skips the follow-up as `invalid_output` |
| failure: timeout, 429, 5xx, 401/403, network, invalid reply | The chat route decides and words as before, in the rest of the live budget |
| cancellation or the caller's deadline | Returned as is; nothing else is called |

Off by default. Settings (env, YAML `ai.follow_up_decision`): `AI_FOLLOW_UP_DECISION_PROVIDER=clef`,
`_MODEL` (`clef-flash` default, or `clef`), `_THRESHOLD` (default 0.5, between 0 and 1), `_TIMEOUT` (default 2 s,
at most 5 s). Credentials only in env: `CLOUDFLARE_ACCOUNT_ID` (32 hex) and `CLOUDFLARE_AI_API_TOKEN` (Workers AI
permission). Without the provider set no Cloudflare value is needed; with it, missing or malformed values fail
startup. Evaluation is unchanged and never uses Clef.

The Clef call gets at most half of what the live budget has left, so its fallback always has time. The state is
the same cleaned, redacted (`textsafe.Redact`) and capped `untrusted_*` object the chat decider sends; the
question (`noul`, fixed English instructions plus yes/no criteria chosen by the guideline's follow-up focus) never
contains input text. The REST call is `POST /client/v4/accounts/{id}/ai/run/@cf/cloudflare/<model>` with
`Authorization: Bearer`; the envelope `{result, success, errors, messages}` was confirmed live on 2026-11-18.
Errors carry a status or reason, never the body. The `llm.task_attempt` log (task `follow_up_decision`, provider
`clef`) holds model, duration, reason/status, `input_tokens`/`output_tokens`, the rounded probability and the
`ask` outcome; never state, answers, instructions or credentials. One follow-up decision still spends one unit
of the daily LLM budget, however many calls it makes inside.

Pricing (checked 2026-11-18): `clef` $0.24 and `clef-flash` $0.038 per million input tokens, output free; the
Workers AI free allocation is 10,000 neurons per day (`clef-flash` 3,455 neurons per million tokens, so roughly
2.9 M input tokens a day). The model page states zero data retention and does not state a region or a rate
limit for Clef.

Live check (billable, opt-in): `go test -tags ai_live ./internal/pkg/ai -run TestLiveClef -v`. One synthetic
sample per case on 2026-11-18 (not a benchmark): a decision used 298–369 input tokens, so the free allocation
covers roughly 8,000 `clef-flash` decisions a day. `clef-flash` answered in 0.13–0.31 s, `clef` in 0.35–0.55 s.
Yes probability (`clef-flash` / `clef`): vague answer 0.76 / 0.94, complete answer 0.12 / 0.27, an answer that
asks the model to "answer no" 0.87 / 0.79 (it was also vague). Clef then Groq wording took 0.79 s end to end.

## Transient failures, retries and the daily budget (KAN-382, KAN-383)

The route above handles one call: it tries each provider once and falls back, and a 429 never sleeps. A call
that failed on **every** provider with a transient reason is then repeated by its caller, above the route:

| Task | What is repeated | Repeats | Retried reasons | Bound |
| --- | --- | --- | --- | --- |
| Roadmap topics, question bank | one topic proposal, or one bank batch (accepted batches are kept) | 2, backoff 1 s then 2 s | `timeout`, `rate_limited`, `http_5xx`, `network`, `invalid_output` (truncated JSON, unexpected EOF) | the task budget (`AI_QUESTION_BANK_TIMEOUT`, 90 s) for the whole step; a repeat is skipped when less than 8 s is left |
| Evaluation | one core question's grading call (the other questions are not asked again) | 2, backoff 1 s then 2 s | `timeout`, `rate_limited`, `http_5xx`, `network` | the evaluation's 2 minute bound; a repeat is skipped when less than 15 s is left |

JD extraction and the live follow-up are not repeated (the follow-up has a 5 s budget). Authentication and other 4xx
errors, cancellation and a spent deadline are never repeated. A grade that is decoded but invalid is not repeated
here: the route already retries or falls back on it.

Grading keeps the questions it has graded. If one question still fails after its repeats the session is not scored
(no partial credit): its evaluation fails and the other questions' grades are kept in memory for 30 minutes, so the
next claim of the same session grades only the missing question. That memory is per API process; after a restart a
claim grades every question again.

**Reading the daily LLM budget** (default 200 operations per user and UTC day, `LLM_USER_DAILY_CALL_BUDGET`). One unit is
spent by: each generation request (confirm, retry, regenerate), a top-up, a new practice session (reserved at start,
and given back if the start creates nothing), and every evaluation claim after the first of a session (a repeated
`POST /interviews/{id}/complete` after a failed evaluation). The first evaluation is paid by the unit reserved at start.
A repeated model call inside one request or one evaluation never costs a unit.

**Reading `EVALUATION_ATTEMPTS_EXCEEDED`** (409 on `POST /interviews/{id}/complete`). Each claim of an evaluation is
one session-level attempt, at most 3 per session. The code means all 3 claims ended in a failed evaluation and the
session will not be evaluated again. With per-call repeats a single 429, 5xx or timeout no longer consumes an attempt;
an attempt is lost only when a question's repeats ran out, or on a non-transient error (provider rejection, invalid
grade, save error). To diagnose, read the API log for the interview id: `grading call failed, retrying` lines show the
repeats (with reason and question position), `llm.task_attempt` lines show each provider try with its reason, and the
`evaluation.grade` step failure shows the final cause. A retry the daily budget cannot pay for is refused with
`LLM_BUDGET_EXCEEDED` and gives its attempt back. The only way to reset the attempt counter is an operator action on
the database; no endpoint does it.

## Limits, vision and capacity

[Groq models](https://console.groq.com/docs/models),
[JSON/structured output support](https://console.groq.com/docs/structured-outputs),
and [rate limits](https://console.groq.com/docs/rate-limits) were checked on 2026-11-03.
The real key listed all three target IDs and accepted JSON object mode for each.
GPT-OSS 20B/120B also support strict structured outputs; this adapter uses JSON
object mode so all task-specific, unchanged validators remain authoritative.

The stated Free limits per model are 30 RPM, 8K TPM, 200K tokens/day. Batching bounds
individual requests; it does not guarantee that repeated batches fit TPM, particularly
as the exclusion list grows. 429 fallback remains necessary. There is no distributed
quota scheduler or daily quota counter in this change. Do not parallelize bank batches.

Both GPT-OSS models are text-only. Optional scanned-PDF/image extraction is deferred:
the existing local PDF parser remains the intake path. A future vision adapter should
consume trusted parser output/images through `qwen/qwen3.8-27b`, then pass its result
through the same JD validators and Gemini fallback. The current Groq adapter accepts
text messages only; it does not enable image upload. Llama 4 Scout is retired and is
not configured anywhere by this feature.

A measured 8-core/8-follow-up Practice used about 900 tokens for JD, 7.4–8.1K for
50 questions, 4.8–5.4K for follow-ups and 2.0K for evaluation. A new JD and bank for
every interview therefore needs roughly 10.3–11K GPT-OSS-120B tokens and 4.8–5.4K
GPT-OSS-20B tokens: about **18–19 full Practices/day** before the 120B daily quota
forces more Gemini use (follow-up daily capacity alone: about 37–41). This is an
estimate, not a reservation: TPM may trigger fallback on the first interview.
Reusing an existing bank improves daily capacity substantially; transcript lengths,
reasoning tokens, retries and other workloads change it.

## Verification and observed runs

From `api/`, load this worktree's env without printing it, then:

```sh
set -a; . ../.env; set +a
make fmt-check vet test lint build
make test-integration
go test -tags ai_live ./internal/pkg/ai -run TestLive -v
go test -tags 'ai_live integration' ./internal/features/interview/tests -run TestRoutingPracticeClone -v
```

Live checks are opt-in and make billable/quota-consuming calls. The Practice check
requires clone port 55473, creates isolated fixtures, spends only fixture coins and
cleans those fixtures. It exercises services and the database, without browser,
WebSocket, speech or real-user transcript logging.

Synthetic task comparison (one sample each, not a benchmark):

| Task | Groq-first latency | Gemini-only latency | Tokens, Groq-first / Gemini-only |
| --- | --- | --- | --- |
| JD | 1.535s | 1.557s | 929 / 642 |
| Bank, 50 | 7.906s (5 batches) | 4.710s (1 call) | 8088 / 1291 |
| Follow-up | 0.707s | 0.728s | 524 / 438 |
| Evaluation | 1.432s (one fallback) | 1.031s | 491 / 470 |

The Groq-first comparison made 9 attempts, one of which was Gemini fallback.
Token counts include successful responses from whichever provider answered.

A recorded clone Practice completed in 28.09s: 50 stored questions, 8 core answers,
8 answered follow-ups, fixture balance 10 → 5, persisted evaluation `completed`.
It made 17 attempts: 15 Groq, 2 Gemini. Bank batch five hit Groq 429 and fell back;
one follow-up failed the existing validator and fell back within 5s. JD took 1.43s,
bank attempts totaled 9.51s, follow-up attempts 8.53s, evaluation 1.87s.
Tokens by provider/task:

| Task | Groq | Gemini |
| --- | ---: | ---: |
| JD | 900 | 0 |
| Bank | 5676 | 2097 |
| Follow-ups | 4769 | 516 |
| Evaluation | 2002 | 0 |

After the final error-sanitization change, the clone Practice passed again in
17.26s with the same 50/8/8, balance and persisted evaluation results. Its 16
attempts included one Gemini evaluation fallback after Groq 429. JD took 1.47s,
bank 7.10s, follow-ups 5.42s total, evaluation 2.61s including fallback. Tokens:
Groq JD 936, bank 7574, follow-ups 4792; Gemini evaluation 2033. Raw numeric
check evidence is kept locally under `api/.tools/kan273-evidence/` (ignored).

Default checks passed: 50 package test suites, vet, fmt-check, lint (0 issues), and
build. Integration checks passed for 61 package suites on the clone. Race checks
passed for ai/config/provider. Eight task/provider live cases, three model-ID/JSON
checks and the full clone Practice passed. Tagged live checks also passed vet/lint.
Frontend checks were not run because no frontend files changed.

No frontend, WebSocket schema, migration, Bedrock or Jev implementation changed.
The two new service validation exports only delegate to existing validators; their
rules are unchanged. At merge, coordinate those small exports with the follow-up
schema owner. Brain Integrations and Product Decisions should record task routes,
Groq quotas, deferred vision/Bedrock/Jev and measured fallback behavior; Jira's older
Bedrock acceptance item needs reconciliation with the newer user decision. Neither
Brain nor Jira was modified.
