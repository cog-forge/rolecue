# Privacy controls: retention, access audit, recording consent

Tickets KAN-281, KAN-296, KAN-297. Migrations 000029 (`recruiter_access_audit`) and 000030 (consent columns).

## Retention (KAN-281)

Off by default. The System Handler runs two extra jobs when `RETENTION_ENABLED=true`, paced by
`JOBS_RECORDING_RETENTION_INTERVAL` and switched off with `JOBS_ENABLED=false`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `RETENTION_ENABLED` | `false` | Turns both passes on. |
| `RETENTION_CV_DAYS` | `180` | CV files of Applications rejected at the CV stage (rejected, no interview ever started) are deleted from storage and `applications.cv_path` is cleared this many days after the decision. |
| `RETENTION_TRANSCRIPT_DAYS` | `365` | Answers and per-turn feedback of interviews completed this many days ago are cleared. Questions, positions and every score stay. |

Recording retention (90 days after the final decision) is unchanged and not configurable.

Both passes are idempotent and take at most 100 items per run. The object is deleted first and the database pointer
is cleared only afterwards, so a storage failure keeps the pointer and the next run retries (a missing object counts as
deleted). Logs carry counts only, never keys or ids.

Decision time is `applications.updated_at` for now (a rejected Application is never updated again, and clearing the
CV does not touch it). TODO: switch the helper query `ListExpiredCVs` to `applications.decided_at` once migration
000028 lands.

## Recruiter access audit (KAN-296)

Every successful Recruiter read that returns a recording link or a transcript writes one `recruiter_access_audit` row
(`recording_view` or `transcript_view`) before the data leaves the service. If the row cannot be written the read fails
and nothing is returned. Refused reads and recording reads without a link (not ready) write no row.

`GET /admin/access-audit?recruiter_id=&application_id=&page=&page_size=` (Admin only) lists the rows newest first.
The page is `/admin/access-audit`.

## Recording consent (KAN-297)

The preflight of a recruitment interview shows the versioned consent text (`CONSENT_VERSION = 2026-11-v1`) with a
required checkbox. Start stays disabled until it is ticked: a recruitment interview is always recorded, so declining
means the interview does not start and nothing is recorded. The tick is remembered per account in the browser and the
version is sent as `consent_version` in `POST /interviews/{id}/recording`; the API stores the version and the time
(`interviews.recording_consent_version`, `recording_consented_at`) with the upload grant.

`RECORDING_CONSENT_REQUIRED=false` (default): a grant without a consent version still works. When `true`, a grant with
no consent sent and none stored is refused with `RECORDING_CONSENT_REQUIRED` (HTTP 409) and nothing is written.
