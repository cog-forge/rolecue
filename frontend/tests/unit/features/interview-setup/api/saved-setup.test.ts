import { beforeEach, expect, it, vi } from "vitest";
import {
  getSavedSetup,
  putSavedSetup,
  saveBody,
} from "@/features/interview-setup/api/saved-setup";
import { roomBackgrounds } from "@/features/interview/components/room-backgrounds";
import {
  applicationId,
  envelope,
  miaId,
  practiceView,
  profileId,
  recruitmentView,
} from "../saved-setup-fixtures";

const mocks = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, put: mocks.put },
}));

const practice = { mode: "practice", roleProfileId: profileId } as const;
const recruitment = { mode: "recruitment", applicationId } as const;

beforeEach(() => {
  mocks.get.mockReset();
  mocks.put.mockReset();
});

it("reads the practice and recruitment views by their own paths", async () => {
  mocks.get.mockResolvedValueOnce(envelope(practiceView()));
  mocks.get.mockResolvedValueOnce(envelope(recruitmentView()));
  expect((await getSavedSetup(practice)).mode).toBe("practice");
  expect((await getSavedSetup(recruitment)).mode).toBe("recruitment");
  expect(mocks.get.mock.calls.map((call) => call[0])).toEqual([
    `/interview-setups/practice/${profileId}`,
    `/interview-setups/recruitment/${applicationId}`,
  ]);
});

it("sends the full replace for practice and only the environment for recruitment", async () => {
  const draft = { avatarId: miaId, voiceId: null, environment: "office-3d" };
  expect(saveBody(practice, draft)).toEqual({
    avatar_id: miaId,
    voice_id: null,
    environment: "office-3d",
  });
  expect(saveBody(recruitment, draft)).toEqual({ environment: "office-3d" });
  mocks.put.mockResolvedValue(envelope(recruitmentView()));
  await putSavedSetup(recruitment, draft);
  expect(mocks.put).toHaveBeenCalledWith(
    `/interview-setups/recruitment/${applicationId}`,
    { environment: "office-3d" },
  );
});

it("rejects an answer that is not a setup view", async () => {
  mocks.get.mockResolvedValue(envelope({ mode: "practice" }));
  await expect(getSavedSetup(practice)).rejects.toThrow();
});

it("offers the same room ids as the backend catalog", () => {
  expect(roomBackgrounds.map((room) => room.id)).toEqual([
    "cafe-3d",
    "office-3d",
    "meeting-3d",
    "office",
    "balcony",
    "cafe",
  ]);
});
