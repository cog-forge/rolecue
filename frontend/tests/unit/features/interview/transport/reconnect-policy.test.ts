import { describe, expect, it } from "vitest";
import {
  RECONNECT_BASE_MS,
  RECONNECT_MAX_MS,
  closeDecision,
  reconnectDelay,
  sessionCheckDecision,
} from "@/features/interview/transport/reconnect-policy";

describe("closeDecision (§7)", () => {
  it.each([
    [1000, { action: "end", end: { reason: "completed" } }],
    [1001, { action: "reconnect" }],
    [1006, { action: "check-session" }],
    [1009, { action: "end", end: { reason: "protocol" } }],
    [1011, { action: "reconnect" }],
    [4000, { action: "end", end: { reason: "protocol" } }],
    [4001, { action: "end", end: { reason: "superseded" } }],
    [4003, { action: "end", end: { reason: "not-eligible" } }],
    [1005, { action: "check-session" }],
  ])("close %i", (code, decision) => {
    expect(closeDecision(code)).toEqual(decision);
  });
});

describe("sessionCheckDecision", () => {
  it.each([
    ["active", { action: "reconnect" }],
    ["unreachable", { action: "reconnect" }],
    ["completed", { action: "end", end: { reason: "completed" } }],
    ["denied", { action: "end", end: { reason: "denied" } }],
    ["not-found", { action: "end", end: { reason: "not-found" } }],
  ] as const)("%s", (check, decision) => {
    expect(sessionCheckDecision(check)).toEqual(decision);
  });
});

describe("reconnectDelay", () => {
  it("grows exponentially within [window / 2, window]", () => {
    for (const attempt of [0, 1, 2, 3]) {
      const window = RECONNECT_BASE_MS * 2 ** attempt;
      expect(reconnectDelay(attempt, () => 0)).toBe(window / 2);
      expect(reconnectDelay(attempt, () => 1)).toBe(window);
      const middle = reconnectDelay(attempt, () => 0.5);
      expect(middle).toBeGreaterThanOrEqual(window / 2);
      expect(middle).toBeLessThanOrEqual(window);
    }
    expect(reconnectDelay(1, () => 0)).toBe(2 * reconnectDelay(0, () => 0));
  });
  it("is capped and tolerates out-of-range randomness", () => {
    expect(reconnectDelay(30, () => 1)).toBe(RECONNECT_MAX_MS);
    expect(reconnectDelay(30, () => 0)).toBe(RECONNECT_MAX_MS / 2);
    expect(reconnectDelay(0, () => 5)).toBe(RECONNECT_BASE_MS);
    expect(reconnectDelay(0, () => -1)).toBe(RECONNECT_BASE_MS / 2);
  });
});
