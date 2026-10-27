import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  decideApplication,
  getApplicationRecording,
  getApplicationTranscript,
  getInterviewReport,
} from "@/features/recruiter/applications/api/applications";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));

const POSTING = "33333333-3333-4333-8333-333333333333";
const APPLICATION = "44444444-4444-4444-8444-444444444444";
const INTERVIEW = "66666666-6666-4666-8666-666666666666";
const path = `/job-postings/${POSTING}/applications/${APPLICATION}`;

const detail = {
  id: APPLICATION,
  job_posting_id: POSTING,
  job_posting_title: "Backend Engineer",
  status: "approved",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-555555555555",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: "2026-10-21T08:00:00Z",
  reject_reason: null,
  updated_at: "2026-10-22T09:00:00Z",
  job_posting: {
    id: POSTING,
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 2,
  },
  cv: null,
};

beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("interview evidence api", () => {
  it("reads the report of an interview", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          interview_id: INTERVIEW,
          type: "recruitment",
          status: "pending",
          overall_score: null,
          started_at: "2026-10-20T08:00:00Z",
          completed_at: null,
          summary: null,
          breakdown: null,
          questions: null,
        },
      },
    });

    const report = await getInterviewReport(INTERVIEW);

    expect(mocks.get).toHaveBeenCalledWith(`/interviews/${INTERVIEW}/report`, {
      signal: undefined,
    });
    expect(report.status).toBe("pending");
  });

  it("reads the recording link from the application path", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          record_status: "ready",
          url: "https://files.example.com/rec.webm",
          expires_at: "2026-10-24T10:05:00Z",
        },
      },
    });

    const recording = await getApplicationRecording(POSTING, APPLICATION);

    expect(mocks.get).toHaveBeenCalledWith(`${path}/recording`, {
      signal: undefined,
    });
    expect(recording.url).toBe("https://files.example.com/rec.webm");
  });

  it("reads the transcript from the application path", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          turns: [{ position: 1, question: "Why Go?", answer: "Concurrency." }],
        },
      },
    });

    const transcript = await getApplicationTranscript(POSTING, APPLICATION);

    expect(mocks.get).toHaveBeenCalledWith(`${path}/transcript`, {
      signal: undefined,
    });
    expect(transcript.turns[0]?.answer).toBe("Concurrency.");
  });

  it("throws on a malformed report", async () => {
    mocks.get.mockResolvedValue({
      data: { success: true, data: { interview_id: "not-a-uuid" } },
    });
    await expect(getInterviewReport(INTERVIEW)).rejects.toThrow();
  });
});

describe("decideApplication", () => {
  it("posts an approval with only the decision field", async () => {
    mocks.post.mockResolvedValue({ data: { success: true, data: detail } });

    const result = await decideApplication(POSTING, APPLICATION, "approve");

    expect(mocks.post).toHaveBeenCalledWith(`${path}/decision`, {
      decision: "approve",
    });
    expect(result.status).toBe("approved");
  });

  it("posts a rejection with the reason when one is given", async () => {
    mocks.post.mockResolvedValue({
      data: {
        success: true,
        data: { ...detail, status: "rejected", reject_reason: "Weak testing" },
      },
    });

    const result = await decideApplication(
      POSTING,
      APPLICATION,
      "reject",
      "Weak testing",
    );

    expect(mocks.post).toHaveBeenCalledWith(`${path}/decision`, {
      decision: "reject",
      reason: "Weak testing",
    });
    expect(result.reject_reason).toBe("Weak testing");
  });

  it("posts a rejection without a reason field when none is given", async () => {
    mocks.post.mockResolvedValue({
      data: { success: true, data: { ...detail, status: "rejected" } },
    });

    await decideApplication(POSTING, APPLICATION, "reject");

    expect(mocks.post).toHaveBeenCalledWith(`${path}/decision`, {
      decision: "reject",
    });
  });

  it("throws on a malformed updated detail", async () => {
    mocks.post.mockResolvedValue({
      data: { success: true, data: { ...detail, job_posting: undefined } },
    });
    await expect(
      decideApplication(POSTING, APPLICATION, "approve"),
    ).rejects.toThrow();
  });
});
