# Candidate Role Profiles

Candidates can create a Role Profile from pasted job-description text or a selectable-text PDF up to 5 MiB. Pasted text is normalized and validated at 100–20,000 characters. PDF uploads use an API-issued ticket, a direct `PUT` to the signed storage URL, and an API completion request that creates the draft. DOCX, scanned, and encrypted PDFs are not supported. The storage `PUT` uses a separate Axios request without API cookies or interceptors. Retrying analysis reuses the saved draft, and retrying PDF completion reuses the upload ticket; the backend makes completion idempotent for the same upload and owner.

The candidate reviews the extracted title, seniority, and requirements, including proficiency targets, classification, priority, and source evidence. A refinement note is saved with the review. Saving calls `PUT /role-profiles/{id}`. Confirmation saves the review first, then calls `POST /role-profiles/{id}/confirm` and waits for that request to finish before showing the confirmed state.

The backend generates and stores the internal Blueprint and core questions during confirmation. The candidate UI does not call a separate question-generation endpoint and does not show or ask the candidate to approve questions. A failed analysis leaves the draft available to retry or reopen. A confirmed profile continues to the interview setup wizard (`/interviews/new/*`): the Role Profile picker, then interviewer and voice choice, then the microphone preflight that starts the practice interview. Setup only opens for a confirmed profile.

The E2E coverage mocks the API and signed-storage request. It checks the frontend request order and visible states; it does not exercise the database, LLM, storage provider, or generated question bank.

## Routes

| Route                             | Screen                                                                                                                                                               |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/role-profiles`                  | Library table of the candidate's Role Profiles (long titles wrap to two lines; the full title is on the link's `title`).                                             |
| `/role-profiles/new`              | Entry: paste text or upload a PDF.                                                                                                                                   |
| `/role-profiles/[id]`             | Review, confirm, or read the confirmed profile.                                                                                                                      |
| `/interviews/new/job-description` | The same entry screen inside the interview wizard (step 1).                                                                                                          |
| `/interviews/new/skills`          | Wizard step 2 ("Requirements"). It has no screen of its own: it redirects to `/role-profiles/[id]` when `roleProfileId` is given, otherwise to `/role-profiles/new`. |

The old `/target-jds` route no longer exists; "Target JD" was the previous name of a Role Profile.

## Code layout

- `schemas/`: zod schemas and the types inferred from them.
  - `role-profile-schema.ts`: enums, the stored requirement and Role Profile shapes, list/detail envelopes, `RoleProfile`, `RoleSummary`, `RoleRequirement`.
  - `role-review-schema.ts` and `role-entry-schema.ts`: form schemas built by `createReviewSchema(v)` / `createEntrySchema(v)`. `v` is the translator for `candidate.roleProfiles.validation` (`RoleValidation`, in `role-validation.ts`), so field errors are already in the active language. The un-parameterised `reviewSchema` / `entrySchema` are request guards whose messages are never shown.
  - `upload-schema.ts`: PDF size limit, the signed storage request and the upload ticket envelope.
- `api/requests.ts`: the HTTP calls, the page size, and `roleProfileKeys` (React Query keys, scoped by user id).
- `lib/`: pure helpers (`role-text.ts` text normalisation, `pdf-file.ts` client-side PDF checks, `targets.ts` proficiency targets, `labels.ts` and `role-error.ts` display and error mapping).
- `components/`: screens and their parts.
