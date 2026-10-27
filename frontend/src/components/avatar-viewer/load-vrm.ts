import { GLTFLoader } from "three/addons/loaders/GLTFLoader.js";
import { VRM, VRMLoaderPlugin, VRMUtils } from "@pixiv/three-vrm";
import type { ModelAccess } from "./avatar-viewer";

const expiryMarginMs = 15_000;

export type VrmLoadErrorKind = "expired" | "network" | "invalid";

export class VrmLoadError extends Error {
  constructor(readonly kind: VrmLoadErrorKind) {
    super(`VRM load failed: ${kind}`);
  }
}

// The grant is a presigned object URL: RoleCue cookies and the referrer must never reach storage.
export async function loadVrm(access: ModelAccess, signal: AbortSignal) {
  // Don't start a download the grant can't finish; the owner fetches a fresh one instead.
  if (Date.parse(access.expiresAt) - Date.now() < expiryMarginMs)
    throw new VrmLoadError("expired");
  let response: Response;
  try {
    response = await fetch(access.url, {
      method: access.method,
      headers: access.headers,
      credentials: "omit",
      referrerPolicy: "no-referrer",
      signal,
    });
  } catch (error) {
    if (signal.aborted) throw error;
    throw new VrmLoadError("network");
  }
  // S3 answers 403 (or 400) once a presigned URL has expired.
  if (response.status === 400 || response.status === 403)
    throw new VrmLoadError("expired");
  if (!response.ok) throw new VrmLoadError("network");
  const buffer = await response.arrayBuffer();
  const loader = new GLTFLoader();
  loader.register((parser) => new VRMLoaderPlugin(parser));
  let vrm: unknown;
  try {
    const gltf = await loader.parseAsync(buffer, "");
    vrm = gltf.userData.vrm;
  } catch {
    throw new VrmLoadError("invalid");
  }
  if (!(vrm instanceof VRM)) throw new VrmLoadError("invalid");
  if (signal.aborted) {
    VRMUtils.deepDispose(vrm.scene);
    throw new DOMException("Aborted", "AbortError");
  }
  VRMUtils.removeUnnecessaryVertices(vrm.scene);
  VRMUtils.combineSkeletons(vrm.scene);
  VRMUtils.rotateVRM0(vrm);
  return vrm;
}
