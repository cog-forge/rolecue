# Learning Roadmap API contract (KAN-341)

This is the contract for the person building the Roadmap UI (RM-12 to RM-14). Together with Swagger
(`/swagger`, tag `roadmap`) it is everything the UI needs; nothing here requires reading the backend code.

Source of truth for the business rules: `Learning-Roadmaps-Spec.md` (Decision 25). This document states what the
API returns and how a client should use it.

> **Status of the sections.** Sections 1 to 8 describe the read API (KAN-352) and are implemented. Section 9.1
> to 9.4 (confirm, retry, regenerate, practice start with `focus_item_id`) are implemented by the generation lane.

## 1. Basics

- Base path and envelope are the same as every other endpoint. A success is
  `{"success": true, "data": ...}`; a failure is `{"success": false, "error": {"code": "...", "message": "..."}}`.
  Branch on `error.code`; `message` is English backend text and must not be shown.
- Authentication is the session cookie (`credentials: include`). Both read endpoints are **Candidate only** and
  only for the **owner of the Role Profile**. Responses carry `Cache-Control: no-store`.
- All fields are `snake_case`. Timestamps are RFC 3339 in UTC. UUIDs are lowercase strings.
- Practice results only. A recruitment interview never appears in a Roadmap, a score or a history.
- **No score is stored.** Every number is computed from the stored practice results when you ask. There is
  nothing to refresh or invalidate; reading again gives the current truth.

## 2. Two different statuses

| Field | Values | Meaning |
| --- | --- | --- |
| `generation_status` | `pending`, `generating`, `ready`, `failed` | Is the Roadmap's content prepared? It says nothing about how well the Candidate is doing. |
| `learning_status` | `not_started`, `in_progress`, `completed` | How the Candidate is doing. Computed from practice results. |

Show the preparation state first. While `generation_status` is not `ready` there are no topics to practise
and `progress_percent` is `null`.

| `generation_status` | What the UI shows | Topics (`items`) |
| --- | --- | --- |
| `pending` | "Your Roadmap will be prepared once the requirements are confirmed" (the Role Profile is not confirmed yet), or a retry action for an older confirmed profile (section 9) | empty |
| `generating` | A progress state; poll (section 5). Confirming a Role Profile is synchronous (section 9.1), so the Candidate who confirmed normally never sees this state; another tab or device can | the currently published set: empty on the first generation, the **previous** set while a regenerate is running (the old topics stay until the new set is published) |
| `failed` | An error state with a retry action (section 9) | the same rule: empty after a failed first generation, the previous set after a failed regenerate |
| `ready` | The linear Roadmap | one entry per topic |

Items can therefore be present while `generation_status` is `generating` or `failed`. `progress_percent` is
`null` in every state except `ready`, so draw the preparation state first and do not present those topics as the
current plan.

`ready` with an empty `items` array can exist. Treat it as "no topics": `progress_percent` is `null` and
`learning_status` is `not_started`. A Roadmap is never `completed` while it is empty or not ready.

## 3. `GET /role-profiles/{id}/roadmap`

Returns the Roadmap of one of the Candidate's own Role Profiles.

### Response `data`

| Field | Type | Null? | Meaning |
| --- | --- | --- | --- |
| `id` | uuid | no | Roadmap id. |
| `role_profile_id` | uuid | no | The profile it belongs to (the `{id}` of the request). |
| `generation_status` | string | no | `pending` \| `generating` \| `ready` \| `failed`. |
| `learning_status` | string | no | `not_started` \| `in_progress` \| `completed`. |
| `progress_percent` | number | **yes** | `100 x completed_items / total_items`, two decimals. `null` unless `generation_status` is `ready` **and** there is at least one topic. Never 100 for an empty Roadmap. |
| `completed_items` | integer | no | Number of topics that are `achieved` right now. |
| `total_items` | integer | no | Number of topics. |
| `practice_started` | boolean | no | `true` once the profile has any practice session, finished or not. Regenerating the Roadmap is only possible while this is `false`. |
| `update_state` | string | no | `up_to_date` \| `updating` \| `delayed`. See section 6. |
| `pending_result_count` | integer | no | How many evaluated practice answers are not part of the Roadmap yet. `0` when `up_to_date`. |
| `window_size` | integer | no | Always `5`: how many different questions a topic needs. |
| `achieved_threshold` | number | no | Always `80`: the minimum mean score of those questions. Read these two instead of hard-coding them. |
| `items` | array | no | The topics, sorted by `position`, then `id`. Never `null`; `[]` when there are none. |

