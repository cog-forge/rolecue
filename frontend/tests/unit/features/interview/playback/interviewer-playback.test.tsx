import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  InterviewerPlaybackProvider,
  createInterviewerPlayback,
  useInterviewerPlayback,
  useInterviewerPlaybackController,
} from "@/features/interview/playback/interviewer-playback";
import { textTurn } from "../transport/fake-socket";

class FakeAudio {
  src = "";
  ended = false;
  onplaying: (() => void) | null = null;
  onended: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onpause: (() => void) | null = null;
  pause = vi.fn();
  load = vi.fn();
  playResult: Promise<void> = Promise.resolve();
  play = vi.fn(() => this.playResult);
  removeAttribute(name: string) {
    if (name === "src") this.src = "";
  }
}
let elements: FakeAudio[] = [];
const createAudio = () => {
  const element = new FakeAudio();
  elements.push(element);
  return element as unknown as HTMLAudioElement;
};
const audioTurn = (position: number, data = "SUQzBA==") =>
  textTurn(position, {
    audio: { mime_type: "audio/mpeg", data },
    audio_status: "ready",
  });
let urls = 0;
const createObjectURL = vi.fn(() => `blob:turn-${++urls}`);
const revokeObjectURL = vi.fn();

beforeEach(() => {
  elements = [];
  urls = 0;
  vi.stubGlobal(
    "URL",
    class extends URL {
      static createObjectURL = createObjectURL;
      static revokeObjectURL = revokeObjectURL;
    },
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  createObjectURL.mockClear();
  revokeObjectURL.mockClear();
});

describe("createInterviewerPlayback", () => {
  it.each(["unavailable", "no_voice"] as const)(
    "a %s turn is shown as text and finishes at once",
    (status) => {
      const playback = createInterviewerPlayback(createAudio);
      const onDone = vi.fn();
      const turn = textTurn(1, { audio_status: status });
      playback.play(turn, onDone);
      expect(onDone).toHaveBeenCalledTimes(1);
      expect(elements).toHaveLength(0);
      expect(playback.getSnapshot()).toMatchObject({
        turn,
        playing: false,
        status: "ended",
        audio: null,
      });
    },
  );

  it("plays turn audio through one owned element and finishes once", async () => {
    const playback = createInterviewerPlayback(createAudio);
    const onDone = vi.fn();
    playback.play(audioTurn(1), onDone);
    const [element] = elements;
    expect(element.src).toBe("blob:turn-1");
    expect(element.play).toHaveBeenCalledTimes(1);
    expect(playback.getSnapshot()).toMatchObject({
      status: "loading",
      playing: false,
    });
    element.onplaying?.();
    expect(playback.getSnapshot()).toMatchObject({
      status: "playing",
      playing: true,
      audio: element,
    });
    element.ended = true;
    element.onended?.();
    element.onended?.();
    expect(onDone).toHaveBeenCalledTimes(1);
    expect(playback.getSnapshot()).toMatchObject({
      status: "ended",
      playing: false,
    });
    playback.play(audioTurn(2), vi.fn());
    expect(elements).toHaveLength(1);
  });

  it("a new turn silences the previous one and releases its URL", () => {
    const playback = createInterviewerPlayback(createAudio);
    const first = vi.fn();
    playback.play(audioTurn(1), first);
    const [element] = elements;
    const staleEnded = element.onended;
    playback.play(audioTurn(2), vi.fn());
    expect(element.pause).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:turn-1");
    expect(element.src).toBe("blob:turn-2");
    staleEnded?.();
    expect(first).not.toHaveBeenCalled();
    expect(playback.getSnapshot().turn?.position).toBe(2);
  });

  it("asks for a tap when autoplay is refused, and resumes from it", async () => {
    const refusing = () => {
      const element = createAudio();
      const refused = Promise.reject(
        new DOMException("Autoplay", "NotAllowedError"),
      );
      refused.catch(() => {});
      elements[0].playResult = refused;
      return element;
    };
    const playback = createInterviewerPlayback(refusing);
    const onDone = vi.fn();
    playback.play(audioTurn(1), onDone);
    await vi.waitFor(() =>
      expect(playback.getSnapshot().status).toBe("blocked"),
    );
    expect(onDone).not.toHaveBeenCalled();
    const [element] = elements;
    element.playResult = Promise.resolve();
    playback.resume();
    expect(element.play).toHaveBeenCalledTimes(2);
    expect(playback.getSnapshot().status).toBe("loading");
    element.onplaying?.();
    expect(playback.getSnapshot().playing).toBe(true);
    element.onended?.();
    expect(onDone).toHaveBeenCalledTimes(1);
  });

  it("other play() failures and media errors still continue the interview", async () => {
    const playback = createInterviewerPlayback(createAudio);
    const failed = vi.fn();
    playback.play(audioTurn(1), failed);
    elements[0].onerror?.();
    expect(failed).toHaveBeenCalledTimes(1);

    const rejected = vi.fn();
    elements[0].playResult = Promise.reject(new Error("decode"));
    playback.play(audioTurn(2), rejected);
    await vi.waitFor(() => expect(rejected).toHaveBeenCalledTimes(1));

    const undecodable = vi.fn();
    playback.play(audioTurn(3, "@@@"), undecodable);
    expect(undecodable).toHaveBeenCalledTimes(1);
  });

  it("stop() silences without finishing, and keeps the caption turn", () => {
    const playback = createInterviewerPlayback(createAudio);
    const onDone = vi.fn();
    playback.play(audioTurn(1), onDone);
    const [element] = elements;
    element.onplaying?.();
    playback.stop();
    expect(element.pause).toHaveBeenCalled();
    expect(element.load).toHaveBeenCalled();
    expect(element.src).toBe("");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:turn-1");
    expect(playback.getSnapshot()).toMatchObject({
      playing: false,
      status: "ended",
    });
    expect(playback.getSnapshot().turn?.position).toBe(1);
    element.onended?.();
    expect(onDone).not.toHaveBeenCalled();
  });

  it("dispose() releases everything and returns to idle", () => {
    const playback = createInterviewerPlayback(createAudio);
    playback.play(audioTurn(1), vi.fn());
    playback.dispose();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:turn-1");
    expect(playback.getSnapshot()).toEqual({
      audio: null,
      turn: null,
      playing: false,
      status: "idle",
    });
  });
});

describe("useInterviewerPlayback", () => {
  it("exposes exactly the element, the current turn and whether it plays", () => {
    const seen: unknown[] = [];
    let controller: ReturnType<typeof useInterviewerPlaybackController> | null =
      null;
    function Probe() {
      seen.push(useInterviewerPlayback());
      controller = useInterviewerPlaybackController();
      return null;
    }
    render(
      <InterviewerPlaybackProvider>
        <Probe />
      </InterviewerPlaybackProvider>,
    );
    expect(seen.at(-1)).toEqual({ audio: null, turn: null, playing: false });
    const turn = textTurn(1);
    act(() => controller?.play(turn, () => {}));
    expect(seen.at(-1)).toEqual({ audio: null, turn, playing: false });
    expect(Object.keys(seen.at(-1) ?? {}).sort()).toEqual([
      "audio",
      "playing",
      "turn",
    ]);
  });

  it("returns the idle view outside a provider", () => {
    let value: unknown = null;
    function Probe() {
      value = useInterviewerPlayback();
      return null;
    }
    render(<Probe />);
    expect(value).toEqual({ audio: null, turn: null, playing: false });
  });
});
