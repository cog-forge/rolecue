import {
  EquirectangularReflectionMapping,
  Group,
  LinearFilter,
  Mesh,
  SRGBColorSpace,
  Texture,
  TextureLoader,
  type Material,
  type Object3D,
} from "three";
import { GLTFLoader } from "three/addons/loaders/GLTFLoader.js";
import { HDRLoader } from "three/addons/loaders/HDRLoader.js";
import { GroundedSkybox } from "three/addons/objects/GroundedSkybox.js";
import { MeshoptDecoder } from "three/addons/libs/meshopt_decoder.module.js";
import type { EnvironmentScene } from "./scenes";

type Progress = { loaded: number; listeners: Set<() => void> };
type Download = { progress: Progress; bytes: Promise<ArrayBuffer> };

// Downloaded files stay in memory for the page's life (about 1–1.5 MB per scene), so switching back to
// a scene skips the network. Downloads are shared and never aborted: a scene left mid-download still
// lands in the cache. Failed downloads are dropped so the next attempt retries.
const downloads = new Map<string, Download>();

function download(url: string): Download {
  const cached = downloads.get(url);
  if (cached) return cached;
  const progress: Progress = { loaded: 0, listeners: new Set() };
  const entry = { progress, bytes: fetchBytes(url, progress) };
  entry.bytes.catch(() => downloads.delete(url));
  downloads.set(url, entry);
  return entry;
}

async function fetchBytes(url: string, progress: Progress) {
  const response = await fetch(url);
  if (!response.ok || !response.body)
    throw new Error(`Environment asset failed: ${response.status}`);
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    progress.loaded += value.byteLength;
    progress.listeners.forEach((listener) => listener());
  }
  const bytes = new Uint8Array(progress.loaded);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes.buffer;
}

export type LoadedEnvironment = {
  /** Skybox and furniture, ready to add to the scene. */
  root: Group;
  /** Equirectangular HDR for `scene.environment`. */
  lighting: Texture;
  dispose: () => void;
};

/**
 * Downloads the scene's three files with byte progress, then builds it. Resolves `null` when
 * `signal` aborts first, so a scene the user already left is never built or uploaded to the GPU.
 */
export async function loadEnvironment(
  scene: EnvironmentScene,
  signal: AbortSignal,
  onProgress: (loaded: number, total: number) => void,
): Promise<LoadedEnvironment | null> {
  const files = [scene.backplate, scene.lighting, scene.props].map(download);
  const report = () =>
    onProgress(
      files.reduce((sum, file) => sum + file.progress.loaded, 0),
      scene.bytes,
    );
  files.forEach((file) => file.progress.listeners.add(report));
  try {
    report();
    const [backplate, lighting, props] = await Promise.all(
      files.map((file) => file.bytes),
    );
    if (signal.aborted) return null;
    const parts = await Promise.all([
      decodeTexture(backplate, new TextureLoader()),
      decodeTexture(lighting, new HDRLoader()),
      decodeProps(props),
    ]);
    const built = assembleEnvironment(scene, ...parts);
    if (signal.aborted) {
      built.dispose();
      return null;
    }
    return built;
  } finally {
    files.forEach((file) => file.progress.listeners.delete(report));
  }
}

async function decodeTexture(
  bytes: ArrayBuffer,
  loader: { loadAsync: (url: string) => Promise<Texture> },
) {
  const url = URL.createObjectURL(new Blob([bytes]));
  try {
    return await loader.loadAsync(url);
  } finally {
    URL.revokeObjectURL(url);
  }
}

async function decodeProps(bytes: ArrayBuffer) {
  const loader = new GLTFLoader();
  loader.setMeshoptDecoder(MeshoptDecoder);
  return (await loader.parseAsync(bytes, "")).scene;
}

export function assembleEnvironment(
  scene: EnvironmentScene,
  backplate: Texture,
  lighting: Texture,
  props: Object3D,
): LoadedEnvironment {
  backplate.colorSpace = SRGBColorSpace;
  backplate.mapping = EquirectangularReflectionMapping;
  // The camera only ever magnifies the photo, so mipmaps would cost a third more GPU memory for nothing.
  backplate.generateMipmaps = false;
  backplate.minFilter = LinearFilter;
  lighting.mapping = EquirectangularReflectionMapping;

  const sky = new GroundedSkybox(backplate, scene.photoHeight, 40, 96);
  // The photo is already tone-mapped; mapping it again would wash it out.
  (sky.material as Material).toneMapped = false;
  sky.position.y = scene.photoHeight;
  sky.rotation.y = scene.rotation;
  sky.name = "environment-backplate";

  props.position.set(...scene.prop.position);
  props.rotation.y = scene.prop.rotationY;
  props.scale.set(...scene.prop.scale);
  props.name = "environment-props";

  const root = new Group();
  root.name = "environment";
  root.add(sky, props);
  return {
    root,
    lighting,
    dispose: () => {
      root.removeFromParent();
      disposeObject(root);
      lighting.dispose();
    },
  };
}

/** Frees every geometry, material and texture under `root` (the GPU copies go with them). */
export function disposeObject(root: Object3D) {
  root.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    object.geometry.dispose();
    const materials: Material[] = Array.isArray(object.material)
      ? object.material
      : [object.material];
    for (const material of materials) {
      for (const value of Object.values(material))
        if (value instanceof Texture) value.dispose();
      material.dispose();
    }
  });
}
