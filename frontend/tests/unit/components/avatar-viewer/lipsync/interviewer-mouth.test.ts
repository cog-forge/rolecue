import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import fixture from "@tests/fixtures/interviewer-turn-timelines.json";
import {
  createInterviewerMouth,
  createMouthForSource,
  pickMouthSource,
} from "@/components/avatar-viewer/lipsync/interviewer-mouth";
import {
  amplitudeToMouth,
  VowelProjector,
} from "@/components/avatar-viewer/lipsync/audio-analysis";
import { MouthSmoother } from "@/components/avatar-viewer/lipsync/smoother";
import type { InterviewerTimelines } from "@/components/avatar-viewer/lipsync/timelines";

const lipsync = vi.hoisted(() => ({
  node: null as null | {
    weights: Record<string, number>;
    volume: number;
    connect: () => void;
    disconnect: () => void;
  },
  fail: false,
}));
vi.mock("wlipsync", () => ({
  parseBinaryProfile: () => ({}),
  createWLipSyncNode: async () => {
    if (lipsync.fail) throw new Error("no AudioWorklet");
    return lipsync.node;
  },
}));

type MediaState = {
  currentTime: number;
  paused: boolean;
  ended: boolean;
  currentSrc: string;
  playbackRate: number;
};
function fakeAudio(init: Partial<MediaState> = {}) {
  const audio = document.createElement("audio");
  const state: MediaState = {
    currentTime: 0,
    paused: false,
    ended: false,
    currentSrc: "blob:turn-1",
    playbackRate: 1,
    ...init,
  };
  for (const key of Object.keys(state) as (keyof MediaState)[])
    Object.defineProperty(audio, key, {
      configurable: true,
      get: () => state[key],
    });
  return { audio, state };
}

let now = 0;
const advance = (ms: number, state: MediaState) => {
  now += ms;
  state.currentTime += ms / 1000;
};
const empty: InterviewerTimelines = {
  visemes: [],
  blend_shapes: [],
  words: [],
};

class FakeNode {
  connect = vi.fn();
  disconnect = vi.fn();
}
let analyserLevel = 0;
class FakeAnalyser extends FakeNode {
  fftSize = 2048;
  getFloatTimeDomainData(buffer: Float32Array) {
    buffer.fill(analyserLevel);
  }
}
const contexts: FakeContext[] = [];
class FakeContext {
  state: AudioContextState = "suspended";
  destination = new FakeNode();
  source = new FakeNode();
  analyser = new FakeAnalyser();
  constructor() {
    contexts.push(this);
  }
  resume = vi.fn(async () => {
    this.state = "running";
  });
  createMediaElementSource = vi.fn(() => this.source);
  createAnalyser = vi.fn(() => this.analyser);
}
const activation = { hasBeenActive: false };
const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

beforeEach(() => {
  now = 0;
  analyserLevel = 0;
  contexts.length = 0;
  activation.hasBeenActive = false;
  lipsync.fail = false;
  lipsync.node = Object.assign(new FakeNode(), {
    weights: {},
    volume: 0,
  });
  vi.spyOn(performance, "now").mockImplementation(() => now);
  vi.stubGlobal("AudioContext", FakeContext);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(new ArrayBuffer(8))),
  );
  Object.defineProperty(navigator, "userActivation", {
    configurable: true,
    value: activation,
  });
});
afterEach(() => {
  vi.unstubAllGlobals();
});

describe("source selection", () => {
  it("prefers blend shapes, then viseme IDs, then audio analysis", () => {
    expect(pickMouthSource(fixture)).toBe("blend-shapes");
    expect(pickMouthSource({ ...fixture, blend_shapes: [] })).toBe("visemes");
    expect(pickMouthSource(empty)).toBe("wlipsync");
  });
  it("reports which source a turn uses", () => {
    const { audio } = fakeAudio();
    const sdk = createMouthForSource(audio, fixture, "auto");
    const rest = createMouthForSource(audio, empty, "auto");
    expect(sdk.status()).toEqual({ source: "blend-shapes", state: "ready" });
    expect(rest.status()).toEqual({ source: "wlipsync", state: "waiting" });
    sdk.dispose();
    rest.dispose();
  });
});

