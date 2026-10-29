import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  countPendingApplications,
  countPostings,
  summarizeDeadlines,
} from "@/features/recruiter/dashboard/api/dashboard";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));

const page = (items: unknown[], total: number) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 100, offset: 0, total } },
  },
});
const NOW = Date.parse("2026-10-29T08:00:00Z");
const at = (hours: number) => new Date(NOW + hours * 3_600_000).toISOString();

beforeEach(() => mocks.get.mockReset());

describe("dashboard counts", () => {
  it("reads the total of a one-row page, not the row count", async () => {
    mocks.get.mockResolvedValue(page([{}], 37));
    expect(await countPendingApplications()).toEqual({
      count: 37,
      atLeast: false,
      detail: null,
    });
    expect(await countPostings("approved")).toMatchObject({ count: 37 });
    expect(mocks.get).toHaveBeenLastCalledWith("/job-postings", {
      params: { status: "open", funded: "false", limit: 1, offset: 0 },
      signal: undefined,
    });
  });

  it("rejects a response without a numeric total", async () => {
    mocks.get.mockResolvedValue({ data: { success: true, data: {} } });
    await expect(countPendingApplications()).rejects.toThrow();
  });
});

describe("summarizeDeadlines", () => {
  it("counts deadlines in (now, now + 24h] and returns the earliest", async () => {
    mocks.get.mockResolvedValue(
      page(
        [
          { interview_eligible: true, interview_deadline: at(24) },
          { interview_eligible: true, interview_deadline: at(24.01) },
          { interview_eligible: true, interview_deadline: at(0) },
          { interview_eligible: true, interview_deadline: at(-1) },
          { interview_eligible: false, interview_deadline: at(2) },
          { interview_eligible: true, interview_deadline: null },
          { interview_eligible: true, interview_deadline: at(6) },
        ],
        7,
      ),
    );
    expect(await summarizeDeadlines(undefined, NOW)).toEqual({
      count: 2,
      atLeast: false,
      detail: at(6),
    });
  });

  it("is a lower bound when the API has more eligible candidates than one page", async () => {
    mocks.get.mockResolvedValue(
      page([{ interview_eligible: true, interview_deadline: at(3) }], 101),
    );
    expect(await summarizeDeadlines(undefined, NOW)).toMatchObject({
      count: 1,
      atLeast: true,
    });
  });
});