### Item

| Field | Type | Null? | Meaning |
| --- | --- | --- | --- |
| `id` | uuid | no | Topic id. Use it for the history endpoint and for `focus_item_id`. |
| `title` | string | no | Topic name, for example "Go concurrency". Generated content: show it as it is. |
| `description` | string | **yes** | Scope of the topic. |
| `position` | integer | no | Order to draw the topics in. It is **not** a lock: every topic can be practised at any time. |
| `score` | number | **yes** | Mean of the topic's current window, two decimals. `null` when the topic has no result yet (draw "not evaluated yet", never 0). **Provisional** while `evidence_question_count` is below 5. |
| `evidence_question_count` | integer | no | Different questions in the current window, `0` to `5`. |
| `learning_status` | string | no | `not_started` \| `in_progress` \| `achieved`. |
| `last_evaluated_at` | timestamp | **yes** | When the newest answer of the window was evaluated. |

### How the numbers are computed (what the UI must explain)

1. For every question of the topic only its **latest** result counts. Answering the same question again
   replaces the old result; it never adds a question.
2. The **five most recent different questions** form the window. Older questions fall out of the window as new
   ones arrive; the full list stays in the history.
3. `score` is the mean of the window. With 1 to 4 questions it is provisional and the topic is `in_progress`.
4. A topic is `achieved` only with **5** different questions **and** a mean of **at least 80**. The comparison
   uses the unrounded mean, so 79.996 is shown as `80` but is **not** achieved; exactly 80 is.
5. A question that was shown to the Candidate and not answered when the session ended counts as **0**.
   A question that was never asked is not counted. A question that could not be graded is not counted.
6. `learning_status` of the Roadmap: `not_started` when no topic has a result, `completed` when it is ready,
   has at least one topic and **every** topic is `achieved`, otherwise `in_progress`.
7. Statuses move both ways. A low new result can turn `achieved` back into `in_progress`, and `completed` back
   into `in_progress`. There is no permanent badge.

`score` is answer quality; `progress_percent` is the share of topics that reached the requirement. A high score
in one topic never makes up for another topic.

### Examples

All examples show `data` only.

**`pending`**

```json
{
  "id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "pending",
  "learning_status": "not_started",
  "progress_percent": null,
  "completed_items": 0,
  "total_items": 0,
  "practice_started": false,
  "update_state": "up_to_date",
  "pending_result_count": 0,
  "window_size": 5,
  "achieved_threshold": 80,
  "items": []
}
```

**`generating`** and **`failed`** are the same shape with `"generation_status": "generating"` or `"failed"`.

**`ready`, empty** (no topics; no progress)

```json
{
  "id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "ready",
  "learning_status": "not_started",
  "progress_percent": null,
  "completed_items": 0,
  "total_items": 0,
  "practice_started": false,
  "update_state": "up_to_date",
  "pending_result_count": 0,
  "window_size": 5,
  "achieved_threshold": 80,
  "items": []
}
```

**`ready`, topics, nothing practised yet** (`progress_percent` is `0`, not `null`)

```json
{
  "id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "ready",
  "learning_status": "not_started",
  "progress_percent": 0,
  "completed_items": 0,
  "total_items": 3,
  "practice_started": false,
  "update_state": "up_to_date",
  "pending_result_count": 0,
  "window_size": 5,
  "achieved_threshold": 80,
  "items": [
    {"id": "a1111111-1111-4111-8111-111111111111", "title": "SQL", "description": "Indexes, joins, transactions", "position": 1,
     "score": null, "evidence_question_count": 0, "learning_status": "not_started", "last_evaluated_at": null},
    {"id": "a2222222-2222-4222-8222-222222222222", "title": "Go concurrency", "description": "Goroutines, channels, race conditions", "position": 2,
     "score": null, "evidence_question_count": 0, "learning_status": "not_started", "last_evaluated_at": null},
    {"id": "a3333333-3333-4333-8333-333333333333", "title": "System design", "description": null, "position": 3,
     "score": null, "evidence_question_count": 0, "learning_status": "not_started", "last_evaluated_at": null}
  ]
}
```

