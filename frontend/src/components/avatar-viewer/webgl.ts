type Capability = { available: boolean; environments: boolean };

let capability: Capability | undefined;

// Software rasterizers draw the avatar, but a full 3D room on top of it drops them to a few frames per
// second, so they keep the 2D backdrop.
const softwareRenderer =
  /swiftshader|llvmpipe|softpipe|software|basic render|mesa offscreen/i;

// Probed once per page: each probe creates a context, and browsers cap live contexts. Capability is
// not a guarantee; a context can still be lost later, which the canvas reports separately.
function probe(): Capability {
  if (capability === undefined) {
    try {
      const canvas = document.createElement("canvas");
      const webgl2 = canvas.getContext("webgl2");
      const context = webgl2 ?? canvas.getContext("webgl");
      if (!context) capability = { available: false, environments: false };
      else {
        const debug = context.getExtension("WEBGL_debug_renderer_info");
        const renderer = String(
          context.getParameter(
            debug ? debug.UNMASKED_RENDERER_WEBGL : context.RENDERER,
          ) ?? "",
        );
        const maxTexture = Number(
          context.getParameter(context.MAX_TEXTURE_SIZE),
        );
        capability = {
          available: true,
          environments:
            webgl2 !== null &&
            maxTexture >= 4096 &&
            !softwareRenderer.test(renderer),
        };
      }
      context?.getExtension("WEBGL_lose_context")?.loseContext();
    } catch {
      capability = { available: false, environments: false };
    }
  }
  return capability;
}

export function isWebGLAvailable() {
  return probe().available;
}

/** WebGL 2 on a hardware GPU with 4K textures: enough for a photographic 3D room behind the avatar. */
export function canRenderEnvironments() {
  return probe().environments;
}
