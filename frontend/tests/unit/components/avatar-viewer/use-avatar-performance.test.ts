import { act, renderHook } from "@testing-library/react";
import { expect, it } from "vitest";
import { useAvatarPerformance } from "@/components/avatar-viewer/use-avatar-performance";

it("keeps a stable per-frame source across rerenders, switches providers and clears on detach", () => {
  const { result, rerender } = renderHook(() => useAvatarPerformance());
  const bridge = result.current;
  expect(bridge.mouth.sample()).toBeNull();
  expect(bridge.triggerGesture("missing")).toBe(false);
  let audioTime = 0;
  act(() => bridge.setMouthSource({ sample: () => ({ aa: audioTime }) }));
  audioTime = 0.5;
  expect(bridge.mouth.sample()).toEqual({ aa: 0.5 });
  rerender();
  expect(result.current).toBe(bridge);
  audioTime = 0.8;
  expect(bridge.mouth.sample()).toEqual({ aa: 0.8 });
  act(() => bridge.setMouthSource(null));
  expect(bridge.mouth.sample()).toBeNull();
  act(() => bridge.onDirectorReady(null));
  expect(bridge.triggerGesture("missing")).toBe(false);
});
