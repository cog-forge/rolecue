import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  BoxGeometry,
  DataTexture,
  Group,
  Mesh,
  MeshBasicMaterial,
  MeshStandardMaterial,
  Scene,
  Texture,
} from "three";
import type { EnvironmentScene } from "@/components/avatar-viewer/environment/scenes";

const mocks = vi.hoisted(() => ({ decodes: 0, parses: 0 }));

// jsdom can't decode images, HDRs or GLBs; these stand-ins return real three objects.
vi.mock("three", async (importOriginal) => {
  const three = await importOriginal<typeof import("three")>();
  return {
    ...three,
    TextureLoader: class {
      async loadAsync() {
        mocks.decodes++;
        return new three.Texture();
      }
    },
  };
});
vi.mock("three/addons/loaders/HDRLoader.js", async () => {
  const three = await vi.importActual<typeof import("three")>("three");
  return {
    HDRLoader: class {
      async loadAsync() {
        mocks.decodes++;
        return new three.DataTexture();
      }
    },
  };
});
vi.mock("three/addons/loaders/GLTFLoader.js", async () => {
  const three = await vi.importActual<typeof import("three")>("three");
  return {
    GLTFLoader: class {
      setMeshoptDecoder() {}
      async parseAsync() {
        mocks.parses++;
        const scene = new three.Group();
        scene.add(
          new three.Mesh(
            new three.BoxGeometry(),
            new three.MeshStandardMaterial({ map: new three.Texture() }),
          ),
        );
        return { scene };
      }
    },
  };
});
vi.mock("three/addons/libs/meshopt_decoder.module.js", () => ({
  MeshoptDecoder: {},
}));

const scene: EnvironmentScene = {
  backplate: "/environments/test/backplate.webp",
  lighting: "/environments/test/lighting.hdr",
  props: "/environments/test/props.glb",
  bytes: 600,
  still: "/environments/test/still.webp",
  thumbnail: "/environments/test/thumb.webp",
  rotation: 1.2,
  photoHeight: 1.6,
  prop: { position: [0, 0, 0.8], rotationY: 0.3, scale: [1, 1.3, 1] },
  light: {
    sky: "#fff",
    ground: "#777",
    ambient: 0.7,
    key: "#fff",
    keyIntensity: 1.2,
    rim: "#fff",
    rimIntensity: 1,
  },
};

let fetches: string[];
let failing: Set<string>;

async function load() {
  vi.resetModules();
  return import("@/components/avatar-viewer/environment/load-environment");
}

beforeEach(() => {
  mocks.decodes = 0;
  mocks.parses = 0;
  fetches = [];
  failing = new Set();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      fetches.push(url);
      if (failing.has(url)) return new Response("missing", { status: 404 });
      return new Response(new Uint8Array(200));
    }),
  );
  Object.assign(URL, {
    createObjectURL: vi.fn(() => "blob:test"),
    revokeObjectURL: vi.fn(),
  });
});
afterEach(() => vi.unstubAllGlobals());

describe("loadEnvironment", () => {
  it("downloads the three files with byte progress and builds the room around the interviewer", async () => {
    const { loadEnvironment } = await load();
    const progress: number[] = [];
    const built = await loadEnvironment(
      scene,
      new AbortController().signal,
      (loaded, total) => progress.push(loaded / total),
    );
    expect(built).not.toBeNull();
    if (!built) return;
    expect(fetches.sort()).toEqual(
      [scene.backplate, scene.lighting, scene.props].sort(),
    );
    expect(progress.at(-1)).toBe(1);
    expect([...progress].sort((a, b) => a - b)).toEqual(progress);

    const sky = built.root.getObjectByName("environment-backplate");
    const props = built.root.getObjectByName("environment-props");
    expect(sky?.position.y).toBe(1.6);
    expect(sky?.rotation.y).toBe(1.2);
    const skyMaterial = (sky as Mesh).material as MeshBasicMaterial;
    expect(skyMaterial.toneMapped).toBe(false);
    expect(skyMaterial.map?.generateMipmaps).toBe(false);
    expect(props?.position.toArray()).toEqual([0, 0, 0.8]);
    expect(props?.scale.toArray()).toEqual([1, 1.3, 1]);
    expect(props?.rotation.y).toBe(0.3);
    expect(built.lighting).toBeInstanceOf(DataTexture);
  });

  it("serves a scene chosen again from the cache instead of the network", async () => {
    const { loadEnvironment } = await load();
    const signal = new AbortController().signal;
    await loadEnvironment(scene, signal, () => {});
    const progress: number[] = [];
    await loadEnvironment(scene, signal, (loaded, total) =>
      progress.push(loaded / total),
    );
    expect(fetches).toHaveLength(3);
    expect(progress).toEqual([1]);
  });

  it("never decodes or uploads a scene the user already left", async () => {
    const { loadEnvironment } = await load();
    const abort = new AbortController();
    const pending = loadEnvironment(scene, abort.signal, () => {});
    abort.abort();
    await expect(pending).resolves.toBeNull();
    expect(mocks.decodes).toBe(0);
    expect(mocks.parses).toBe(0);
  });

  it("rejects when a file fails and downloads it again on the next attempt", async () => {
    const { loadEnvironment } = await load();
    const signal = new AbortController().signal;
    failing.add(scene.props);
    await expect(loadEnvironment(scene, signal, () => {})).rejects.toThrow(
      "404",
    );
    failing.clear();
    await expect(
      loadEnvironment(scene, signal, () => {}),
    ).resolves.not.toBeNull();
    expect(fetches.filter((url) => url === scene.props)).toHaveLength(2);
    expect(fetches.filter((url) => url === scene.backplate)).toHaveLength(1);
  });
});

describe("environment disposal", () => {
  it("removes the room from the scene and frees every geometry, material and texture, including the light", async () => {
    const { assembleEnvironment } = await load();
    const backplate = new Texture();
    const lighting = new DataTexture();
    const propTexture = new Texture();
    const geometry = new BoxGeometry();
    const material = new MeshStandardMaterial({ map: propTexture });
    const props = new Group().add(new Mesh(geometry, material));
    const built = assembleEnvironment(scene, backplate, lighting, props);
    const sky = built.root.getObjectByName("environment-backplate") as Mesh;
    const spies = [
      backplate,
      lighting,
      propTexture,
      geometry,
      material,
      sky.geometry,
      sky.material as MeshBasicMaterial,
    ].map((resource) => vi.spyOn(resource, "dispose"));
    const stage = new Scene();
    stage.add(built.root);

    built.dispose();

    expect(stage.children).toHaveLength(0);
    for (const spy of spies) expect(spy).toHaveBeenCalled();
  });
});