**`ready`, provisional scores** (spec example: SQL has 2 of 5 questions, Go has 3 of 5; nothing is achieved)

```json
{
  "id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "ready",
  "learning_status": "in_progress",
  "progress_percent": 0,
  "completed_items": 0,
  "total_items": 3,
  "practice_started": true,
  "update_state": "up_to_date",
  "pending_result_count": 0,
  "window_size": 5,
  "achieved_threshold": 80,
  "items": [
    {"id": "a1111111-1111-4111-8111-111111111111", "title": "SQL", "description": "Indexes, joins, transactions", "position": 1,
     "score": 70, "evidence_question_count": 2, "learning_status": "in_progress", "last_evaluated_at": "2026-11-13T03:20:11Z"},
    {"id": "a2222222-2222-4222-8222-222222222222", "title": "Go concurrency", "description": "Goroutines, channels, race conditions", "position": 2,
     "score": 70, "evidence_question_count": 3, "learning_status": "in_progress", "last_evaluated_at": "2026-11-13T04:05:42Z"},
    {"id": "a3333333-3333-4333-8333-333333333333", "title": "System design", "description": null, "position": 3,
     "score": null, "evidence_question_count": 0, "learning_status": "not_started", "last_evaluated_at": null}
  ]
}
```

**`ready`, one topic achieved** (SQL 82 over 5 questions, Go 72 over 5, System design not started:
1 of 3 achieved, `progress_percent` 33.33, the Roadmap is **not** completed)

```json
{
  "id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "ready",
  "learning_status": "in_progress",
  "progress_percent": 33.33,
  "completed_items": 1,
  "total_items": 3,
  "practice_started": true,
  "update_state": "up_to_date",
  "pending_result_count": 0,
  "window_size": 5,
  "achieved_threshold": 80,
  "items": [
    {"id": "a1111111-1111-4111-8111-111111111111", "title": "SQL", "description": "Indexes, joins, transactions", "position": 1,
     "score": 82, "evidence_question_count": 5, "learning_status": "achieved", "last_evaluated_at": "2026-11-13T05:10:00Z"},
    {"id": "a2222222-2222-4222-8222-222222222222", "title": "Go concurrency", "description": "Goroutines, channels, race conditions", "position": 2,
     "score": 72, "evidence_question_count": 5, "learning_status": "in_progress", "last_evaluated_at": "2026-11-13T05:10:00Z"},
    {"id": "a3333333-3333-4333-8333-333333333333", "title": "System design", "description": null, "position": 3,
     "score": null, "evidence_question_count": 0, "learning_status": "not_started", "last_evaluated_at": null}
  ]
}
```

**`ready`, completed** (every topic achieved)

```json
{
  "id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "ready",
  "learning_status": "completed",
  "progress_percent": 100,
  "completed_items": 2,
  "total_items": 2,
  "practice_started": true,
  "update_state": "up_to_date",
  "pending_result_count": 0,
  "window_size": 5,
  "achieved_threshold": 80,
  "items": [
    {"id": "a1111111-1111-4111-8111-111111111111", "title": "SQL", "description": "Indexes, joins, transactions", "position": 1,
     "score": 84.6, "evidence_question_count": 5, "learning_status": "achieved", "last_evaluated_at": "2026-11-13T05:10:00Z"},
    {"id": "a2222222-2222-4222-8222-222222222222", "title": "Go concurrency", "description": "Goroutines, channels, race conditions", "position": 2,
     "score": 80, "evidence_question_count": 5, "learning_status": "achieved", "last_evaluated_at": "2026-11-13T06:30:00Z"}
  ]
}
```

## 4. `GET /role-profiles/{id}/roadmap/items/{item_id}/history`

The practice answers that feed one topic, for a "history of this topic" screen. It shows the **full** history,
and marks which answers are the ones the current score is computed from.

Query parameters: `limit` (1 to 100, default 20) and `offset` (0 or more, default 0), the same paging as
`GET /role-profiles`. Each parameter may appear once. Anything else is `VALIDATION_ERROR`.

Response `data` is the repository's standard page: `{"items": [...], "pagination": {"limit", "offset", "total"}}`.
`total` is the number of answers in the whole history. The order is **newest first** (evaluation time, then
interview id, then question number) and is stable across pages.

Each row is one practice answer that was evaluated:

| Field | Type | Null? | Meaning |
| --- | --- | --- | --- |
| `interview_id` | uuid | no | The practice session. Open its report with `GET /interviews/{interview_id}/report`. |
| `question_number` | integer | no | The number of the question in that session; the practice report uses the same number. |
| `result` | string | no | `scored`: the question was answered and graded. `skipped`: it was shown to the Candidate and not answered; it counts as `0`. |
| `score` | number | no | The question score, 0 to 100; `0` for `skipped`. |
| `question_text` | string | **yes** | The question as it was asked and as the practice report shows it. `null` for `skipped` (the report does not list it). |
| `evaluated_at` | timestamp | no | When the session was completed and evaluated. |
| `in_current_window` | boolean | no | `true` for the (at most five) answers the topic score is computed from now. An older answer to a question that was answered again, and answers beyond the five most recent questions, are `false`. |

Example (topic with a repeated question; the first row is the newest):

```json
{
  "items": [
    {"interview_id": "c4444444-4444-4444-8444-444444444444", "question_number": 1, "result": "scored", "score": 20,
     "question_text": "Explain how an index speeds up a query.", "evaluated_at": "2026-11-13T07:00:00Z", "in_current_window": true},
    {"interview_id": "c3333333-3333-4333-8333-333333333333", "question_number": 2, "result": "skipped", "score": 0,
     "question_text": null, "evaluated_at": "2026-11-13T05:10:00Z", "in_current_window": true},
    {"interview_id": "c1111111-1111-4111-8111-111111111111", "question_number": 1, "result": "scored", "score": 70,
     "question_text": "Explain how an index speeds up a query.", "evaluated_at": "2026-11-13T03:20:11Z", "in_current_window": false}
  ],
  "pagination": {"limit": 20, "offset": 0, "total": 3}
}
```

A topic with no results returns `{"items": [], "pagination": {"limit": 20, "offset": 0, "total": 0}}`.
Answers that are evaluated but not yet part of the Roadmap (section 6) are **not** in the history yet.

## 5. Polling while `generating`

Generation is **synchronous** for the person who starts it: `POST .../confirm`, `.../roadmap/retry` and
`.../roadmap/regenerate` return only when the Roadmap is `ready` (or with an error), in about **40 to 50 seconds**
(two model steps: the topics, then the question bank). The server-side deadline of these requests is **200
seconds**. Set the client timeout of these three calls to **at least 200 seconds** and show a progress state; do
not poll to learn the result of your own call.

> **For the UI developer.** The frontend fetch timeout of `confirmRoleProfile` and `analyze` is currently
> 150_000 ms (`frontend/src/features/role-profile/api/requests.ts`). It must be raised to at least 200_000 ms,
> otherwise the browser gives up before the server does. This is UI work and is not done in this backend change.

Polling is for the **other** cases, when someone else started the generation (another tab or device), or you
arrived while it runs:

- Poll `GET /role-profiles/{id}/roadmap` (the same endpoint) while `generation_status` is `generating`.
- Interval: every **3 seconds** for the first 30 seconds, then every **5 seconds**. Stop on `ready` or `failed`.
  Pause while the tab is hidden and read once when it becomes visible again.
- Never poll faster than 3 seconds.
- A `failed` Roadmap stays `failed` until the Candidate retries; do not poll it.
- A generation that has run longer than **5 minutes** is considered dead: the next retry or confirm takes it over
  (section 9). So if it is still `generating` after **5 minutes**, stop polling and offer a retry; before that, a
  retry answers `409 ROADMAP_GENERATING` and the client keeps waiting.

## 6. `update_state`: finished practice that is not in the Roadmap yet

After a practice session is evaluated, its results are linked into the Roadmap by the backend, normally within a
moment. If that link does not happen at once (a failure, a restart), a background job (the "catch-up", every
minute by default) links it. The Roadmap read tells you honestly where things stand:

