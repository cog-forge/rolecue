export type GlassSurface = "squircle" | "circle" | "concave" | "lip";

export type GlassOptics = {
  /** Shape of the rim's cross-section. */
  surface?: GlassSurface;
  /** Index of refraction of the glass (air is 1). */
  ior?: number;
  /** Height of the glass above the backdrop, in px. */
  thickness?: number;
};

export type GlassMapOptions = GlassOptics & {
  width: number;
  height: number;
  /** Corner radius in px (clamped to half the shortest side). */
  radius: number;
  /** Width of the refracting rim in px. */
  bezel: number;
};

export type DisplacementMap = {
  /** Data URL of the RG vector map (128 = no displacement). */
  url: string;
  /**
   * `feDisplacementMap` scale: 2 × the largest displacement in px, because the
   * filter moves pixels by scale · (C / 255 − 0.5) and a full vector is ±0.5.
   */
  scale: number;
};

export const DEFAULT_IOR = 1.5;
/** Thickness relative to the bezel when none is given. */
export const DEFAULT_THICKNESS_RATIO = 1.4;
/** Samples taken across the bezel; one per 8-bit step of the encoded map. */
export const PROFILE_SAMPLES = 128;

const MAX_MAP_SIZE = 480;
const cache = new Map<string, DisplacementMap>();
const specularCache = new Map<string, string>();

const squircle = (t: number) => Math.pow(1 - Math.pow(1 - t, 4), 0.25);
const smootherstep = (t: number) => t * t * t * (t * (t * 6 - 15) + 10);

/**
 * Height of the glass across the bezel, normalised to 0 – 1.
 * `t` runs from 0 at the outer edge to 1 where the flat top begins.
 */
export const surfaces: Record<GlassSurface, (t: number) => number> = {
  /** Quarter circle: y = √(1 − (1 − t)²). Steep edge, round shoulder. */
  circle: (t) => Math.sqrt(1 - (1 - t) ** 2),
  /** Squircle: y = ⁴√(1 − (1 − t)⁴). Apple's profile — the flat top blends in smoothly. */
  squircle,
  /** Concave: y = 1 − squircle. A bowl that spreads light outwards. */
  concave: (t) => 1 - squircle(t),
  /** Lip: convex at the edge, concave inside, blended with smootherstep. */
  lip: (t) => {
    const k = smootherstep(t);
    return squircle(t) * (1 - k) + (1 - squircle(t)) * k;
  },
};

/**
 * Snell's law: n₁ sin θ₁ = n₂ sin θ₂.
 * Returns the refracted angle, or `null` on total internal reflection.
 */
export function refract(
  incidence: number,
  n1: number,
  n2: number,
): number | null {
  const s = (n1 / n2) * Math.sin(incidence);
  return Math.abs(s) > 1 ? null : Math.asin(s);
}

/** Slope dy/dt of a surface, by central difference. */
export function surfaceSlope(surface: GlassSurface, t: number) {
  const f = surfaces[surface];
  const e = 1e-4;
  const a = Math.max(0, t - e);
  const b = Math.min(1, t + e);
  return (f(b) - f(a)) / (b - a);
}

/**
 * Traces vertical rays through the bezel and returns how far (px) each one
 * lands from where it entered. Positive values bend inwards.
 *
 * At each sample the surface normal is tilted by θ₁ = atan(slope); the ray
 * refracts to θ₂ = asin(sin θ₁ / n) and travels down the glass height h, so
 * it shifts by h · tan(θ₁ − θ₂).
 */
export function displacementProfile({
  surface = "squircle",
  ior = DEFAULT_IOR,
  thickness,
  bezel,
  samples = PROFILE_SAMPLES,
}: GlassOptics & { bezel: number; samples?: number }) {
  const b = Math.max(1, bezel);
  const T = Math.max(0, thickness ?? b * DEFAULT_THICKNESS_RATIO);
  const n = Math.max(1, ior);
  const f = surfaces[surface];
  const values = new Float32Array(samples);
  let max = 0;
  for (let i = 0; i < samples; i++) {
    const t = Math.max(i / (samples - 1), 1e-3);
    const slope = (surfaceSlope(surface, t) * T) / b;
    const theta1 = Math.atan(Math.abs(slope));
    const theta2 = refract(theta1, 1, n) ?? theta1;
    const d = T * f(t) * Math.tan(theta1 - theta2) * Math.sign(slope);
    values[i] = d;
    max = Math.max(max, Math.abs(d));
  }
  return { values, max };
}

function sample(values: Float32Array, t: number) {
  const x = Math.min(1, Math.max(0, t)) * (values.length - 1);
  const i = Math.floor(x);
  const j = Math.min(values.length - 1, i + 1);
  return values[i] + (values[j] - values[i]) * (x - i);
}

/**
 * Walks every pixel of a rounded rectangle, calling `fn` with the distance
 * inside the edge and the outward normal. Returns the canvas.
 */
