import { expect, it } from "vitest";
import {
  readSetupParams,
  setupHref,
} from "@/features/interview-setup/lib/setup-params";

const profile = "22222222-2222-4222-8222-222222222222";
const avatar = "44444444-4444-4444-8444-444444444444";

it("keeps only UUID choices from a page's search params", () => {
  expect(
    readSetupParams({
      roleProfileId: profile,
      avatarId: "../../etc",
      voiceId: [avatar, avatar],
      applicationId: undefined,
      other: "x",
    }),
  ).toEqual({ roleProfileId: profile });
});

it("reads the browser's URLSearchParams the same way", () => {
  expect(
    readSetupParams(
      new URLSearchParams(`roleProfileId=${profile}&avatarId=${avatar}`),
    ),
  ).toEqual({ roleProfileId: profile, avatarId: avatar });
});

it("builds a step URL in a stable order and leaves out unset choices", () => {
  expect(
    setupHref("/interviews/new/preflight", {
      avatarId: avatar,
      voiceId: undefined,
      roleProfileId: profile,
    }),
  ).toBe(
    `/interviews/new/preflight?roleProfileId=${profile}&avatarId=${avatar}`,
  );
  expect(setupHref("/interviews/new/setup", {})).toBe("/interviews/new/setup");
});
