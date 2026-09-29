import { beforeEach, describe, expect, it, vi } from "vitest";

const { post } = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("@/lib/api/client", () => ({ apiClient: { post } }));

import { login } from "@/features/auth/api/login";

describe("login API contract", () => {
  beforeEach(() => post.mockReset());

  it("posts validated, trimmed credentials to the Go login endpoint and returns its JWT", async () => {
    post.mockResolvedValue({ data: { success: true, data: "access-token" } });

    await expect(
      login({ email: "  Candidate@Example.com  ", password: "passphrase" }),
    ).resolves.toEqual({ accessToken: "access-token" });
    expect(post).toHaveBeenCalledWith("/auth/login", {
      email: "Candidate@Example.com",
      password: "passphrase",
    });
  });

  it("rejects a malformed success envelope", async () => {
    post.mockResolvedValue({ data: { success: true, data: "" } });

    await expect(
      login({ email: "candidate@example.com", password: "passphrase" }),
    ).rejects.toThrow();
  });
});