describe("createInterviewerMouth", () => {
  it("follows audio.currentTime on the server timeline", () => {
    const { audio, state } = fakeAudio({ currentTime: 1.7 });
    const mouth = createInterviewerMouth(audio, fixture);
    const vowel = mouth.sample();
    expect(vowel?.jawOpen).toBeCloseTo(0.35, 2);
    advance(1700, state);
    const bilabial = mouth.sample();
    expect(bilabial?.mouthClose).toBeCloseTo(0.441, 2);
    expect(Object.keys(bilabial ?? {}).some((k) => k.startsWith("eye"))).toBe(
      false,
    );
    mouth.dispose();
  });
  it("returns null when paused or ended so the director releases the mouth", () => {
    const { audio, state } = fakeAudio({ currentTime: 1.7, paused: true });
    const mouth = createInterviewerMouth(audio, fixture);
    expect(mouth.sample()).toBeNull();
    state.paused = false;
    expect(mouth.sample()).not.toBeNull();
    state.ended = true;
    expect(mouth.sample()).toBeNull();
    mouth.dispose();
  });
  it("is still during silence instead of flapping", () => {
    const { audio, state } = fakeAudio({ currentTime: 0.01 });
    const mouth = createMouthForSource(audio, fixture, "visemes");
    expect(mouth.sample()).toEqual({});
    state.currentTime = 6;
    expect(mouth.sample()).toEqual({});
    mouth.dispose();
  });
  it("lands on the timeline after a seek instead of easing from stale weights", () => {
    const { audio, state } = fakeAudio({ currentTime: 3.425 });
    const mouth = createMouthForSource(audio, fixture, "visemes");
    expect(mouth.sample()?.viseme_PP).toBeGreaterThan(0.95);
    // Seek back into silence: the bilabial must not linger.
    now += 16;
    state.currentTime = 0.01;
    expect(mouth.sample()?.viseme_PP).toBeUndefined();
    mouth.dispose();
  });
  it("ignores a new turn loaded into the same element", () => {
    const { audio, state } = fakeAudio({ currentTime: 1.7 });
    const mouth = createInterviewerMouth(audio, fixture);
    expect(mouth.sample()).not.toBeNull();
    state.currentSrc = "blob:turn-2";
    expect(mouth.sample()).toBeNull();
    mouth.dispose();
  });
  it("stays released after dispose", () => {
    const { audio } = fakeAudio({ currentTime: 1.7 });
    const mouth = createInterviewerMouth(audio, fixture);
    mouth.dispose();
    mouth.dispose();
    expect(mouth.sample()).toBeNull();
  });
});

