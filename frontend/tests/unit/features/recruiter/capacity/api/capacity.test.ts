import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  closeIntake,
  fundCapacity,
  getCapacity,
} from "@/features/recruiter/capacity/api/capacity";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const path = `/job-postings/${postingId}/interview-capacity`;
const state = {
  interview_slot: 5,
  funded: true,
  price_per_slot: 1,
  cv_approvals: 0,
  consumed: 0,
  available_to_approve: 5,
  refundable: 5,
  posting_status: "open",
  refunded: null,
};

beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("capacity api", () => {
  it("reads the capacity state of the posting", async () => {
    mocks.get.mockResolvedValue({ data: { success: true, data: state } });
    await expect(getCapacity(postingId)).resolves.toEqual(state);
    expect(mocks.get).toHaveBeenCalledWith(path, { signal: undefined });
  });

  it("funds N slots with the count only", async () => {
    mocks.post.mockResolvedValue({ data: { success: true, data: state } });
    await expect(fundCapacity(postingId, 5)).resolves.toEqual(state);
    expect(mocks.post).toHaveBeenCalledWith(path, { slots: 5 });
  });

  it("rejects a funding response that does not match the contract", async () => {
    mocks.post.mockResolvedValue({ data: { success: true, data: {} } });
    await expect(fundCapacity(postingId, 5)).rejects.toThrow();
  });

  it("closes intake with a POST to the existing close-intake route", async () => {
    mocks.post.mockResolvedValue({ data: { success: true } });
    await closeIntake(postingId);
    expect(mocks.post).toHaveBeenCalledWith(
      `/job-postings/${postingId}/close-intake`,
    );
  });
});
