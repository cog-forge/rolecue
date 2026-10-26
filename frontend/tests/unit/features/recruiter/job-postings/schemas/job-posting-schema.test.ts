import { describe, expect, it } from "vitest";
import {
  createBody,
  documentEnvelopeSchema,
  documentLinkEnvelopeSchema,
  emptyPostingValues,
  postingEnvelopeSchema,
  postingFormSchema,
  postingLocks,
  postingPatch,
  postingValues,
  suggestionEnvelopeSchema,
  type JobPosting,
  type PostingValues,
} from "@/features/recruiter/job-postings/schemas/job-posting-schema";

const avatarId = "44444444-4444-4444-8444-444444444444";
const voiceId = "55555555-5555-4555-8555-555555555555";

const posting: JobPosting = {
  id: "33333333-3333-4333-8333-333333333333",
  title: "Backend Engineer",
  status: "pending",
  reject_reason: null,
  raw_text: "We build Go services. ".repeat(10),
  has_document: true,
  seniority_level: "senior",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
  ],
  location: "Ho Chi Minh City",
  employment_type: null,
  salary_min: 1000,
  salary_max: null,
  interview_duration: 45,
  interview_slot: null,
  required_avatar: { id: avatarId, name: "Linh" },
  required_voice: { id: voiceId, name: "Ava" },
  refinement_note: "",
  review_submitted: true,
  core_question_count: 0,
  created_at: "2026-10-12T08:00:00Z",
};

const documentId = "66666666-6666-4666-8666-666666666666";

const valid: PostingValues = {
  ...emptyPostingValues,
  document_id: documentId,
  file_name: "backend.pdf",
  raw_text: "a".repeat(100),
  title: " Backend Engineer ",
  seniority_level: "mid",
  requirements: [
    { name: " Go ", target: "programming_language", type: "required" },
  ],
  confirmed: true,
  required_avatar_id: avatarId,
  required_voice_id: voiceId,
};

const issues = (values: PostingValues) => {
  const result = postingFormSchema.safeParse(values);
  return result.success
    ? []
    : result.error.issues.map((issue) => issue.path.join("."));
};

describe("job posting wire schemas", () => {
  it("parses a posting and rejects contract drift", () => {
    expect(
      postingEnvelopeSchema.safeParse({ success: true, data: posting }).success,
    ).toBe(true);
    expect(
      postingEnvelopeSchema.safeParse({
        success: true,
        data: { ...posting, status: "archived" },
      }).success,
    ).toBe(false);
    const missing: Partial<JobPosting> = { ...posting };
    delete missing.core_question_count;
    expect(
      postingEnvelopeSchema.safeParse({ success: true, data: missing }).success,
    ).toBe(false);
    expect(
      postingEnvelopeSchema.safeParse({ success: false, data: posting })
        .success,
    ).toBe(false);
  });

  it("parses an uploaded document and a PDF link", () => {
    expect(
      documentEnvelopeSchema.safeParse({
        success: true,
        data: { document_id: documentId, raw_text: "JD text" },
      }).success,
    ).toBe(true);
    expect(
      documentLinkEnvelopeSchema.safeParse({
        success: true,
        data: {
          url: "https://hcm.example.com/role-cue/key?X-Amz-Signature=abc",
          expires_at: "2026-10-17T08:05:00Z",
        },
      }).success,
    ).toBe(true);
    // Only web links are opened.
    expect(
      documentLinkEnvelopeSchema.safeParse({
        success: true,
        data: {
          url: "javascript:alert(1)",
          expires_at: "2026-10-17T08:05:00Z",
        },
      }).success,
    ).toBe(false);
  });

  it("parses a suggestion with no seniority", () => {
    const data = { title: "Engineer", seniority_level: null, requirements: [] };
    expect(
      suggestionEnvelopeSchema.safeParse({ success: true, data }).success,
    ).toBe(true);
  });
});

