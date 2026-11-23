// A stateful stand-in for the saved-setup endpoints (KAN-231): GET and PUT of
// /interview-setups/practice/{id} and /interview-setups/recruitment/{id}. Blockers follow the API
// contract (api/docs/interview-setup-api.md); the stored choice survives reloads within a test.
export const environments = [
  "cafe-3d",
  "office-3d",
  "meeting-3d",
  "office",
  "balcony",
  "cafe",
];

type Named = Record<string, string>;
type Stored = {
  avatar_id: string | null;
  voice_id: string | null;
  environment: string | null;
  saved: boolean;
};
type Blocker = { code: string; field?: string };

export type SetupMock = ReturnType<typeof createSetupMock>;

export function createSetupMock(options: {
  profileId: string;
  applicationId: string;
  avatars: Named;
  voices: Named;
  /** Voices an admin switched off: still stored, reported as unavailable. */
  inactiveVoices?: string[];
  initial?: Partial<Stored>;
  lockedAvatar?: [string, string];
  lockedVoice?: [string, string];
}) {
  const stored: Stored = {
    avatar_id: null,
    voice_id: null,
    environment: null,
    saved: false,
    ...options.initial,
  };
  const puts: Record<string, unknown>[] = [];
  const named = (map: Named, id: string | null) =>
    id && map[id] ? { id, name: map[id] } : null;

  function view(mode: "practice" | "recruitment") {
    const blockers: Blocker[] = [];
    if (mode === "practice") {
      if (!stored.avatar_id)
        blockers.push({ code: "avatar_required", field: "avatar" });
      else if (!options.avatars[stored.avatar_id])
        blockers.push({ code: "avatar_unavailable", field: "avatar" });
      if (!stored.voice_id)
        blockers.push({ code: "voice_required", field: "voice" });
      else if (options.inactiveVoices?.includes(stored.voice_id))
        blockers.push({ code: "voice_unavailable", field: "voice" });
    }
    if (!stored.environment)
      blockers.push({ code: "environment_required", field: "environment" });
    else if (!environments.includes(stored.environment))
      blockers.push({ code: "environment_unsupported", field: "environment" });
    const common = {
      mode,
      environment: stored.environment,
      saved: stored.saved,
      updated_at: stored.saved ? "2026-11-18T08:00:00Z" : null,
      ready: blockers.length === 0,
      blockers,
    };
    return mode === "practice"
      ? {
          ...common,
          role_profile_id: options.profileId,
          target_title: "Frontend Engineer – Design System",
          avatar: named(options.avatars, stored.avatar_id),
          voice: named(options.voices, stored.voice_id),
          locked_fields: [],
        }
      : {
          ...common,
          application_id: options.applicationId,
          target_title: "Go Backend Engineer",
          avatar: options.lockedAvatar
            ? { id: options.lockedAvatar[0], name: options.lockedAvatar[1] }
            : null,
          voice: options.lockedVoice
            ? { id: options.lockedVoice[0], name: options.lockedVoice[1] }
            : null,
          locked_fields: ["avatar", "voice"],
        };
  }

  /** The answer for a setup request, or null when the path is not a setup path. */
  function handle(path: string, method: string, body: unknown) {
    const match = /^\/interview-setups\/(practice|recruitment)\/[^/]+$/.exec(
      path,
    );
    if (!match) return null;
    const mode = match[1] as "practice" | "recruitment";
    if (method === "PUT") {
      const input = (body ?? {}) as Record<string, string | null | undefined>;
      puts.push(input);
      if (
        mode === "recruitment" &&
        ("avatar_id" in input || "voice_id" in input)
      )
        return validation();
      if (input.environment && !environments.includes(input.environment))
        return validation();
      if (mode === "practice") {
        stored.avatar_id = input.avatar_id ?? null;
        stored.voice_id = input.voice_id ?? null;
      }
      stored.environment = input.environment ?? null;
      stored.saved = true;
    }
    return { status: 200, data: view(mode) as unknown };
  }

  return { handle, puts, stored };
}

function validation() {
  return {
    status: 400,
    data: {
      success: false,
      error: { code: "VALIDATION_ERROR", message: "invalid" },
    },
  };
}
