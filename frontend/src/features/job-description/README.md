# Shared JD preparation

Candidate Role Profiles and recruiter-owned Job Postings share text/PDF entry, extraction review, requirement editing and original-document presentation. Their API functions, per-user cache keys and routes are supplied through the public `JDWorkflow` interface. Workflow configuration is composed inside client components; App Router pages remain server components.

Text validation counts Unicode code points after whitespace normalization (100–20,000). PDFs must be selectable-text documents up to 5 MiB. A separate Axios transport uploads to signed storage URLs without API cookies or interceptors. Retrying analysis reuses the draft; retrying PDF completion reuses its upload ticket. The backend makes completion idempotent for the same upload and owner.

Confirmation saves the reviewed requirements and refinement note first, then waits for the backend to generate and persist interview content. Candidates receive no question bank. Recruiters proceed to the bank for their own posting. These are independent owned JDs; candidate practice data is never transferred to recruiters.