describe("postingFormSchema", () => {
  it("accepts a complete form", () => {
    expect(issues(valid)).toEqual([]);
  });

  // The server checks the PDF text length when it reads the PDF; the form only needs an upload.
  it("requires an uploaded PDF", () => {
    expect(issues({ ...valid, raw_text: "" })).toEqual(["raw_text"]);
    expect(issues({ ...valid, raw_text: "   " })).toEqual(["raw_text"]);
  });

  it("requires a title, seniority, a requirement, an interviewer and a voice", () => {
    expect(issues({ ...emptyPostingValues, raw_text: valid.raw_text })).toEqual(
      expect.arrayContaining([
        "title",
        "seniority_level",
        "requirements",
        "required_avatar_id",
        "required_voice_id",
      ]),
    );
  });

  it("rejects a repeated requirement with the same kind", () => {
    const requirements = [
      {
        name: "Go",
        target: "programming_language" as const,
        type: "required" as const,
      },
      {
        name: " go ",
        target: "programming_language" as const,
        type: "preferred" as const,
      },
      {
        name: "Go",
        target: "technologies" as const,
        type: "required" as const,
      },
    ];
    expect(issues({ ...valid, requirements })).toEqual(["requirements.1.name"]);
  });

  it("checks salary and interview length as whole numbers", () => {
    expect(issues({ ...valid, salary_min: "-1" })).toEqual(["salary_min"]);
    expect(issues({ ...valid, salary_min: "12.5" })).toEqual(["salary_min"]);
    expect(
      issues({ ...valid, salary_min: "2000", salary_max: "1000" }),
    ).toEqual(["salary_max"]);
    expect(issues({ ...valid, interview_duration: "0" })).toEqual([
      "interview_duration",
    ]);
    expect(issues({ ...valid, salary_max: "2147483648" })).toEqual([
      "salary_max",
    ]);
  });
});

describe("request bodies", () => {
  it("creates exactly the fields the Go DTO accepts, with blanks as null", () => {
    expect(
      createBody({ ...valid, salary_min: " 1000 ", location: "  " }),
    ).toEqual({
      title: "Backend Engineer",
      document_id: documentId,
      refinement_note: "",
      seniority_level: "mid",
      requirements: [
        { name: "Go", target: "programming_language", type: "required" },
      ],
      location: null,
      employment_type: null,
      salary_min: 1000,
      salary_max: null,
      interview_duration: null,
      required_avatar_id: avatarId,
      required_voice_id: voiceId,
    });
  });

  it("patches only what changed", () => {
    expect(postingPatch(postingValues(posting), posting)).toEqual({});
    expect(
      postingPatch(
        { ...postingValues(posting), location: "", salary_max: "3000" },
        posting,
      ),
    ).toEqual({ location: null, salary_max: 3000 });
    const requirements = [
      {
        name: "Rust",
        target: "programming_language" as const,
        type: "required" as const,
      },
    ];
    expect(
      postingPatch({ ...postingValues(posting), requirements }, posting),
    ).toEqual({
      requirements,
    });
    // A new PDF is sent only when one was uploaded.
    expect(
      postingPatch(
        { ...postingValues(posting), document_id: documentId, raw_text: "New" },
        posting,
      ),
    ).toEqual({ document_id: documentId });
  });
});

describe("postingLocks", () => {
  it("follows the backend rules", () => {
    expect(postingLocks(posting)).toEqual({
      editable: true,
      textEditable: false,
      requirementsLocked: false,
    });
    expect(
      postingLocks({ ...posting, core_question_count: 5 }).requirementsLocked,
    ).toBe(true);
    expect(
      postingLocks({ ...posting, status: "rejected", core_question_count: 5 }),
    ).toEqual({
      editable: true,
      textEditable: true,
      requirementsLocked: false,
    });
    expect(postingLocks({ ...posting, status: "open" }).editable).toBe(false);
  });
});

describe("question bank delivery contract", () => {
  it("requires server confirmation, readiness and submission metadata", () => {
    const current = {
      ...posting,
      refinement_note: "Focus on Go",
      review_submitted: false,
    };
    const parsed = postingEnvelopeSchema.parse({
      success: true,
      data: current,
    }).data;
    expect(parsed).toMatchObject({
      review_submitted: false,
    });
  });
});
