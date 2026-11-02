import type { Profile, WLipSyncAudioNode } from "wlipsync";
import type { MouthSample } from "../avatar-director";

export type AnalysisState = "waiting" | "loading" | "ready" | "failed";

/** In-browser analysis of the playing element. `target` is null until the analysis runs. */
export interface AudioDriver {
  readonly state: AnalysisState;
  target: (dt: number) => MouthSample | null;
  dispose: () => void;
}

type AudioGraph = {
  context: AudioContext;
  source: MediaElementAudioSourceNode;
};
const graphs = new WeakMap<HTMLMediaElement, Promise<AudioGraph>>();

export const webAudioSupported = () => typeof AudioContext === "function";

/** Sticky user activation: the browser lets an AudioContext run only after a gesture. */
const userActivated = () =>
  typeof navigator !== "undefined" &&
  navigator.userActivation?.hasBeenActive === true;

/**
 * One graph per media element for the element's lifetime. `createMediaElementSource` works once per
 * element and reroutes its sound through the context, so the element is attached only after the context
 * is running (a suspended context would mute it), stays connected to the speakers, and the context is
 * never closed. Analysis nodes are attached and detached per turn. Anything else that needs Web Audio on
 * the same element must reuse this graph.
 */
export function connectAudioGraph(audio: HTMLMediaElement) {
  let graph = graphs.get(audio);
  if (!graph) {
    graph = (async () => {
      const context = new AudioContext();
      if (context.state !== "running") await context.resume();
      const source = context.createMediaElementSource(audio);
      source.connect(context.destination);
      audio.addEventListener("play", () => {
        if (context.state === "suspended") void context.resume();
      });
      return { context, source };
    })();
    graphs.set(audio, graph);
    graph.catch(() => graphs.delete(audio));
  }
  return graph;
}

const gestures = ["pointerdown", "keydown", "touchend"] as const;

/**
 * Attaches an analysis node lazily: right away when the page already had a user gesture, otherwise on
 * the next one. Nodes are disconnected on dispose; the element keeps playing through the graph.
 */
function createAnalysisDriver<N extends AudioNode>(
  audio: HTMLMediaElement,
  create: (context: AudioContext) => Promise<N>,
  read: (node: N, dt: number) => MouthSample,
): AudioDriver {
  let state: AnalysisState = "waiting";
  let node: N | null = null;
  let disposed = false;
  const removeGestureListeners = () => {
    for (const type of gestures)
      window.removeEventListener(type, start, { capture: true });
  };
  function start() {
    if (state !== "waiting" || disposed) return;
    removeGestureListeners();
    if (!webAudioSupported()) {
      state = "failed";
      return;
    }
    state = "loading";
    connectAudioGraph(audio)
      .then(async (graph) => {
        const created = await create(graph.context);
        if (disposed) return created.disconnect();
        graph.source.connect(created);
        node = created;
        state = "ready";
      })
      .catch(() => {
        if (!disposed) state = "failed";
      });
  }
  if (userActivated()) start();
  else if (typeof window !== "undefined")
    for (const type of gestures)
      window.addEventListener(type, start, { capture: true, passive: true });
  return {
    get state() {
      return state;
    },
    target: (dt) => (node && state === "ready" ? read(node, dt) : null),
    dispose() {
      if (disposed) return;
      disposed = true;
      removeGestureListeners();
      node?.disconnect();
      node = null;
    },
  };
}

const rawToVowel: Readonly<Record<string, string>> = {
  A: "aa",
  E: "ee",
  I: "ih",
  O: "oh",
  U: "ou",
  S: "ih",
};

/**
 * wLipSync AEIOU weights → VRM vowels, after AIRI (moeru-ai, MIT, packages/model-driver-lipsync): volume
 * shaping, winner/runner-up selection, and a silence gate with a short hold so the mouth never flaps on
 * noise between words.
 */
export class VowelProjector {
  private silentFor = Number.POSITIVE_INFINITY;
  private last: MouthSample = {};
  project(
    weights: Readonly<Record<string, number>>,
    volume: number,
    dt: number,
  ): MouthSample {
    const amplitude = Math.min(Math.max(volume, 0) * 0.9, 1) ** 0.7;
    const projected = new Map<string, number>();
    for (const [raw, weight] of Object.entries(weights)) {
      const vowel = rawToVowel[raw];
      if (vowel && Number.isFinite(weight))
        projected.set(
          vowel,
          Math.max(projected.get(vowel) ?? 0, weight * amplitude),
        );
    }
    const [winner, runner] = [...projected].sort((a, b) => b[1] - a[1]);
    if (amplitude < 0.04 || !winner || winner[1] < 0.05) {
      this.silentFor += dt;
      if (this.silentFor >= 0.16) this.last = {};
      return this.last;
    }
    this.silentFor = 0;
    const out: Record<string, number> = {
      [winner[0]]: Math.min(0.7, winner[1]),
    };
    if (runner && runner[1] > 0)
      out[runner[0]] = Math.min(0.35, runner[1] * 0.6);
    this.last = out;
    return out;
  }
}

// MIT profile from the wLipSync repository (example/profile.bin); the npm package ships none.
const profileUrl = "/lipsync/wlipsync-profile.bin";
let profile: Promise<Profile> | undefined;

async function createWLipSyncNode(context: AudioContext) {
  // Loaded on demand: wlipsync evaluates AudioWorkletNode at import, so it must never run on the server.
  const lipsync = await import("wlipsync");
  profile ??= fetch(profileUrl)
    .then((response) => {
      if (!response.ok) throw new Error(`wLipSync profile ${response.status}`);
      return response.arrayBuffer();
    })
    .then((buffer) => lipsync.parseBinaryProfile(buffer));
  profile.catch(() => {
    profile = undefined;
  });
  return lipsync.createWLipSyncNode(context, await profile);
}

export function createWLipSyncDriver(audio: HTMLMediaElement): AudioDriver {
  const projector = new VowelProjector();
  return createAnalysisDriver<WLipSyncAudioNode>(
    audio,
    createWLipSyncNode,
    (node, dt) => projector.project(node.weights, node.volume, dt),
  );
}

/**
 * Peak level → open mouth, after Amica (semperai, MIT, src/features/lipSync): a steep sigmoid with a
 * floor, so quiet noise stays closed.
 */
export function amplitudeToMouth(peak: number): MouthSample {
  const level = 1 / (1 + Math.exp(-45 * Math.abs(peak) + 5));
  return level < 0.1 ? {} : { aa: level * 0.6 };
}

export function createAmplitudeDriver(audio: HTMLMediaElement): AudioDriver {
  let buffer: Float32Array<ArrayBuffer> | null = null;
  return createAnalysisDriver<AnalyserNode>(
    audio,
    async (context) => {
      const analyser = context.createAnalyser();
      analyser.fftSize = 2048;
      buffer = new Float32Array(analyser.fftSize);
      return analyser;
    },
    (analyser) => {
      if (!buffer) return {};
      analyser.getFloatTimeDomainData(buffer);
      let peak = 0;
      for (const sample of buffer) peak = Math.max(peak, Math.abs(sample));
      return amplitudeToMouth(peak);
    },
  );
}