describe("audio analysis", () => {
  it("waits for a user gesture before touching Web Audio", async () => {
    const add = vi.spyOn(window, "addEventListener");
    const remove = vi.spyOn(window, "removeEventListener");
    const { audio } = fakeAudio();
    const mouth = createMouthForSource(audio, empty, "amplitude");
    expect(mouth.sample()).toBeNull();
    expect(contexts).toHaveLength(0);
    activation.hasBeenActive = true;
    window.dispatchEvent(new Event("pointerdown"));
    await flush();
    expect(contexts).toHaveLength(1);
    const context = contexts[0];
    expect(context.resume).toHaveBeenCalled();
    // The element is rerouted only once the context runs, and keeps reaching the speakers.
    expect(context.source.connect).toHaveBeenCalledWith(context.destination);
    expect(context.source.connect).toHaveBeenCalledWith(context.analyser);
    expect(mouth.status()).toEqual({ source: "amplitude", state: "ready" });
    const gestures = ["pointerdown", "keydown", "touchend"];
    expect(add.mock.calls.map(([type]) => type)).toEqual(gestures);
    expect(new Set(remove.mock.calls.map(([type]) => type))).toEqual(
      new Set(gestures),
    );
    mouth.dispose();
  });
  it("opens with loudness, stays shut in silence, and detaches on dispose", async () => {
    activation.hasBeenActive = true;
    const { audio, state } = fakeAudio();
    const mouth = createMouthForSource(audio, empty, "amplitude");
    await flush();
    analyserLevel = 0.002;
    expect(mouth.sample()).toEqual({});
    analyserLevel = 0.3;
    advance(16, state);
    expect(mouth.sample()?.aa).toBeGreaterThan(0.2);
    mouth.dispose();
    expect(contexts[0].analyser.disconnect).toHaveBeenCalled();
    expect(contexts[0].source.disconnect).not.toHaveBeenCalled();
  });
  it("reuses one element graph across turns", async () => {
    activation.hasBeenActive = true;
    const { audio } = fakeAudio();
    createMouthForSource(audio, empty, "amplitude").dispose();
    await flush();
    const next = createMouthForSource(audio, empty, "amplitude");
    await flush();
    expect(contexts).toHaveLength(1);
    expect(contexts[0].createMediaElementSource).toHaveBeenCalledTimes(1);
    next.dispose();
  });
  it("drives VRM vowels from wLipSync and disconnects it on interruption", async () => {
    activation.hasBeenActive = true;
    const { audio, state } = fakeAudio();
    const mouth = createMouthForSource(audio, empty, "auto");
    await vi.waitFor(() => expect(mouth.status().state).toBe("ready"));
    lipsync.node!.weights = { A: 0.9, E: 0.3, I: 0.1, O: 0, U: 0, S: 0 };
    lipsync.node!.volume = 0.8;
    const open = mouth.sample();
    expect(open?.aa).toBeGreaterThan(0.5);
    expect(open?.ee).toBeGreaterThan(0);
    state.paused = true;
    expect(mouth.sample()).toBeNull();
    mouth.dispose();
    expect(lipsync.node!.disconnect).toHaveBeenCalled();
  });
  it("falls back to amplitude when wLipSync cannot load", async () => {
    activation.hasBeenActive = true;
    lipsync.fail = true;
    const { audio } = fakeAudio();
    const mouth = createMouthForSource(audio, empty, "auto");
    await vi.waitFor(() => expect(mouth.status().state).toBe("failed"));
    expect(mouth.sample()).toBeNull();
    expect(mouth.status().source).toBe("amplitude");
    await vi.waitFor(() => expect(mouth.status().state).toBe("ready"));
    analyserLevel = 0.3;
    expect(mouth.sample()?.aa).toBeGreaterThan(0.2);
    mouth.dispose();
  });
  it("never creates a graph for an element after dispose", async () => {
    const { audio } = fakeAudio();
    const mouth = createMouthForSource(audio, empty, "wlipsync");
    mouth.dispose();
    activation.hasBeenActive = true;
    window.dispatchEvent(new Event("keydown"));
    await flush();
    expect(contexts).toHaveLength(0);
  });
});

describe("analysis shaping", () => {
  it("gates silence with a short hold and keeps winner and runner-up vowels", () => {
    const projector = new VowelProjector();
    const speaking = projector.project({ A: 0.2, O: 0.8, E: 0.4 }, 1, 0.016);
    expect(Object.keys(speaking).sort()).toEqual(["ee", "oh"]);
    expect(speaking.oh).toBeLessThanOrEqual(0.7);
    expect(projector.project({ O: 0.8 }, 0, 0.1)).toEqual(speaking);
    expect(projector.project({ O: 0.8 }, 0, 0.1)).toEqual({});
  });
  it("keeps quiet noise closed on the amplitude path", () => {
    expect(amplitudeToMouth(0.01)).toEqual({});
    expect(amplitudeToMouth(0.5).aa).toBeGreaterThan(0.5);
  });
  it("attacks faster than it releases and settles to rest", () => {
    const smoother = new MouthSmoother(0.02, 0.08);
    expect(smoother.step({ aa: 1 }, 0.02).aa).toBeCloseTo(1 - Math.exp(-1));
    const falling = smoother.step({}, 0.02).aa!;
    expect(falling).toBeGreaterThan(0.4);
    for (let i = 0; i < 60; i++) smoother.step({}, 0.02);
    expect(smoother.step({}, 0.02)).toEqual({});
  });
});
