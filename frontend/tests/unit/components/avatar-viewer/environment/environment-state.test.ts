import { describe, expect, it } from "vitest";
import {
  environmentLoading,
  environmentReducer,
  type EnvironmentStatus,
} from "@/components/avatar-viewer/environment/environment-state";

describe("environmentReducer", () => {
  it("moves a loading scene forward by byte progress, never backwards and never to 100% before it is built", () => {
    let status = environmentReducer(environmentLoading, {
      type: "progress",
      loaded: 400,
      total: 1000,
    });
    expect(status).toEqual({ state: "loading", progress: 0.4 });
    const same = environmentReducer(status, {
      type: "progress",
      loaded: 300,
      total: 1000,
    });
    expect(same).toBe(status);
    status = environmentReducer(status, {
      type: "progress",
      loaded: 1000,
      total: 1000,
    });
    expect(status).toEqual({ state: "loading", progress: 0.99 });
    expect(environmentReducer(status, { type: "loaded" })).toEqual({
      state: "ready",
    });
  });

  it("ends a failed or slow load in the still-image fallback with its reason", () => {
    expect(environmentReducer(environmentLoading, { type: "failed" })).toEqual({
      state: "fallback",
      reason: "error",
    });
    expect(
      environmentReducer(environmentLoading, { type: "timed-out" }),
    ).toEqual({ state: "fallback", reason: "timeout" });
  });

  it("ignores late events once a scene is ready or has fallen back", () => {
    const ready: EnvironmentStatus = { state: "ready" };
    const timedOut: EnvironmentStatus = {
      state: "fallback",
      reason: "timeout",
    };
    for (const status of [ready, timedOut]) {
      expect(environmentReducer(status, { type: "loaded" })).toBe(status);
      expect(environmentReducer(status, { type: "failed" })).toBe(status);
      expect(
        environmentReducer(status, { type: "progress", loaded: 5, total: 10 }),
      ).toBe(status);
    }
  });

  it("keeps zero progress when the expected size is unknown", () => {
    expect(
      environmentReducer(environmentLoading, {
        type: "progress",
        loaded: 10,
        total: 0,
      }),
    ).toBe(environmentLoading);
  });
});
