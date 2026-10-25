import { describe, expect, it } from "vitest";
import {
  transition,
  type InterviewEvent,
  type InterviewState,
} from "@/features/interview/machine/interview-machine";
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
  it("requires synchronization after pause or connection loss", () => {
    expect(transition("LISTENING", "CONNECTION_LOST")).toBe("RECONNECTING");
    expect(transition(transition("LISTENING", "PAUSE"), "RESUME")).toBe(
      "RECONNECTING",
    );
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