| `update_state` | Meaning | What to show |
| --- | --- | --- |
| `up_to_date` | Every evaluated practice answer of this profile is already part of the Roadmap. | Nothing special. |
| `updating` | `pending_result_count` evaluated practice answers are not part of the Roadmap yet, and the oldest of them is less than 10 minutes old (counted from the moment the session ended). The numbers shown are correct for the results already linked. | "Your latest session is being added to your Roadmap." Do **not** show 0 or "not started" as if nothing happened. |
| `delayed` | At least one of those answers has waited **more than 10 minutes since its session ended**. (If the evaluation itself only succeeded later, for example after a retry, the wait can already exceed that when the result first appears, so `delayed` can show up at once.) The catch-up normally clears a result within a minute or two, so something is wrong (it keeps failing, or it is switched off). Nothing is lost: the result is stored with the interview and will appear once it is linked. | "Your latest session has not been added to your Roadmap yet. It will appear later." Stop polling quickly, no error dialog; the Candidate can still read the practice report. |

While `updating`, read the Roadmap again every **10 seconds** for up to **2 minutes**, then stop and keep the
message ("it will appear shortly"). Read it again on the next visit; a `delayed` state clears by itself once the
backend links the result, so it is safe to show it on every visit until then.

`update_state` only counts answers of **completed practice sessions with a successful evaluation that could be
trusted**; a session that is still running, whose evaluation failed, or whose evaluation was flagged as
unreliable is not "pending" for the Roadmap (it never will be).

## 7. How practice is reported

- Only a **completed practice** session with a **successful evaluation** updates the Roadmap. A session that is
  unfinished, still being evaluated or failed to evaluate changes nothing. A session whose evaluation was flagged
  as unreliable (the answers tried to steer the grader, `injection_flagged`) is not evidence either and never
  counts (product owner decision, 13/11).
- A question counts for exactly one topic (its primary topic). Follow-up questions are part of their core
  question and never add a second result.
- A question that was **shown and not answered** when the session ended counts as **0** for the topic (it appears
  in the history as `skipped`), whether the Candidate left or the session was abandoned after the idle timeout
  (product owner decision, 13/11). A question that was **never reached** is not counted and not 0. A question that
  could not be graded (technical failure, no grading result) is not counted and not 0.
- This applies to the Roadmap's own per-question score only. The practice report and the session's overall score
  are not changed by it.
- Known limit: the system cannot tell a deliberate skip from a technical problem (for example speech
  recognition that never worked) that kept the Candidate from answering the **last shown** question before they
  left. At most one question per session can be in that state. The product owner accepted this limit: such a
  question counts as 0.

## 8. Errors and what the client should do

| HTTP | `error.code` | When | What the client should do |
| --- | --- | --- | --- |
| 400 | `VALIDATION_ERROR` | Malformed `id` or `item_id`, bad `limit`/`offset`. | A client bug. Do not retry. |
| 401 | `INVALID_TOKEN` | No valid session. | Send the user to sign in. |
| 403 | `FORBIDDEN` | The user is not a Candidate (Recruiter, Admin), or onboarding is not complete. | Hide the Roadmap; do not retry. |
| 404 | `JD_NOT_FOUND` | The Role Profile does not exist **or belongs to someone else** (the two are indistinguishable on purpose). | Show "this Role Profile no longer exists" and go back to the list. |
| 404 | `ROADMAP_ITEM_NOT_FOUND` | The topic is not part of this Role Profile's Roadmap (for example after a regenerate replaced the topics). | Reload the Roadmap and drop the stale topic. |
| 500 | `INTERNAL_ERROR` | Unexpected failure. | Show a generic error with a retry button; retry later. |

Codes of the confirm, retry, regenerate and practice-start endpoints are in section 9.

## 9. Confirm, retry, regenerate and starting a practice session

Common to the three generation calls (9.1 to 9.3): Candidate only, session cookie, **no request body**, the
Candidate must own the Role Profile (a foreign or unknown profile is `404 JD_NOT_FOUND`, indistinguishable). They
are **synchronous** and call a language model, so each request is rate limited and spends one unit of the
Candidate's model budget; they can take 40 to 50 seconds (section 5).

Response `data` of retry and regenerate (field names are `snake_case`):

```json
{
  "roadmap_id": "5d1f6f48-7a2e-4a0b-9d0c-5b3e8a1c2f10",
  "role_profile_id": "0b6a52c4-3d57-4f2f-8b7c-1a4f0e9d7c21",
  "generation_status": "ready"
}
```

`generation_status` is `pending`, `generating`, `ready` or `failed`; on a 200 it is the finished state (`ready`).
Read the topics with `GET /role-profiles/{id}/roadmap` afterwards.

### 9.1 `POST /role-profiles/{id}/confirm` (generates the Roadmap)

