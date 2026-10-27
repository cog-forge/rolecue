let available: boolean | undefined;

// Probed once per page: each probe creates a context, and browsers cap live contexts. Capability is
// not a guarantee; a context can still be lost later, which the canvas reports separately.
export function isWebGLAvailable() {
  if (available === undefined) {
    try {
      const canvas = document.createElement("canvas");
      const context = canvas.getContext("webgl2") ?? canvas.getContext("webgl");
      available = !!context;
      context?.getExtension("WEBGL_lose_context")?.loseContext();
    } catch {
      available = false;
    }
  }
  return available;
}