function paintRoundedRect(
  { width, height, radius }: { width: number; height: number; radius: number },
  fn: (
    dist: number,
    nx: number,
    ny: number,
    data: Uint8ClampedArray,
    i: number,
  ) => void,
) {
  const w = Math.max(1, Math.round(width));
  const h = Math.max(1, Math.round(height));
  // Render at a reduced resolution for large surfaces; feImage stretches it.
  const scale = Math.min(1, MAX_MAP_SIZE / Math.max(w, h));
  const mw = Math.max(1, Math.round(w * scale));
  const mh = Math.max(1, Math.round(h * scale));
  const r = Math.min(radius, w / 2, h / 2) * scale;

  const canvas = document.createElement("canvas");
  canvas.width = mw;
  canvas.height = mh;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;
  const img = ctx.createImageData(mw, mh);
  const data = img.data;
  const hw = mw / 2;
  const hh = mh / 2;

  for (let y = 0; y < mh; y++) {
    for (let x = 0; x < mw; x++) {
      const px = x + 0.5 - hw;
      const py = y + 0.5 - hh;
      const qx = Math.abs(px) - (hw - r);
      const qy = Math.abs(py) - (hh - r);

      // Signed distance to the rounded-rect edge and outward normal.
      let dist: number;
      let nx = 0;
      let ny = 0;
      if (qx > 0 && qy > 0) {
        const len = Math.hypot(qx, qy);
        dist = r - len;
        nx = (qx / len) * Math.sign(px);
        ny = (qy / len) * Math.sign(py);
      } else if (qx > qy) {
        dist = r - qx;
        nx = Math.sign(px);
      } else {
        dist = r - qy;
        ny = Math.sign(py);
      }
      fn(dist / scale, nx, ny, data, (y * mw + x) * 4);
    }
  }
  ctx.putImageData(img, 0, 0);
  return canvas.toDataURL();
}

function remember<T>(map: Map<string, T>, key: string, value: T) {
  const oldest = map.keys().next().value;
  if (map.size > 64 && oldest !== undefined) map.delete(oldest);
  map.set(key, value);
  return value;
}

/**
 * Builds the displacement map for a rounded rectangle.
 *
 * Each pixel in the bezel looks up its ray's displacement, divides it by the
 * largest one so it fits in −1 … 1, and stores the vector as
 * R = 128 + x·127, G = 128 + y·127. `feDisplacementMap` moves each pixel by
 * scale · (C / 255 − 0.5), so the returned scale of 2 × max recovers pixels.
 */
export function createDisplacementMap(
  options: GlassMapOptions,
): DisplacementMap {
  const { width, height, radius, bezel } = options;
  const b = Math.max(1, Math.min(bezel, width / 2, height / 2));
  const { values, max } = displacementProfile({ ...options, bezel: b });
  const key = [
    Math.round(width),
    Math.round(height),
    Math.round(radius),
    Math.round(b),
    options.surface ?? "squircle",
    (options.ior ?? DEFAULT_IOR).toFixed(3),
    Math.round(options.thickness ?? -1),
  ].join(":");
  const cached = cache.get(key);
  if (cached) return cached;

  const url = paintRoundedRect(
    { width, height, radius },
    (dist, nx, ny, data, i) => {
      let dx = 0;
      let dy = 0;
      if (max > 0 && dist > 0 && dist < b) {
        // Sample from where the ray lands: inwards for a positive shift.
        const m = sample(values, dist / b) / max;
        dx = -nx * m;
        dy = -ny * m;
      }
      data[i] = 128 + dx * 127;
      data[i + 1] = 128 + dy * 127;
      data[i + 2] = 128;
      data[i + 3] = 255;
    },
  );
  if (!url) return { url: "", scale: 0 };
  return remember(cache, key, { url, scale: max * 2 });
}

/**
 * Builds a specular rim: white wherever the bezel faces the light.
 * Brightness is how directly the tilted surface normal points at `angle`
 * (degrees, 0 = right, -90 = up), so both the lit edge and its opposite glint.
 */
export function createSpecularMap(
  options: GlassMapOptions & { angle?: number },
): string {
  const {
    width,
    height,
    radius,
    bezel,
    surface = "squircle",
    angle = -60,
  } = options;
  const b = Math.max(1, Math.min(bezel, width / 2, height / 2));
  const key = [
    Math.round(width),
    Math.round(height),
    Math.round(radius),
    Math.round(b),
    surface,
    angle,
  ].join(":");
  const cached = specularCache.get(key);
  if (cached) return cached;

  const lx = Math.cos((angle * Math.PI) / 180);
  const ly = Math.sin((angle * Math.PI) / 180);
  const url = paintRoundedRect(
    { width, height, radius },
    (dist, nx, ny, data, i) => {
      let a = 0;
      if (dist > 0 && dist < b) {
        // sin of the surface's tilt: 0 where flat, → 1 where vertical.
        const slope = Math.abs(surfaceSlope(surface, Math.max(dist / b, 1e-3)));
        const tilt = slope / Math.hypot(1, slope);
        const facing = Math.abs(nx * lx + ny * ly);
        a = Math.pow(facing, 3) * Math.pow(tilt, 0.6);
      }
      data[i] = 255;
      data[i + 1] = 255;
      data[i + 2] = 255;
      data[i + 3] = a * 255;
    },
  );
  return url ? remember(specularCache, key, url) : "";
}

const decoded = new Set<string>();

/**
 * Resolves once the map image is decoded (and painted for one frame), so an
 * SVG filter referencing it never renders before its input exists.
 */
export function preloadDisplacementMap(url: string): Promise<void> {
  if (decoded.has(url)) return Promise.resolve();
  const img = new Image();
  img.src = url;
  return img
    .decode()
    .catch(() => undefined)
    .then(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    )
    .then(() => {
      decoded.add(url);
    });
}

let support: boolean | undefined;

/** True when the browser can refract the backdrop with SVG filters (Chromium). */
export function supportsLiquidGlass(): boolean {
  if (support !== undefined) return support;
  if (typeof navigator === "undefined") return false;
  const ua = navigator.userAgent;
  const chromium =
    /Chrome\/|Chromium\//.test(ua) && !/CriOS|FxiOS|EdgiOS/.test(ua);
  const reduced =
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-transparency: reduce)").matches;
  support = chromium && !reduced;
  return support;
}