Confirming the reviewed requirements now also generates the Roadmap, in the same request: first the topics, then
the practice question bank mapped to those topics, published together. The response is the Role Profile, as before.

- Success: the profile is `confirmed` and its Roadmap is `ready` (about 40 to 50 seconds).
- Failure (a model error or invalid model output, see the error table): the profile stays in `review`, the Roadmap
  is `failed` with no topics and no bank. **Calling confirm again is the retry.**
- Two concurrent confirms produce one generation; the other gets `409 ROADMAP_GENERATING`.
- Confirming a profile that is already `confirmed` does **not** generate: use retry (9.2).
- A profile that practised before the Roadmap existed has no Roadmap and cannot get one: retry answers
  `409 ROADMAP_NOT_RETRYABLE`. The way forward is a new Role Profile (product owner decision).

### 9.2 `POST /role-profiles/{id}/roadmap/retry`

Starts the generation again for a Roadmap that is `pending` or `failed`.

- Allowed when the requirements are confirmed and the Roadmap is `pending` or `failed`.
- **Idempotent.** On a Roadmap that is already `ready` it is a no-op that returns 200 with `generation_status`
  `ready`; it never resets a ready Roadmap. Concurrent retries run one generation; the others get
  `409 ROADMAP_GENERATING`. A generation that has been `generating` for more than 5 minutes is taken over by the
  next retry instead of answering 409.
- Not allowed when the profile already has any practice interview, because its bank cannot be replaced
  (`409 ROADMAP_NOT_RETRYABLE`).

### 9.3 `POST /role-profiles/{id}/roadmap/regenerate`

Replaces the topics and the whole question bank with a new set, **before the first practice session only**.

- Allowed when the requirements are confirmed and the profile has **no interview at all** (`practice_started` is
  `false`; a finished-without-score or abandoned session counts as started).
- The Roadmap id stays the same. The topics get **new ids** (drop any topic id you stored). The topics and the
  bank are replaced in one transaction, and the swap is excluded from starting a practice session, so a session
  can never reference a topic that was just replaced. If a practice start wins the race, regenerate answers
  `409 ROADMAP_REGENERATE_NOT_ALLOWED` and nothing changes.
- On failure the previous set stays published and the status returns to `ready` (from a `failed` or `pending`
  Roadmap it stays `failed`). The Candidate can try again.
- The UI must warn that the unused plan and questions will be replaced.

### Errors of 9.1 to 9.3

| HTTP | `error.code` | When | What the client should do |
| --- | --- | --- | --- |
| 400 | `VALIDATION_ERROR` | Malformed id. | A client bug. |
| 401 | `INVALID_TOKEN` | No valid session. | Send the user to sign in. |
| 403 | `FORBIDDEN` | Not a Candidate, or onboarding incomplete. | Hide the action. |
| 404 | `JD_NOT_FOUND` | Unknown or foreign Role Profile. | Go back to the list. |
| 409 | `REQUIREMENTS_NOT_CONFIRMED` | Retry or regenerate on a profile whose requirements are not confirmed. | Send the Candidate to confirm the requirements first. |
| 409 | `ROADMAP_GENERATING` | Another request is generating this Roadmap (a generation older than 5 minutes is taken over instead). | Wait and poll the Roadmap read (section 5); do not send another request. |
| 409 | `ROADMAP_NOT_RETRYABLE` | Retry: the profile already has a practice interview (including a legacy profile from before the Roadmap), so its bank cannot be replaced. | Hide retry; suggest creating a new Role Profile. |
| 409 | `ROADMAP_REGENERATE_NOT_ALLOWED` | Regenerate: the profile already has an interview, or a start won the race. | Hide regenerate for good (`practice_started` is `true`); explain that the plan is kept so history keeps its meaning. |
| 429 | `RATE_LIMITED` | Too many generation requests. | Wait and let the user retry. |
| 409 | `LLM_BUDGET_EXCEEDED` | The model budget is spent (each generation request charges one unit). | Tell the user to try later. (Documented by the generation lane as 409; check swagger for the final status.) |
| 502 | `ROADMAP_GENERATION_FAILED`, `INVALID_ROADMAP_GENERATION_OUTPUT` | The topic step failed or returned an unusable result. | Show "could not prepare your Roadmap" with a retry button (retry, or confirm again on a profile still in `review`). |
| 502 | `QUESTION_GENERATION_FAILED`, `INVALID_QUESTION_GENERATION_OUTPUT` | The question bank step failed or returned an unusable result. | The same. |
| 500 | `INTERNAL_ERROR` | Unexpected failure. | The same, later. |

