export type InterviewState =
  | "IDLE"
  | "CONNECTING"
  | "ASSET_LOADING"
  | "INTERVIEWER_SPEAKING"
  | "LISTENING"
  | "PROCESSING_ANSWER"
  | "TRANSITIONING"
  | "RECONNECTING"
  | "DEGRADED_2D"
  | "COMPLETED"
  | "TERMINATED_EARLY"
  | "FATAL_ERROR";
export type InterviewEvent =
  | "START"
  | "CONNECTED"
  | "ASSETS_READY"
  | "SPEECH_ENDED"
  | "ANSWER_SUBMITTED"
  | "ANSWER_PROCESSED"
  | "NEXT_QUESTION"
  | "CONNECTION_LOST"
  | "RECONNECTED"
  | "AVATAR_UNAVAILABLE"
  | "CONTINUE_2D"
  | "COMPLETE"
  | "TERMINATE"
  | "FAIL"
  | "RESET";
const transitions: Partial<
  Record<InterviewState, Partial<Record<InterviewEvent, InterviewState>>>
> = {
  IDLE: { START: "CONNECTING" },
  CONNECTING: { CONNECTED: "ASSET_LOADING" },
  ASSET_LOADING: {
    ASSETS_READY: "INTERVIEWER_SPEAKING",
    AVATAR_UNAVAILABLE: "DEGRADED_2D",
  },
  INTERVIEWER_SPEAKING: {
    SPEECH_ENDED: "LISTENING",
    AVATAR_UNAVAILABLE: "DEGRADED_2D",
  },
  LISTENING: { ANSWER_SUBMITTED: "PROCESSING_ANSWER" },
  PROCESSING_ANSWER: { ANSWER_PROCESSED: "TRANSITIONING" },
  TRANSITIONING: {
    NEXT_QUESTION: "INTERVIEWER_SPEAKING",
    COMPLETE: "COMPLETED",
  },
  RECONNECTING: { RECONNECTED: "ASSET_LOADING" },
  DEGRADED_2D: { CONTINUE_2D: "INTERVIEWER_SPEAKING" },
};
const terminalStates: readonly InterviewState[] = [
  "COMPLETED",
  "TERMINATED_EARLY",
  "FATAL_ERROR",
];
/** Local lifecycle foundation, not a wire protocol.
 * There is no pause: leaving ends the interview (docs/interview-ws-protocol.md §3.1), so the room goes
 * straight to COMPLETED. A reconnect always requires synchronization; invalid events are deterministic
 * no-ops.
 * DEGRADED_2D is a fallback acknowledgement gate, not a persistent rendering mode.
 */
export function transition(
  state: InterviewState,
  event: InterviewEvent,
): InterviewState {
  if (terminalStates.includes(state)) return event === "RESET" ? "IDLE" : state;
  if (state !== "IDLE") {
    if (event === "FAIL") return "FATAL_ERROR";
    if (event === "TERMINATE") return "TERMINATED_EARLY";
    if (event === "CONNECTION_LOST") return "RECONNECTING";
  }
  return transitions[state]?.[event] ?? state;
}

/** Server states of the live protocol (docs/interview-ws-protocol.md §3). */
export type ServerPhase =
  | "interviewer_speaking"
  | "listening"
  | "processing"
  | "transitioning"
  | "completed";
const serverPhaseStates: Record<ServerPhase, InterviewState> = {
  interviewer_speaking: "INTERVIEWER_SPEAKING",
  listening: "LISTENING",
  processing: "PROCESSING_ANSWER",
  transitioning: "TRANSITIONING",
  completed: "COMPLETED",
};
/** The server is authoritative: its announced state replaces the local phase, so a reconnect never
 * guesses where the conversation was. A turn announced outside the live TRANSITIONING step (a fresh
 * sync, a reconnect) starts in ASSET_LOADING until ASSETS_READY. Terminal states stay terminal.
 */
export function reconcile(
  state: InterviewState,
  phase: ServerPhase,
): InterviewState {
  if (terminalStates.includes(state)) return state;
  if (phase === "interviewer_speaking")
    return state === "TRANSITIONING" || state === "INTERVIEWER_SPEAKING"
      ? "INTERVIEWER_SPEAKING"
      : "ASSET_LOADING";
  return serverPhaseStates[phase];
}
