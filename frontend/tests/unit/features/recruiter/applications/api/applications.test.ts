import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  approveApplicationCv,
  getApplicationDetail,
  rejectApplicationCv,
} from "@/features/recruiter/applications/api/applications";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));

const POSTING = "33333333-3333-4333-8333-333333333333";
const APPLICATION = "44444444-4444-4444-8444-444444444444";
const path = `/job-postings/${POSTING}/applications/${APPLICATION}`;

const detail = {
  id: APPLICATION,
  job_posting_id: POSTING,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-555555555555",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T09:00:00Z",
  job_posting: {
    id: POSTING,
    title: "Backend Engineer",
    status: "open",
    interview_slot: null,
    cv_approvals: 0,
  },
  cv: null,
};
const screening = {
  id: APPLICATION,
  job_posting_id: POSTING,
  status: "interview_eligible",
  interview_deadline: "2026-10-21T08:00:00Z",
  reject_reason: null,
  updated_at: "2026-10-20T09:00:00Z",
  job_posting_status: "open",
};

beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("application detail api", () => {
  it("gets the detail path and parses the envelope", async () => {
    mocks.get.mockResolvedValue({ data: { success: true, data: detail } });
    const controller = new AbortController();

    const result = await getApplicationDetail(
      POSTING,
      APPLICATION,
      controller.signal,
    );

    expect(mocks.get).toHaveBeenCalledWith(path, {
      signal: controller.signal,
    });
    expect(result.candidate.name).toBe("Linh Tran");
    expect(result.job_posting.status).toBe("open");
  });

  it("encodes ids in the detail path", async () => {
    mocks.get.mockResolvedValue({ data: { success: true, data: detail } });

    await getApplicationDetail("a/b", "c?d");

    expect(mocks.get).toHaveBeenCalledWith(
      "/job-postings/a%2Fb/applications/c%3Fd",
      { signal: undefined },
    );
  });

  it("throws on a malformed detail response", async () => {
    mocks.get.mockResolvedValue({
      data: { success: true, data: { ...detail, job_posting: undefined } },
    });
    await expect(getApplicationDetail(POSTING, APPLICATION)).rejects.toThrow();
  });

  it("posts approve-cv and parses the screening response", async () => {
    mocks.post.mockResolvedValue({ data: { success: true, data: screening } });

    const result = await approveApplicationCv(POSTING, APPLICATION);

    expect(mocks.post).toHaveBeenCalledWith(`${path}/approve-cv`, {});
    expect(result.status).toBe("interview_eligible");
  });

  it("posts reject-cv with the reason when one is given", async () => {
    mocks.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          ...screening,
          status: "rejected",
          interview_deadline: null,
          reject_reason: "No Go experience",
        },
      },
    });

    const result = await rejectApplicationCv(
      POSTING,
      APPLICATION,
      "No Go experience",
    );

    expect(mocks.post).toHaveBeenCalledWith(`${path}/reject-cv`, {
      reject_reason: "No Go experience",
    });
    expect(result.reject_reason).toBe("No Go experience");
  });

  it("posts an empty body when reject-cv has no reason", async () => {
    mocks.post.mockResolvedValue({
      data: { success: true, data: { ...screening, status: "rejected" } },
    });

    await rejectApplicationCv(POSTING, APPLICATION);

    expect(mocks.post).toHaveBeenCalledWith(`${path}/reject-cv`, {});
  });

  it("throws on a malformed screening response", async () => {
    mocks.post.mockResolvedValue({
      data: { success: true, data: { ...screening, job_posting_status: 1 } },
    });
    await expect(approveApplicationCv(POSTING, APPLICATION)).rejects.toThrow();
  });
});
