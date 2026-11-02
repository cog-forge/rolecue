import type { TransportEnd } from "./interview-transport";

export type SessionCheck =
  "active" | "completed" | "denied" | "not-found" | "unreachable";
export type CloseDecision =
  | { action: "reconnect" }
  | { action: "check-session" }
  | { action: "end"; end: TransportEnd };

/** docs/interview-ws-protocol.md §7. Codes the table does not name are checked like a lost connection. */
export function closeDecision(code: number): CloseDecision {
  switch (code) {
    case 1000:
      return { action: "end", end: { reason: "completed" } };
    case 1001:
    case 1011:
    case 1012:
    case 1013:
      return { action: "reconnect" };
    case 1009:
    case 4000:
      return { action: "end", end: { reason: "protocol" } };
    case 4001:
      return { action: "end", end: { reason: "superseded" } };
    case 4003:
      return { action: "end", end: { reason: "not-eligible" } };
    default:
      return { action: "check-session" };
  }
}

/** §7, close 1006: GET /interviews/{id} tells a dropped connection from a refused one. */
export function sessionCheckDecision(check: SessionCheck): CloseDecision {
  switch (check) {
    case "active":
    case "unreachable":
      return { action: "reconnect" };
    case "completed":
      return { action: "end", end: { reason: "completed" } };
    case "denied":
      return { action: "end", end: { reason: "denied" } };
    case "not-found":
      return { action: "end", end: { reason: "not-found" } };
  }
}

export const RECONNECT_BASE_MS = 500;
export const RECONNECT_MAX_MS = 15_000;
export const RECONNECT_MAX_ATTEMPTS = 10;

/** Exponential backoff with equal jitter: half the window is fixed, half random. `attempt` starts at 0. */
export function reconnectDelay(attempt: number, random: () => number) {
  const window = Math.min(
    RECONNECT_MAX_MS,
    RECONNECT_BASE_MS * 2 ** Math.max(0, attempt),
  );
  const jitter = Math.min(1, Math.max(0, random()));
  return Math.round(window / 2 + (window / 2) * jitter);
}
