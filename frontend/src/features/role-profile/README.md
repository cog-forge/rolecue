# Candidate Role Profiles

Candidates can create a Role Profile from pasted job-description text or a selectable-text PDF up to 5 MiB. Pasted text is normalized and validated at 100–20,000 characters. PDF uploads use an API-issued ticket, a direct `PUT` to the signed storage URL, and an API completion request that creates the draft. DOCX, scanned, and encrypted PDFs are not supported.

The candidate reviews the extracted title, seniority, and requirements, including proficiency targets, classification, priority, and source evidence. A refinement note is saved with the review. Saving calls `PUT /role-profiles/{id}`. Confirmation saves the review first, then calls `POST /role-profiles/{id}/confirm` and waits for that request to finish before showing the confirmed state.

The backend generates and stores the internal Blueprint and core questions during confirmation. The candidate UI does not call a separate question-generation endpoint and does not show or ask the candidate to approve questions. A failed analysis leaves the draft available to retry or reopen. A confirmed profile can continue to interview setup; the setup page currently validates its confirmed status and then shows a placeholder for the remaining setup flow.

The E2E coverage mocks the API and signed-storage request. It checks the frontend request order and visible states; it does not exercise the database, LLM, storage provider, or generated question bank.