A failed generation never leaves a half-published Roadmap: the previous state is kept, and `GET .../roadmap`
shows `failed` where a retry applies.

### 9.4 Practice start with `focus_item_id`

The existing `POST /interviews` (practice) gains an optional, nullable `focus_item_id`: a topic `id` taken from
`GET .../roadmap`. `null` or absent is general practice. Recruitment interviews never accept it.

- **Request:** `{ "role_profile_id": "...", "type": "practice", "focus_item_id": "<topic id> | null", ... }` plus the
  usual `Idempotency-Key` header.
- **Response:** the session now carries `focus_item_id` (the focus the server actually stored: `null` = general) and,
  on the start answer only, `resumed` (`true` = the session already existed, `false` = created now). Session history
  items also carry `focus_item_id`. **The UI must show the focus the server reports, never the one it asked for.**
- **Status:** `201` = created, `200` = an existing session came back (`resumed: true`).
- **Same key, same intent:** the first session again (`200`, `resumed: true`), one charge in total.
- **Same key, different focus** (a topic vs general, or two topics): `409 INTERVIEW_START_INTENT_CONFLICT`, nothing is created.
- **An unfinished practice session of the profile** (no key, or another key): that session comes back (`200`,
  `resumed: true`) with ITS focus, whatever focus was asked. No charge, no change. Compare the returned
  `focus_item_id` with what the user asked for and explain, for example "you have an unfinished general practice, continue or end it first".
- **Sampling:** general practice draws from the whole bank, preferring unevaluated questions and topics not yet
  achieved (it still covers every Barem metric as far as the question limit allows). Focused practice draws only from that topic, never
  from another one.
- **Top-up:** a focused start of a topic that has fewer bank questions than the question limit makes one model call
  to add questions before the charge. This can take up to about two minutes: give this request a client timeout of at
  least 120 s (200 s is safe). If it fails, no session and no charge. The top-up runs **before** the wallet is
  checked: a Candidate without enough coins can still trigger one top-up and then gets `INSUFFICIENT_BALANCE`. The
  added questions stay in the bank and are valid, so this costs one model call, never data. Check the balance in
  the UI before offering a focused start of a short topic if that cost matters.

| Status | Code | When | Client behavior |
| --- | --- | --- | --- |
| 400 | `VALIDATION_ERROR` | Focus on a recruitment interview, or the topic is not of this profile (checked even when a session is open). | Programming error: do not offer it. |
| 404 | `JD_NOT_FOUND` | The profile is not the Candidate's. | Back to the list. |
| 409 | `ROADMAP_NOT_READY` | The Roadmap is `pending`, `generating` or `failed`. | Send the user to the Roadmap screen (poll or retry, section 5). |
| 409 | `QUESTION_BANK_NOT_READY` | The topic still has fewer questions than the limit after the top-up attempt (never borrowed from another topic). | Offer general practice or another topic, try later. |
| 409 | `INTERVIEW_START_INTENT_CONFLICT` | Same idempotency key, different focus. | Use a new key for a new intent. |
| 409 | `ROLE_PROFILE_REQUIRED` | The profile is missing or not confirmed. | As today. |
| 402/409 | `INSUFFICIENT_BALANCE` | Not enough coins (5 per practice). | As today. |
| 429 | `LLM_BUDGET_EXCEEDED` | The model budget is spent (the top-up charges one unit). | Try later. |
| 502 | `QUESTION_GENERATION_FAILED`, `INVALID_QUESTION_GENERATION_OUTPUT` | The top-up failed. | Try again; nothing was charged. |

A general start (`focus_item_id` null) on a profile that has no Roadmap (a legacy profile) is unchanged and still starts.

## 10. What the API never returns

The Roadmap and history responses contain no expected answers, rubric names or criteria, feedback text,
transcripts, audio or recording links, bank questions that were not part of a session, internal scoring metadata,
or anything from a recruitment interview. `question_text` is only the text of a question the Candidate answered,
exactly as the practice report already shows it after evaluation. The Candidate cannot send a score, a status or
a question-to-topic mapping; the server decides all of them.
