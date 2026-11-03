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
`ai.<task>` sections; credentials must stay in env. Bedrock and Jev are deferred,
with the existing BaseChatModel and task interfaces remaining the extension seams.

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
