import { describe, expect, it } from "vitest";
import { AxiosError, AxiosHeaders } from "axios";
import {
  isAccessDenied,
  isVersionConflict,
} from "@/features/admin/guidelines/hooks/use-interview-guidelines";

function failure(status: number, code: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message: "x" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

describe("guideline error classes", () => {
  it("recognises a version conflict only by status and code together", () => {
    expect(isVersionConflict(failure(409, "GUIDELINE_VERSION_CONFLICT"))).toBe(
      true,
    );
    expect(isVersionConflict(failure(409, "OTHER_CONFLICT"))).toBe(false);
    expect(isVersionConflict(failure(500, "GUIDELINE_VERSION_CONFLICT"))).toBe(
      false,
    );
    expect(isVersionConflict(new Error("boom"))).toBe(false);
  });

  it("treats 401 and 403 as lost access", () => {
    expect(isAccessDenied(failure(401, "INVALID_TOKEN"))).toBe(true);
    expect(isAccessDenied(failure(403, "FORBIDDEN"))).toBe(true);
    expect(isAccessDenied(failure(409, "GUIDELINE_VERSION_CONFLICT"))).toBe(
      false,
    );
    expect(isAccessDenied(new Error("boom"))).toBe(false);
  });
});
