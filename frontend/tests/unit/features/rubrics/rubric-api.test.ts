import { beforeEach, expect, it, vi } from "vitest";
import {
  getRubric,
  listRubrics,
  rubricKeys,
  writeRubric,
} from "@/features/rubrics/rubric-api";
const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  patch: vi.fn(),
  post: vi.fn(),
}));
vi.mock("@/lib/api/client", () => ({ apiClient: mocks }));
beforeEach(() => vi.clearAllMocks());
it("scopes caches to the signed-in user", () =>
  expect(rubricKeys.all("a")).not.toEqual(rubricKeys.all("b")));
it("passes list scope, pagination and abort signal to the API", async () => {
  const signal = new AbortController().signal;
  mocks.get.mockResolvedValue({
    data: { success: true, data: { items: [], pagination: { total: 0 } } },
  });
  expect(
    (await listRubrics("own", "archived", 20, signal)).pagination.total,
  ).toBe(0);
  expect(mocks.get).toHaveBeenCalledWith("/rubrics", {
    params: { scope: "own", status: "archived", limit: 20, offset: 20 },
    signal,
  });
});
it("rejects malformed detail envelopes", async () => {
  mocks.get.mockResolvedValue({
    data: { success: true, data: { id: "invalid" } },
  });
  await expect(getRubric("x")).rejects.toThrow();
});
it("forwards the original concurrency token and preserves server errors", async () => {
  const error = {
    response: { status: 409, data: { error: { code: "RUBRIC_STALE" } } },
  };
  mocks.patch.mockRejectedValue(error);
  const body = { updated_at: "2026-11-11T06:00:00.123456Z", is_active: false };
  await expect(writeRubric({ id: "x", body })).rejects.toBe(error);
  expect(mocks.patch).toHaveBeenCalledWith("/rubrics/x", body);
  expect(mocks.post).not.toHaveBeenCalled();
});
