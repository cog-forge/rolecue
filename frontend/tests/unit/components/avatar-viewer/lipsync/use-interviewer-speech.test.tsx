import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import fixture from "@tests/fixtures/interviewer-turn-timelines.json";
import { useInterviewerSpeech } from "@/components/avatar-viewer/lipsync/use-interviewer-speech";
import { createInterviewerMouth } from "@/components/avatar-viewer/lipsync/interviewer-mouth";

vi.mock(
  "@/components/avatar-viewer/lipsync/interviewer-mouth",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/components/avatar-viewer/lipsync/interviewer-mouth")
      >();
    return {
      ...actual,
      createInterviewerMouth: vi.fn(actual.createInterviewerMouth),
    };
  },
);

function playingAudio() {
  const audio = document.createElement("audio");
  Object.defineProperty(audio, "paused", { get: () => false });
  Object.defineProperty(audio, "currentTime", { get: () => 1.7 });
  return audio;
}

describe("useInterviewerSpeech", () => {
  it("keeps one stable mouth that follows the current turn and disposes replaced ones", () => {
    const create = vi.mocked(createInterviewerMouth);
    const audio = playingAudio();
    const first = { ...fixture };
    const { result, rerender, unmount } = renderHook(
      ({ turn }) => useInterviewerSpeech(audio, turn),
      { initialProps: { turn: first as typeof first | null } },
    );
    const stable = result.current.mouth;
    expect(stable.sample()?.jawOpen).toBeCloseTo(0.35, 2);
    expect(result.current.speech).toEqual({
      audio,
      cues: [],
      words: fixture.words,
      text: fixture.text,
    });
    const firstMouth = create.mock.results.at(-1)!.value;
    const disposeFirst = vi.spyOn(firstMouth, "dispose");

    // A reconnect re-sends the same turn as a new object: fresh mouth, same bridge.
    rerender({ turn: { ...fixture } });
    expect(disposeFirst).toHaveBeenCalled();
    expect(result.current.mouth).toBe(stable);
    const secondMouth = create.mock.results.at(-1)!.value;
    const disposeSecond = vi.spyOn(secondMouth, "dispose");

    rerender({ turn: null });
    expect(disposeSecond).toHaveBeenCalled();
    expect(stable.sample()).toBeNull();
    expect(result.current.speech).toBeUndefined();
    unmount();
  });
});
