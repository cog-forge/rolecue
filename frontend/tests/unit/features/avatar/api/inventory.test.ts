import { beforeEach, describe, expect, it, vi } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import {
  deleteAvatar,
  inventoryFailure,
  listAvatars,
  purchaseSlots,
} from "@/features/avatar/api/inventory";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
}));
vi.mock("@/lib/api/client", () => ({ apiClient: mocks }));

function apiError(status: number, data?: unknown) {
  return new AxiosError(
    "Request failed",
    "ERR_BAD_RESPONSE",
    undefined,
    undefined,
    {
      status,
      statusText: "",
      data,
      headers: {},
      config: { headers: new AxiosHeaders() },
    },
  );
}
const envelope = (code: string) => ({
  success: false,
  error: { code, message: "backend message" },
});
const summary = { capacity: 4, used: 3, available: 1, is_full: false };

beforeEach(() => vi.clearAllMocks());

describe("inventoryFailure", () => {
  it.each([
    ["create", 409, "INVENTORY_FULL"],
    ["create", 422, "AVATAR_SOURCE_UNSUPPORTED"],
    ["create", 503, "AVATAR_CONVERTER_UNAVAILABLE"],
    ["create", 504, "AVATAR_CONVERSION_TIMEOUT"],
    ["purchase", 422, "INSUFFICIENT_BALANCE"],
    ["delete", 409, "AVATAR_IN_USE"],
    ["delete", 404, "AVATAR_NOT_FOUND"],
  ] as const)("confirms %s failure %s %s", (operation, status, code) => {
    expect(
      inventoryFailure(apiError(status, envelope(code)), operation),
    ).toEqual({
      confirmed: true,
      code,
      denied: false,
    });
  });

  it("treats access errors as confirmed and denied", () => {
    expect(
      inventoryFailure(apiError(403, envelope("FORBIDDEN")), "create"),
    ).toEqual({
      confirmed: true,
      code: "FORBIDDEN",
      denied: true,
    });
  });

  it.each([
    ["a dropped connection", new AxiosError("Network Error", "ERR_NETWORK")],
    [
      "a proxy error without an envelope",
      apiError(502, "<html>Bad gateway</html>"),
    ],
    [
      "an internal error that may follow a commit",
      apiError(500, envelope("INTERNAL_ERROR")),
    ],
    ["a code from another operation", apiError(409, envelope("AVATAR_IN_USE"))],
    ["a 2xx body that fails validation", new Error("ZodError")],
  ])("reports an unknown outcome for %s", (_, error) => {
    expect(inventoryFailure(error, "create")).toEqual({
      confirmed: false,
      denied: false,
    });
  });
});

describe("purchaseSlots", () => {
  it("returns the summary and balance from a 200", async () => {
    mocks.post.mockResolvedValue({
      status: 200,
      data: { success: true, data: { ...summary, balance: 7 } },
    });
    await expect(purchaseSlots(2)).resolves.toEqual({ ...summary, balance: 7 });
    expect(mocks.post).toHaveBeenCalledWith("/inventory/purchase", {
      amount: 2,
    });
  });

  it("never reports an empty 204 as a purchase", async () => {
    mocks.post.mockResolvedValue({ status: 204, data: "" });
    await expect(purchaseSlots(1)).rejects.toThrow();
  });

  it("never sends an amount outside 1 to 10", async () => {
    await expect(purchaseSlots(11)).rejects.toThrow();
    await expect(purchaseSlots(0)).rejects.toThrow();
    expect(mocks.post).not.toHaveBeenCalled();
  });
});

describe("inventory requests", () => {
  it("pages the list without unsupported parameters", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: { items: [], pagination: { limit: 12, offset: 12, total: 13 } },
      },
    });
    await listAvatars(2);
    expect(mocks.get).toHaveBeenCalledWith("/inventory/avatars", {
      params: { limit: 12, offset: 12 },
      signal: undefined,
    });
  });

  it("deletes with no body and requires the 204", async () => {
    mocks.delete.mockResolvedValue({ status: 204, data: "" });
    await deleteAvatar("11111111-1111-4111-8111-111111111111");
    expect(mocks.delete).toHaveBeenCalledWith(
      "/inventory/avatars/11111111-1111-4111-8111-111111111111",
    );
    mocks.delete.mockResolvedValue({ status: 200, data: {} });
    await expect(deleteAvatar("x")).rejects.toThrow();
  });
});
