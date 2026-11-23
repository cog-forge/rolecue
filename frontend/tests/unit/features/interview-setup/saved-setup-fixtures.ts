import type { SavedSetup } from "@/features/interview-setup/api/saved-setup";

export const miaId = "44444444-4444-4444-8444-000000000001";
export const ducId = "44444444-4444-4444-8444-000000000002";
export const avaVoiceId = "55555555-5555-4555-8555-000000000001";
export const profileId = "22222222-2222-4222-8222-222222222222";
export const applicationId = "77777777-7777-4777-8777-000000000001";

// A GET / PUT answer of the setup API. The default is a never-saved practice setup.
export function practiceView(over: Partial<SavedSetup> = {}): SavedSetup {
  return {
    mode: "practice",
    role_profile_id: profileId,
    target_title: "Backend Engineer",
    avatar: null,
    voice: null,
    environment: null,
    locked_fields: [],
    saved: false,
    ready: false,
    blockers: [
      { code: "avatar_required", field: "avatar" },
      { code: "voice_required", field: "voice" },
      { code: "environment_required", field: "environment" },
    ],
    ...over,
  };
}

export function recruitmentView(over: Partial<SavedSetup> = {}): SavedSetup {
  return {
    mode: "recruitment",
    application_id: applicationId,
    target_title: "Go Engineer",
    avatar: { id: miaId, name: "Mia" },
    voice: { id: avaVoiceId, name: "Ava - English" },
    environment: null,
    locked_fields: ["avatar", "voice"],
    saved: false,
    ready: false,
    blockers: [{ code: "environment_required", field: "environment" }],
    ...over,
  };
}

export const envelope = (data: unknown) => ({ data: { success: true, data } });
