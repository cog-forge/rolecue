# Interview Setup API contract (KAN-231)

The persisted pre-interview setup: the interviewer **avatar**, the **voice** and the **environment** (room) a
Candidate chose before an interview, and a live **readiness** that says whether those choices can start.
Together with Swagger (`/swagger`, tag `interview-setups`) it is everything a client needs.

Difficulty and duration are **not** part of the setup: difficulty comes from the Job Description and the bank, and
an interview ends when every core question is answered.

## 1. Basics

- A setup is a draft the Candidate comes back to. It is **not** an interview. Starting still goes through
  `POST /interviews` with its own request; that endpoint re-checks the saved setup before a new session starts (section 6) but never writes it. A setup call **never**
  debits practice coins, uses a recruitment slot, creates an `interviews` row, or returns core questions, the bank
  size or evaluation weights.
- Envelope, session cookie and `snake_case` are the same as every other endpoint. Branch on `error.code`.
- **Candidate only** (`403 FORBIDDEN` for any other role, `401` without a session, `403` before onboarding is done).
  Responses carry `Cache-Control: no-store`.
- Every `GET` recomputes the readiness from the data as it is now and **writes nothing**. Every `PUT` answers with
  the same revalidated view.

| Method | Path | Purpose |
|---|---|---|
| GET | `/interview-setups/practice/{roleProfileId}` | Read and revalidate the practice setup |
| PUT | `/interview-setups/practice/{roleProfileId}` | Save the practice setup (avatar, voice, environment) |
| GET | `/interview-setups/recruitment/{applicationId}` | Read and revalidate the recruitment setup |
| PUT | `/interview-setups/recruitment/{applicationId}` | Save the Candidate's environment (the only editable field) |

## 2. The view (GET and PUT answer)

```json
{
  "mode": "practice",
  "role_profile_id": "…",
  "target_title": "Backend Engineer",
  "avatar": { "id": "…", "name": "Minh" },
  "voice": { "id": "…", "name": "Lan" },
  "environment": "office-3d",
  "locked_fields": [],
  "saved": true,
  "updated_at": "2026-11-18T08:00:00Z",
  "ready": false,
  "blockers": [{ "code": "voice_unavailable", "field": "voice" }]
}
```

- `mode` is `practice` or `recruitment`. `role_profile_id` (practice) or `application_id` (recruitment) is set.
- `target_title` is the Role Profile title, or the Job Posting title for recruitment.
- `avatar`, `voice`, `environment` are `null` when not chosen yet. `saved` is false until the first save.
- **Recruitment:** `avatar` and `voice` are the ones the **Recruiter locked** on the posting, read-only
  (`locked_fields: ["avatar","voice"]`): only `id` and `name`, never a model or a grant (the preflight model comes from
  `GET /applications/{id}/interviewer`). `interview_deadline` is returned.
- `ready` is true **exactly when `blockers` is empty**. `blockers[].field` is `avatar`, `voice` or `environment` when
  the blocker is about one of them.
- A stored choice that became stale is **reported and kept**, never silently cleared.

## 3. Environments

