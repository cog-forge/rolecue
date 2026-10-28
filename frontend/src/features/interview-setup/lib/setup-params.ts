import { z } from "zod";

// The setup choices live in the URL, so every step is a real, shareable address and a reload keeps
// them. Nothing is stored in the browser. A value that is not a UUID is dropped, never sent.
export const setupParamNames = [
  "roleProfileId",
  "avatarId",
  "voiceId",
  "applicationId",
] as const;
export type SetupParams = Partial<
  Record<(typeof setupParamNames)[number], string>
>;

const uuid = z.uuid();

type RawParams =
  URLSearchParams | Record<string, string | string[] | undefined>;

export function readSetupParams(raw: RawParams): SetupParams {
  const params: SetupParams = {};
  for (const name of setupParamNames) {
    const value = raw instanceof URLSearchParams ? raw.get(name) : raw[name];
    if (typeof value === "string" && uuid.safeParse(value).success)
      params[name] = value;
  }
  return params;
}

export function setupHref(path: string, params: SetupParams) {
  const query = new URLSearchParams();
  for (const name of setupParamNames) {
    const value = params[name];
    if (value) query.set(name, value);
  }
  const search = query.toString();
  return search ? `${path}?${search}` : path;
}
