# Privacy controls: retention, access audit, recording consent

Tickets KAN-281, KAN-296, KAN-297. Migrations 000029 (`recruiter_access_audit`) and 000030 (consent columns).

## Retention (KAN-281)

Off by default. The System Handler runs two extra jobs when `RETENTION_ENABLED=true`, paced by
`JOBS_RECORDING_RETENTION_INTERVAL` and switched off with `JOBS_ENABLED=false`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `RETENTION_ENABLED` | `false` | Turns both passes on. |
| `RETENTION_CV_DAYS` | `180` | CV files of Applications rejected at the CV stage (rejected, no interview ever started) are deleted from storage and `applications.cv_path` is cleared this many days after the decision. |
| `RETENTION_TRANSCRIPT_DAYS` | `365` | Answers and per-turn feedback of interviews completed this many days ago are cleared. Questions, positions and every score stay. TODO KAN-338: the evidence quotes and feedback of rubric criterion results (`interview_criterion_scores`) are not cleared by this pass yet. |

Recording retention (90 days after the final decision) is unchanged and not configurable.

Both passes are idempotent and take at most 100 items per run. The object is deleted first and the database pointer
is cleared only afterwards, so a storage failure keeps the pointer and the next run retries (a missing object counts as
deleted). Logs carry counts only, never keys or ids.

Decision time is `applications.decided_at`, set once when the Application is decided (migration 000028).

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

## Learning Roadmap data (KAN-356)

The Roadmap adds three tables (`roadmaps`, `roadmap_items`, `roadmap_assessments`) and two columns
(`core_questions.roadmap_item_id`, `interviews.focus_item_id`). None of them holds a score, a transcript, an answer or
an expected answer: a Roadmap row is structure, and an assessment only names the topic that receives one answered
practice question. Scores are read from `interview_questions.score` on every read, so the privacy rules of the
interview data apply unchanged.

| Control | Behaviour |
| --- | --- |
| Account erasure | `DELETE /account` removes the links first (`roadmap_assessments` by the account's interview ids), then the interviews, then the account's Roadmaps (their topics go with them), then the Role Profiles and Job Descriptions. Bank questions that pointed at a topic lose the pointer and are deleted with their Job Description. The restricting foreign keys (a topic with links or a focused interview) never stop an erasure, because the rows that reference a topic are deleted before it. |
| Deleting a Role Profile | Refused (`JD_IN_USE`) while the profile has any interview, as before. A profile without interviews is deleted with its Roadmap and topics. |
| Data export (`GET /account/export`) | Each Role Profile in `role_profiles` carries `roadmap`: `generation_status`, `topics` (id, title, description, position) and `results` (topic id, interview id, question number, the question score, link time). Only the owner's own practice interviews are linked: a recruitment interview, another account's data, question text, answers, expected answers and rubrics are never part of it. The transcripts and criterion results stay in `interviews`, so nothing is exported twice. |
| Transcript retention (`RETENTION_ENABLED`) | The pass clears the answer and per-turn feedback text and keeps the turns, the questions and every score. The Roadmap is unaffected: "was this question answered" is read from the existence of the turn, never from its text, so an answer whose text expired is never turned into a skipped question worth 0, and no expired text is needed to compute a score. |
| Logs | The assessment writer and the catch-up job log interview ids, counts and error classes only. The error a catch-up run returns says how many sessions failed and nothing else. |

Injection-flagged practice evaluations (`interviews.injection_flagged`) never feed a Roadmap: the evaluation could
not trust the answers, so they are not evidence (Learning Roadmap spec section 8).

