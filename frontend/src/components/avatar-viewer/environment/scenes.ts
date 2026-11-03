/**
 * Photographic 3D rooms for the interview stage (KAN-275). Each scene is a 360° photo projected onto a
 * ground-projected skybox, the HDR of the same photo as image-based light, and real foreground
 * furniture. Assets and credits: `docs/interview-environments.md`.
 */
export type EnvironmentScene = {
  /** Equirectangular photo, already tone-mapped: drawn as is around the room. */
  backplate: string;
  /** Small HDR of the same photo: image-based light for the furniture. */
  lighting: string;
  /** Foreground furniture (GLB, meshopt + WebP). */
  props: string;
  /** Size of the three files above, for the progress bar. A unit test keeps it in sync. */
  bytes: number;
  /** The scene rendered without the interviewer: shown while it loads and as its 2D fallback. */
  still: string;
  thumbnail: string;
  /** Turns the photo so its best side is behind the interviewer, in radians. */
  rotation: number;
  /** Height of the camera that took the photo, in metres: puts the photo's floor at the avatar's feet. */
  photoHeight: number;
  /** Furniture placement in metres; the interviewer stands at the origin facing the camera (+z). */
  prop: {
    position: readonly [number, number, number];
    rotationY: number;
    scale: readonly [number, number, number];
  };
  /**
   * Direct lights matched to the photo. MToon avatars ignore image-based light, so these carry the
   * room's colour onto the interviewer.
   */
  light: {
    sky: string;
    ground: string;
    ambient: number;
    key: string;
    keyIntensity: number;
    rim: string;
    rimIntensity: number;
  };
};

export const environmentScenes = {
  cafe: {
    backplate: "/environments/cafe/backplate.webp",
    lighting: "/environments/cafe/lighting.hdr",
    props: "/environments/cafe/props.glb",
    bytes: 1_015_186,
    still: "/environments/cafe/still.webp",
    thumbnail: "/environments/cafe/thumb.webp",
    rotation: 0,
    photoHeight: 1.5,
    // A 1 m café table: its near edge hides the hips of the standing interviewer.
    prop: { position: [0, 0, 0.85], rotationY: 0, scale: [1, 1, 1] },
    light: {
      sky: "#fff1de",
      ground: "#6f6253",
      ambient: 0.7,
      key: "#ffe9cf",
      keyIntensity: 1.3,
      rim: "#ffd9a8",
      rimIntensity: 1.1,
    },
  },
  office: {
    backplate: "/environments/office/backplate.webp",
    lighting: "/environments/office/lighting.hdr",
    props: "/environments/office/props.glb",
    bytes: 1_466_838,
    still: "/environments/office/still.webp",
    thumbnail: "/environments/office/thumb.webp",
    rotation: 0,
    photoHeight: 1.5,
    // Raised to standing-desk height (1.03 m) so its edge frames the standing interviewer.
    prop: { position: [0, 0, 0.62], rotationY: 0, scale: [1, 1.3, 1] },
    light: {
      sky: "#f3f7ff",
      ground: "#7d7a74",
      ambient: 0.75,
      key: "#fff8ee",
      keyIntensity: 1.35,
      rim: "#e4efff",
      rimIntensity: 1.3,
    },
  },
  meeting: {
    backplate: "/environments/meeting/backplate.webp",
    lighting: "/environments/meeting/lighting.hdr",
    props: "/environments/meeting/props.glb",
    bytes: 1_054_057,
    still: "/environments/meeting/still.webp",
    thumbnail: "/environments/meeting/thumb.webp",
    rotation: 2.2,
    photoHeight: 1.5,
    // A conference table raised to 1.03 m, for the same reason as the office desk.
    prop: { position: [0, 0, 0.95], rotationY: 0, scale: [1, 1.3, 1] },
    light: {
      sky: "#f1f4f8",
      ground: "#77736d",
      ambient: 0.75,
      key: "#ffffff",
      keyIntensity: 1.3,
      rim: "#dfe8f5",
      rimIntensity: 1.2,
    },
  },
} as const satisfies Record<string, EnvironmentScene>;

export type EnvironmentSceneId = keyof typeof environmentScenes;