One of: `cafe-3d`, `office-3d`, `meeting-3d`, `office`, `balcony`, `cafe` (the room picker's ids, KAN-275). The
frontend file `src/features/interview/components/room-backgrounds.ts` is the single source; a backend test fails when
the two lists differ. Anything else is `400 VALIDATION_ERROR` on save and `environment_unsupported` on read.

## 4. Practice

Scoped to a Role Profile the Candidate owns that is **confirmed** and whose Job Description has a ready hidden bank.

`PUT` body (strict JSON: an unknown field is `400 VALIDATION_ERROR`):

```json
{ "avatar_id": "uuid|null", "voice_id": "uuid|null", "environment": "string|null" }
```

A **full replacement**: a missing or `null` field means "not chosen yet", so a partial draft can be saved. Repeating
the same body changes nothing, not even `updated_at`.

| Refusal (nothing is saved) | Status | Code |
|---|---|---|
| Role Profile foreign or missing | 404 | `JD_NOT_FOUND` |
| Role Profile not confirmed | 409 | `ROLE_PROFILE_REQUIRED` |
| No ready bank | 409 | `QUESTION_BANK_NOT_READY` |
| Avatar not in the Candidate's own inventory, deleted, or its model blocked | 404 | `AVATAR_NOT_FOUND` |
| Voice unknown or inactive | 404 | `VOICE_NOT_FOUND` |
| Environment outside the catalog, unknown JSON field, malformed UUID | 400 | `VALIDATION_ERROR` |
| Not a Candidate | 403 | `FORBIDDEN` |

`GET` blockers: `role_profile_not_confirmed`, `question_bank_not_ready` (only reported for a confirmed profile),
`avatar_required`, `avatar_unavailable`, `voice_required`, `voice_unavailable`, `environment_required`,
`environment_unsupported`.

A deleted avatar or removed voice is cleared by the database (`ON DELETE SET NULL`), so it shows as `avatar_required`
or `voice_required`; a voice deactivated by an admin or an avatar that is no longer usable shows as `*_unavailable`.

## 5. Recruitment

Linked to an Application the Candidate owns that the Recruiter approved (`interview_eligible`), before its 24-hour
deadline, on a funded `open` or `intake_closed` posting.

`PUT` body: `{ "environment": "string|null" }`. The locked avatar and voice **cannot be sent** (`400 VALIDATION_ERROR`).

| Refusal (nothing is saved) | Status | Code |
|---|---|---|
| Application foreign or missing | 404 | `APPLICATION_NOT_FOUND` |
| CV not approved, deadline passed, posting not accepting, or interview already started | 409 | `APPLICATION_NOT_ELIGIBLE` |
| Environment outside the catalog, unknown JSON field | 400 | `VALIDATION_ERROR` |
| Not a Candidate | 403 | `FORBIDDEN` |

`GET` blockers: `application_not_eligible`, `interview_deadline_passed`, `interview_already_started`,
`locked_avatar_unavailable` (no avatar locked, or it is no longer in the Recruiter's inventory, or its model is
blocked), `locked_voice_unavailable`, `environment_required`, `environment_unsupported`. Interview capacity is **not**
a blocker: the slot is consumed by the first start, which checks it.

## 6. The start flow (`POST /interviews`)

A **new** session starts only from a saved, ready setup. `POST /interviews` calls `Service.Practice` (a practice start,
by `role_profile_id`) or `Service.Recruitment` (a recruitment start, by `application_id`) and starts only when
`ready == (no blockers)`, computed from live data. The check runs before anything is charged or taken: a refusal debits
no coins, uses no recruitment slot, writes no interview row and gives the AI budget reservation back.

**What is never checked** (the start only returns a session that already exists, so a setup that went stale or was never
saved cannot block it): resuming the open practice session of the Role Profile, a retry with the same `Idempotency-Key`,
and the Application's existing recruitment session (even though its setup then reports `interview_already_started`).
`POST /interviews/{id}/resume` and the live socket do not read the setup at all. A focused practice start
(`focus_item_id`) is checked like any new practice start, after the topic is validated and before any top-up call.

**Refusal.** A setup that is not saved or not ready answers `409 SETUP_NOT_READY` with the blockers of section 4/5 in
`error.details`, the same `{code, field}` objects `GET` returns:

```json
{"success": false, "error": {"code": "SETUP_NOT_READY", "message": "The interview setup is not ready to start.",
  "details": {"blockers": [{"code": "avatar_required", "field": "avatar"}, {"code": "environment_required", "field": "environment"}]}}}
```

Blockers the start flow always reported with a code of its own keep that code (and no `details`): an unconfirmed Role
Profile is `ROLE_PROFILE_REQUIRED`, a profile without a ready bank `QUESTION_BANK_NOT_READY`, an Application that is not
`interview_eligible`, whose posting stopped accepting or whose deadline passed `APPLICATION_NOT_ELIGIBLE`. Every other
blocker (avatar, voice, environment, locked avatar/voice) is `SETUP_NOT_READY`. A foreign or missing Role Profile or
Application is still `JD_NOT_FOUND` / `APPLICATION_NOT_FOUND`.

**Avatar and voice of a practice start.** The session uses the saved avatar and voice. `avatar_id` and `voice_id` in
the request body are optional:

- omitted: the saved value is used;
- present and equal to the saved value: accepted (the current client repeats them);
- present and different: refused with `400 VALIDATION_ERROR`, nothing charged. The start never silently replaces what the
  Candidate asked for; to change the interviewer, save the setup first.

A recruitment start takes the posting's avatar and voice and still refuses a body that names its own (`VALIDATION_ERROR`).
Starts that return an existing session ignore the body, as before.

The capacity of a recruitment posting is still checked by the start itself (`INTERVIEW_CAPACITY_NOT_FUNDED`,
`INTERVIEW_CAPACITY_FULL`), under the posting lock.

## 7. Storage

Table `interview_setups` (migration `000037`): `type`, `role_profile_id` or `application_id` (unique, `ON DELETE
CASCADE`), `avatar_id` and `voice_id` (`ON DELETE SET NULL`), `environment`, timestamps. A recruitment row never holds
an avatar or voice (CHECK). Erasing an account removes its setups through the cascade.
