import { describe, expect, it } from "vitest";
import {
  reconcile,
  transition,
  type InterviewEvent,
  type InterviewState,
} from "@/features/interview/machine/interview-machine";
import { serverStateSchema } from "@/features/interview/transport/protocol";
describe("interview lifecycle", () => {
  it("progresses through an answer and completion", () => {
    const events: InterviewEvent[] = [
      "START",
      "CONNECTED",
      "ASSETS_READY",
      "SPEECH_ENDED",
      "ANSWER_SUBMITTED",
      "ANSWER_PROCESSED",
      "COMPLETE",
    ];
    expect(events.reduce<InterviewState>(transition, "IDLE")).toBe("COMPLETED");
  });
  it("requires synchronization after a connection loss", () => {
    expect(transition("LISTENING", "CONNECTION_LOST")).toBe("RECONNECTING");
    expect(transition("RECONNECTING", "RECONNECTED")).toBe("ASSET_LOADING");
  });
  it("acknowledges a 2D fallback without fabricating avatar behavior", () => {
    expect(transition("ASSET_LOADING", "AVATAR_UNAVAILABLE")).toBe(
      "DEGRADED_2D",
    );
    expect(transition("DEGRADED_2D", "CONTINUE_2D")).toBe(
      "INTERVIEWER_SPEAKING",
    );
  });
  it("ignores impossible events and protects terminal states", () => {
    expect(transition("IDLE", "ANSWER_SUBMITTED")).toBe("IDLE");
    expect(transition("COMPLETED", "START")).toBe("COMPLETED");
    expect(transition("COMPLETED", "RESET")).toBe("IDLE");
    expect(transition("CONNECTING", "FAIL")).toBe("FATAL_ERROR");
    expect(transition("LISTENING", "TERMINATE")).toBe("TERMINATED_EARLY");
  });
});
describe("server reconciliation", () => {
  it("starts a synced turn in ASSET_LOADING and a live next turn in INTERVIEWER_SPEAKING", () => {
    expect(reconcile("CONNECTING", "interviewer_speaking")).toBe(
      "ASSET_LOADING",
    );
    expect(transition("ASSET_LOADING", "ASSETS_READY")).toBe(
      "INTERVIEWER_SPEAKING",
    );
    expect(reconcile("TRANSITIONING", "interviewer_speaking")).toBe(
      "INTERVIEWER_SPEAKING",
    );
  });
  it.each([
    ["interviewer_speaking", "ASSET_LOADING"],
    ["listening", "LISTENING"],
    ["processing", "PROCESSING_ANSWER"],
    ["transitioning", "TRANSITIONING"],
    ["completed", "COMPLETED"],
  ] as const)(
    "after a reconnect, server %s becomes %s without guessing",
    (phase, state) => {
      expect(reconcile("RECONNECTING", phase)).toBe(state);
    },
  );
  it("has no pause state: the protocol never sends one", () => {
    expect(serverStateSchema.safeParse("paused").success).toBe(false);
  });
  it("lets the server move a failed answer back to listening", () => {
    expect(reconcile("PROCESSING_ANSWER", "listening")).toBe("LISTENING");
  });
  it.each(["COMPLETED", "TERMINATED_EARLY", "FATAL_ERROR"] as const)(
    "keeps %s terminal",
    (state) => {
      expect(reconcile(state, "listening")).toBe(state);
      expect(reconcile(state, "interviewer_speaking")).toBe(state);
    },
  );
});
