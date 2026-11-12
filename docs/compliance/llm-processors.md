# LLM and speech processors (KAN-298)

> **DRAFT - Pending product-owner approval.** This is an engineering inventory of what the code sends to
> third-party processors. It is not legal advice and it states no legal conclusion. Items marked TODO are
> unknown to engineering and need the product owner, legal or the vendor contract. Do not publish or quote
> this file externally until it is approved.

Source of truth: the Go API under `api/`. Verified from code on the date of the commit that added this file;
re-check when providers, routing or prompts change.

## 1. Processors found in the code

| Processor | Where it is wired | How it is selected | Verified from code |
| --- | --- | --- | --- |
| Google Gemini (Gemini API) | `api/internal/pkg/ai/model.go` (eino Gemini chat model) | `llm.provider: gemini` in `api/configs/config.yaml`; key in `LLM_API_KEY`; model `llm.model` (default `gemini-3.1-flash-lite`) | Yes |
| Groq | `api/internal/pkg/ai/model.go` and `groq.go`, endpoint `https://api.groq.com/openai/v1/chat/completions` | Opt-in per task through `ai.<task>.provider: groq` (`ai.jd_extraction`, `ai.question_bank`, `ai.follow_up`, `ai.evaluation`); key in `GROQ_API_KEY` | Yes |
| Microsoft Azure AI Speech (text to speech) | `api/internal/pkg/tts/azure/azure.go` (REST synthesis) and `api/internal/features/admin/voicecatalog/provider/azure.go` (voice list) | `tts.key`, `tts.region` (env `AZURE_SPEECH_KEY`, `AZURE_SPEECH_REGION`) | Yes |
| Azure speech to text | None. `interview.stt_provider` accepts only `fake` (`api/internal/config/config.go`); no real transcriber is wired | n/a | Yes: not used |

Other third parties in the product that are not LLM or speech processors (out of scope here, listed so the
register is complete): Avaturn (avatar creation, camera access), PayOS (payments), Resend (e-mail), object
storage (`storage.*`). TODO product owner: confirm whether they belong in the same register.

Not verified from code: whether the frontend sends any audio or text to a speech service from the browser
(for example the browser speech API or a viseme service). `frontend/src/components/avatar-viewer/lipsync/azure.ts`
maps Azure viseme output to blend shapes; engineering has not traced where that data originates. TODO.

## 2. Data categories sent, and why

| Flow (code path) | Processor | Data sent in the prompt | Purpose | Minimisation in code |
| --- | --- | --- | --- | --- |
| JD extraction (`roleprofile/provider/eino.go`) | Gemini or Groq | The job description text a candidate pasted or uploaded (may contain recruiter or company contact details) | Extract title, seniority, skills | JD is cleaned (NFKC, invisible characters) and fenced as untrusted data. Contact details are not stripped, because the model must quote the JD verbatim. TODO product owner: accept, or add redaction with a placeholder-aware evidence check |
| Question bank (`questionbank/provider/eino.go`) | Gemini or Groq | Seniority level, confirmed requirement names and targets, JD excerpts (`evidence_text`), the owner's refinement note, earlier generated questions | Write core interview questions | Free text is cleaned, capped and fenced |
| Follow-up decision (`interview/provider/eino.go`) | Gemini or Groq (`ai.follow_up`) | Core question, the candidate's spoken answers as text and earlier questions, requirement names | Decide and word one follow-up | Answers go through `textsafe.Redact` (e-mail, phone, URL) |
| Rubric grading (`evaluation/provider/grader.go`, KAN-334) | Gemini or Groq (`ai.evaluation`) | One core question, its follow-ups and the candidate's answers to them as text; the question's rubric (name, description, criteria, metric names, maximum points, scoring guides, which a Recruiter may have written) and its expected answer | Grade each rubric criterion and write feedback | Answers and questions go through `textsafe.Redact`; every value is cleaned, capped and fenced as untrusted data; one call per core question, so a call sees only that question's answers and rubric. `QuestionGradeInput.RedactNames` can carry the candidate's own names; TODO engineering: the evaluation service does not populate it yet |
| Text to speech (`tts/azure`) | Azure AI Speech | The text of the interviewer's questions (core questions and follow-ups) and the voice id | Speak the question to the candidate | Questions only; no candidate answers are sent |
| Voice catalog (`voicecatalog/provider/azure.go`) | Azure AI Speech | Admin request for the voice list; no candidate data | Keep the voice catalog current | None needed |

Identifiers that the prompts do not need and that are removed before sending, when present in answer text: e-mail
addresses, phone numbers (9 to 15 digits), URLs and the names given to `Redact`. The user id, e-mail address and
profile fields are not part of any prompt payload in the code reviewed.

Not found in code (TODO engineering to re-confirm before approval): any CV or resume text sent to an LLM. The
current flows send JD text, question text and answer text only.

## 3. Region and transfer notes

All entries are unknown to engineering. Do not fill them from assumptions.

| Processor | Processing region | Cross-border transfer mechanism | Retention / training use | Contract status |
| --- | --- | --- | --- | --- |
| Google Gemini API | TODO | TODO | TODO (paid tier versus free tier terms differ; confirm which applies to the key in use) | TODO |
| Groq | TODO | TODO | TODO | TODO |
| Azure AI Speech | TODO: the region is the value of `AZURE_SPEECH_REGION` in each environment | TODO | TODO | TODO |

## 4. DPA and SCC checklist

Each item is open until the product owner or legal marks it done. Engineering cannot tick them.

- [ ] Role of RoleCue and of each processor under the applicable data protection law (controller or processor): TODO legal.
- [ ] Data processing agreement signed or accepted online with Google (Gemini API).
- [ ] Data processing agreement signed or accepted with Groq.
- [ ] Data processing agreement signed or accepted with Microsoft (Azure AI Speech).
- [ ] For each processor outside the data subjects' jurisdiction: transfer mechanism recorded (for example standard
      contractual clauses or an adequacy decision) and the transfer impact reviewed, where the law requires it.
- [ ] Each processor's sub-processor list reviewed and a notification route for changes recorded.
- [ ] Confirmed that prompts and outputs are not used to train the vendor's models (per vendor terms and plan).
- [ ] Retention period of prompts and outputs at each vendor recorded; abuse-monitoring retention noted.
- [ ] Privacy notice to candidates and recruiters lists these processors and the purposes (frontend copy is not part of this ticket).
- [ ] Lawful basis and the candidate-facing explanation for scoring by an automated system reviewed by legal,
      including any right to human review.
- [ ] Data subject request route (access, deletion) covers data held by the processors.
- [ ] Records of processing updated with this inventory.
- [ ] Decision on which regions or endpoints to pin per processor, and whether `config.yaml` or env must change to do it.
- [ ] Re-review trigger agreed: any new provider, new task routed to a provider, or new field added to a prompt.

## 5. Change log

- Draft created with KAN-298 (code part). Pending product-owner approval.
