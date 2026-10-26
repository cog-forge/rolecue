import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  getCandidateApplication,
  listCandidateApplications,
  uploadApplicationCv,
} from "@/features/candidate/applications/api/applications";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-444444444444";

beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("candidate applications API", () => {
  it("uploads the CV as one multipart file part and reports progress", async () => {
    mocks.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          id: applicationId,
          job_posting_id: postingId,
          status: "pending",
          created_at: "2026-10-25T08:00:00Z",
        },
      },
    });
    const onProgress = vi.fn();
    const file = new File(["%PDF-1.4"], "cv.pdf", { type: "application/pdf" });

    const created = await uploadApplicationCv(postingId, file, onProgress);

    expect(created).toEqual({
      id: applicationId,
      job_posting_id: postingId,
      status: "pending",
      created_at: "2026-10-25T08:00:00Z",
    });
    const [url, body, config] = mocks.post.mock.calls[0];
    expect(url).toBe(`/job-board/${postingId}/applications`);
    expect(body).toBeInstanceOf(FormData);
    expect((body as FormData).get("file")).toBe(file);
    (
      config as {
        onUploadProgress: (e: { loaded: number; total?: number }) => void;
      }
    ).onUploadProgress({ loaded: 1, total: 4 });
    (
      config as {
        onUploadProgress: (e: { loaded: number; total?: number }) => void;
      }
    ).onUploadProgress({ loaded: 3 });
    expect(onProgress.mock.calls).toEqual([[25]]);
  });

  it("rejects a response that does not match the contract", async () => {
    mocks.post.mockResolvedValue({
      data: { success: true, data: { id: "x" } },
    });
    const file = new File(["%PDF"], "cv.pdf", { type: "application/pdf" });
    await expect(uploadApplicationCv(postingId, file)).rejects.toThrow();
  });

  it("lists one page of the Candidate's applications", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: { items: [], total: 0, page: 3, page_size: 20 },
      },
    });
    const page = await listCandidateApplications(3);
    expect(page).toEqual({ items: [], total: 0, page: 3, page_size: 20 });
    expect(mocks.get).toHaveBeenCalledWith("/applications", {
      params: { page: 3, page_size: 20 },
      signal: undefined,
    });
  });

  it("reads one application by id", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          id: applicationId,
          job_posting_id: postingId,
          job_posting_title: "Go Backend Engineer",
          status: "interview_eligible",
          reject_reason: null,
          interview_deadline: "2026-10-26T08:00:00Z",
          can_start_interview: true,
          created_at: "2026-10-20T08:00:00Z",
        },
      },
    });
    const application = await getCandidateApplication(applicationId);
    expect(application.can_start_interview).toBe(true);
    expect(mocks.get).toHaveBeenCalledWith(`/applications/${applicationId}`, {
      signal: undefined,
    });
  });
});
