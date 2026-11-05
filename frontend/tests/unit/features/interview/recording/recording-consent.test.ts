import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  acceptedConsentVersion,
  CONSENT_VERSION,
  forgetRecordingConsent,
  rememberRecordingConsent,
} from "@/features/interview/recording/recording-consent";
import { createRecordingUpload } from "@/features/interview/recording/recording-api";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { post: mocks.post, get: vi.fn() },
}));

const grantResponse = {
  data: {
    success: true,
    data: {
      upload_url: "https://storage.example.test/put",
      method: "PUT",
      headers: {},
      expires_at: "2026-11-05T08:15:00Z",
    },
  },
};

beforeEach(() => {
  window.localStorage.clear();
  mocks.post.mockReset();
  mocks.post.mockResolvedValue(grantResponse);
});

describe("the remembered recording consent", () => {
  it("is versioned and kept per account", () => {
    expect(CONSENT_VERSION).toBe("2026-11-v1");
    expect(acceptedConsentVersion("user-a")).toBeUndefined();
    rememberRecordingConsent("user-a");
    expect(acceptedConsentVersion("user-a")).toBe(CONSENT_VERSION);
    expect(acceptedConsentVersion("user-b")).toBeUndefined();
    forgetRecordingConsent("user-a");
    expect(acceptedConsentVersion("user-a")).toBeUndefined();
  });

  it("survives a browser without storage", () => {
    const broken = vi
      .spyOn(Storage.prototype, "getItem")
      .mockImplementation(() => {
        throw new Error("blocked");
      });
    expect(acceptedConsentVersion("user-a")).toBeUndefined();
    broken.mockRestore();
  });
});

describe("the upload grant request", () => {
  it("sends the accepted consent version", async () => {
    await createRecordingUpload("interview-1", {
      contentType: "video/webm",
      size: 10,
      consentVersion: CONSENT_VERSION,
    });
    expect(mocks.post.mock.calls[0][1]).toEqual({
      content_type: "video/webm",
      size: 10,
      consent_version: "2026-11-v1",
    });
  });

  it("sends none when none was accepted", async () => {
    await createRecordingUpload("interview-1", {
      contentType: "video/webm",
      size: 10,
    });
    expect(mocks.post.mock.calls[0][1]).toEqual({
      content_type: "video/webm",
      size: 10,
    });
  });
});
