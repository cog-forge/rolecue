import {
  Box3,
  DirectionalLight,
  HemisphereLight,
  PerspectiveCamera,
  Scene,
  SRGBColorSpace,
  Vector3,
  WebGLRenderer,
} from "three";
import { VRMUtils, type VRM } from "@pixiv/three-vrm";
import type { ModelAccess } from "./avatar-viewer";
import { loadVrm } from "./load-vrm";
import { poseIdle } from "./pose";

// Portraits share one renderer: a page of avatar cards must not open a WebGL context per card,
// since browsers keep only 8-16 alive. Requests run one at a time in arrival order.
const width = 480;
const height = 600;
const fov = 24;
let renderer: WebGLRenderer | null = null;
let queue: Promise<unknown> = Promise.resolve();

function sharedRenderer() {
  if (!renderer) {
    const created = new WebGLRenderer({
      antialias: true,
      alpha: true,
      preserveDrawingBuffer: true,
    });
    created.setPixelRatio(1);
    created.setSize(width, height, false);
    created.outputColorSpace = SRGBColorSpace;
    created.setClearColor(0x000000, 0);
    created.domElement.addEventListener("webglcontextlost", () => {
      created.dispose();
      if (renderer === created) renderer = null;
    });
    renderer = created;
  }
  return renderer;
}

export function renderPortrait(access: ModelAccess, signal: AbortSignal) {
  const run = queue.then(() => draw(access, signal));
  queue = run.catch(() => undefined);
  return run;
}

async function draw(access: ModelAccess, signal: AbortSignal) {
  signal.throwIfAborted();
  const vrm = await loadVrm(access, signal);
  try {
    signal.throwIfAborted();
    return await photograph(vrm);
  } finally {
    VRMUtils.deepDispose(vrm.scene);
  }
}

// Frames head to upper thighs, slightly from the front-left, the way an interviewer is seen at a desk.
async function photograph(vrm: VRM) {
  poseIdle(vrm);
  const scene = new Scene();
  scene.add(new HemisphereLight(0xffffff, 0x8a8a8a, 1.7));
  const key = new DirectionalLight(0xffffff, 1.9);
  key.position.set(1.2, 2.6, 2.4);
  scene.add(key);
  scene.add(vrm.scene);
  vrm.scene.rotation.y = -0.18;
  vrm.scene.updateMatrixWorld(true);

  const bounds = new Box3().setFromObject(vrm.scene);
  const hips = vrm.humanoid
    .getRawBoneNode("hips")
    ?.getWorldPosition(new Vector3());
  const top = bounds.max.y;
  const bottom = hips
    ? hips.y - (top - hips.y) * 0.18
    : bounds.min.y + (top - bounds.min.y) * 0.4;
  const center = bounds.getCenter(new Vector3());
  const span = (top - bottom) * 1.08;
  const distance = span / 2 / Math.tan(((fov / 2) * Math.PI) / 180);
  const camera = new PerspectiveCamera(fov, width / height, 0.05, 50);
  const middle = (top + bottom) / 2;
  camera.position.set(center.x, middle + span * 0.03, center.z + distance);
  camera.lookAt(center.x, middle, center.z);

  const target = sharedRenderer();
  target.render(scene, camera);
  scene.remove(vrm.scene);
  return new Promise<Blob>((resolve, reject) =>
    target.domElement.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error("portrait failed"))),
      "image/webp",
      0.9,
    ),
  );
}
