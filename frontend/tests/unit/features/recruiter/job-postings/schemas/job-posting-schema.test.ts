import { describe, expect, it } from "vitest";
import {
  createBody,
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
  core_question_count: 0,
  created_at: "2026-10-12T08:00:00Z",
};

const valid: PostingValues = {
  ...emptyPostingValues,
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

  it("counts the JD text in Unicode characters, trimmed like Go", () => {
    expect(issues({ ...valid, raw_text: "ệ".repeat(100) })).toEqual([]);
    expect(issues({ ...valid, raw_text: `  ${"ệ".repeat(99)}  ` })).toEqual([
      "raw_text",
    ]);
    expect(issues({ ...valid, raw_text: "a".repeat(20_001) })).toEqual([
      "raw_text",
    ]);
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
      raw_text: valid.raw_text,
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
