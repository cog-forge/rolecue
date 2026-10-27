import type { ModelAccess } from "./avatar-viewer";

/**
 * Renders a still portrait of a VRM and returns an object URL for an <img>. three.js and the VRM
 * loader load on first use only, so routes that show portraits keep them out of their bundle.
 * The caller owns the URL and revokes it when the portrait is no longer shown.
 */
export async function renderAvatarPortrait(
  access: ModelAccess,
  signal: AbortSignal,
) {
  const { renderPortrait } = await import("./portrait-renderer");
  return URL.createObjectURL(await renderPortrait(access, signal));
}
