import type {
  ConnectionStatus,
  InterviewTransport,
  TransportEnd,
} from "./interview-transport";
import type { ClientMessage, ServerMessage, ServerState } from "./protocol";

type Payload = ServerMessage extends infer Message
  ? Message extends ServerMessage
    ? Omit<Message, "v" | "stream" | "seq">
    : never
  : never;

/** Development and test stand-in for the live server: text-only turns, no network, no audio.
 * Never selected in a production build (see create-room-transport.ts).
 */
export const scriptedQuestions = [
  "Tell me about a project you are proud of.",
  "Describe a time you disagreed with a teammate and how you resolved it.",
  "What would you improve in that project if you started again?",
] as const;

export function createScriptedTransport({
  sessionId,
  delayMs = 600,
}: {
  sessionId: string;
  delayMs?: number;
}): InterviewTransport<ServerMessage, ClientMessage> {
  const listeners = new Set<(message: ServerMessage) => void>();
  const statusListeners = new Set<(status: ConnectionStatus) => void>();
  const endListeners = new Set<(end: TransportEnd) => void>();
  const timers = new Set<ReturnType<typeof setTimeout>>();
  let seq = 0;
  let open = false;
  let position = 1;
  let state: ServerState = "interviewer_speaking";

  function later(run: () => void) {
    const timer = setTimeout(() => {
      timers.delete(timer);
      if (open) run();
    }, delayMs);
    timers.add(timer);
  }
  function emit(message: Payload) {
    const full: ServerMessage = {
      v: 1 as const,
      stream: "scripted",
      seq: ++seq,
      ...message,
    };
    listeners.forEach((listener) => listener(full));
  }
  function setState(next: ServerState, replyTo?: string) {
    state = next;
    emit({
      type: "state",
      reply_to: replyTo,
      data: {
        state: next,
        position:
          next === "transitioning" || next === "completed" ? null : position,
      },
    });
  }
  function turn() {
    emit({
      type: "interviewer.turn",
      data: {
        position,
        question_number: position,
        kind: "core",
        question_count: scriptedQuestions.length,
        text: scriptedQuestions[position - 1],
        audio: null,
        audio_status: "unavailable",
        visemes: [],
        blend_shapes: [],
        words: [],
        cues: [],
      },
    });
  }
  function answer(id: string, transcript: string) {
    setState("processing", id);
    later(() => {
      emit({
        type: "answer.accepted",
        reply_to: id,
        data: {
          position,
          answered_count: position,
          duplicate: false,
          transcript,
        },
      });
      setState("transitioning");
      later(() => {
        if (position === scriptedQuestions.length) {
          setState("completed");
          emit({
            type: "session.completed",
            data: { evaluation_status: "pending" },
          });
          close({ reason: "completed" });
          return;
        }
        position++;
        setState("interviewer_speaking");
        turn();
      });
    });
  }
  function close(end?: TransportEnd) {
    open = false;
    timers.forEach(clearTimeout);
    timers.clear();
    statusListeners.forEach((listener) => listener("disconnected"));
    if (end) endListeners.forEach((listener) => listener(end));
  }
  return {
    connect() {
      if (open) return Promise.resolve();
      open = true;
      statusListeners.forEach((listener) => listener("connecting"));
      later(() => {
        statusListeners.forEach((listener) => listener("connected"));
        state = "interviewer_speaking";
        emit({
          type: "session.sync",
          data: {
            session_id: sessionId,
            session_type: "practice",
            state,
            question_count: scriptedQuestions.length,
            answered_count: position - 1,
            position,
            question_number: position,
            resumed: seq > 0,
            evaluation_status: null,
            server_time: new Date().toISOString(),
          },
        });
        turn();
      });
      return Promise.resolve();
    },
    disconnect: () => close(),
    send(message) {
      if (!open) return false;
      if (message.type === "turn.played" && state === "interviewer_speaking")
        setState("listening", message.id);
      else if (message.type === "answer.text" && state === "listening")
        answer(message.id, message.data.text);
      else if (
        message.type === "answer.audio" &&
        message.data.final &&
        state === "listening"
      )
        answer(message.id, "(Scripted preview: spoken answer received.)");
      return true;
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    subscribeStatus(listener) {
      statusListeners.add(listener);
      return () => statusListeners.delete(listener);
    },
    subscribeEnd(listener) {
      endListeners.add(listener);
      return () => endListeners.delete(listener);
    },
  };
}
