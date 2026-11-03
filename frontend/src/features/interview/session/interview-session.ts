import {
  reconcile,
  transition,
  type InterviewState,
} from "../machine/interview-machine";
import {
  answerAudioMessages,
  type RecordedAnswer,
} from "../answer/answer-audio";
import type {
  ConnectionStatus,
  InterviewTransport,
  TransportEnd,
} from "../transport/interview-transport";
import { readApiError } from "@/lib/i18n/api-errors";
import type { EndedSession } from "../api/leave-interview";
import {
  MAX_ANSWER_TEXT_LENGTH,
  newCommandId,
  type ClientMessage,
  type EvaluationStatus,
  type InterviewerTurn,
  type ServerMessage,
} from "../transport/protocol";

/** Only real runtime captions: the interviewer's question text and practice transcripts. */
export type TranscriptEntry = {
  id: string;
  speaker: "interviewer" | "candidate";
  text: string;
  partial?: boolean;
};
export interface InterviewSessionView {
  phase: InterviewState;
  connection: ConnectionStatus;
  end: TransportEnd | null;
  sessionType: "practice" | "recruitment" | null;
  position: number | null;
  /** The core question of the open turn (§4.1): progress is "question N of questionCount". */
  questionNumber: number | null;
  questionCount: number | null;
  answeredCount: number;
  turn: InterviewerTurn | null;
  transcript: readonly TranscriptEntry[];
  /** Server error code of the last answer, for localized retry copy. */
  issue: string | null;
  /** An answer is out and the server has not taken it yet. */
  sending: boolean;
  /** The last answer failed on the server side (INTERNAL_ERROR) and can be sent again as is. */
  canResend: boolean;
  evaluationStatus: EvaluationStatus | null;
  /** Leave ends the interview (no pause): submitting while the end request runs, failed when it did
   * not reach the server and the Candidate may try again. */
  leaving: "idle" | "submitting" | "failed";
}
interface PlaybackPort {
  play(turn: InterviewerTurn, onDone: () => void): void;
  stop(): void;
}
/** The answer and the turn it answers (`turnKey`): a position alone can be reused (see `turnKey`). */
type PendingAnswer = { id: string; position: number; turn: string | null } & (
  { kind: "text"; text: string } | { kind: "audio"; audio: RecordedAnswer }
);
/** Errors after which the Candidate answers again; the server is back in listening. */
const answerAgainCodes = new Set([
  "STT_NO_SPEECH",
  "STT_FAILED",
  "ANSWER_TOO_LONG",
  "AUDIO_FORMAT_UNSUPPORTED",
  "AUDIO_TOO_LARGE",
  "AUDIO_CHUNK_ORDER",
  "VALIDATION_ERROR",
  "INVALID_STATE",
  "INTERVIEW_TURN_CONFLICT",
]);
const INITIAL: InterviewSessionView = {
  phase: "IDLE",
  connection: "disconnected",
  end: null,
  sessionType: null,
  position: null,
  questionNumber: null,
  questionCount: null,
  answeredCount: 0,
  turn: null,
  transcript: [],
  issue: null,
  sending: false,
  canResend: false,
  evaluationStatus: null,
  leaving: "idle",
};

/**
 * Identifies an interviewer turn. A position alone is not enough: a follow-up that was never answered is
 * dropped on reconnect, and the next core question takes its position (§4.2).
 */
function turnKey(turn: InterviewerTurn) {
  return `${turn.position}:${turn.kind}:${turn.question_number}`;
}

/** Drives one room from the server's messages. The server decides every phase; this only
 * renders it, plays turns, and reports `turn.played`, and sends answers with ids that are reused only to retry the same answer.
 */
export function createInterviewSession({
  transport,
  playback,
  endSession,
  newId = newCommandId,
}: {
  transport: InterviewTransport<ServerMessage, ClientMessage>;
  playback: PlaybackPort;
  /** Ends the session on the server (POST /interviews/{id}/complete); the room's Leave calls it. */
  endSession: () => Promise<EndedSession>;
  newId?: () => string;
}) {
  let view = INITIAL;
  const listeners = new Set<() => void>();
  // Turns (`turnKey`) that finished playing. Each delivery of a turn is reported with a fresh id: the
  // server remembers accepted ids for the whole stream, so an id reused after a reconnect would only
  // be acknowledged as a duplicate and the session would never move to listening.
  const played = new Set<string>();
  let pending: PendingAnswer | null = null;
  // The pending answer may not have reached the server (send failed or the socket dropped).
  let unconfirmed = false;
  let unsubscribe: (() => void) | null = null;

  function publish(patch: Partial<InterviewSessionView>) {
    view = { ...view, ...patch };
    listeners.forEach((listener) => listener());
  }
  function addEntry(entry: TranscriptEntry) {
    if (view.transcript.some((item) => item.id === entry.id)) return;
    publish({ transcript: [...view.transcript, entry] });
  }
  function sendAll(messages: ClientMessage[]) {
    return messages.every((message) => transport.send(message));
  }
  function sendPending() {
    if (!pending) return;
    const messages: ClientMessage[] =
      pending.kind === "text"
        ? [
            {
              id: pending.id,
              type: "answer.text",
              data: { position: pending.position, text: pending.text },
            },
          ]
        : answerAudioMessages(pending.id, pending.position, pending.audio);
    // While reconnecting the answer waits; it goes out when the server listens again.
    unconfirmed = view.phase !== "LISTENING" || !sendAll(messages);
    publish({ sending: true, canResend: false, issue: null });
  }
  function clearPending(issue: string | null = null) {
    pending = null;
    unconfirmed = false;
    publish({ sending: false, canResend: false, issue });
  }
  function reportPlayed(turn: InterviewerTurn) {
    played.add(turnKey(turn));
    transport.send({
      id: newId(),
      type: "turn.played",
      data: { position: turn.position },
    });
  }
  function enterServerState(
    state: Parameters<typeof reconcile>[1],
    position: number | null,
  ) {
    const phase = reconcile(view.phase, state);
    if (state !== "interviewer_speaking") playback.stop();
    if (pending && position !== null && pending.position !== position)
      clearPending();
    publish({ phase, position: position ?? view.position });
    if (state === "listening" && pending && unconfirmed) sendPending();
  }
  function handle(message: ServerMessage) {
    // While leaving, only the end itself matters: no new turn plays and no answer goes out.
    if (view.leaving === "submitting" && message.type !== "session.completed")
      return;
    switch (message.type) {
      case "session.sync": {
        const data = message.data;
        // The server moved past the pending answer: it was stored, or the session restarted the turn.
        if (pending && data.answered_count >= pending.position) clearPending();
        publish({
          sessionType: data.session_type,
          questionNumber: data.question_number ?? view.questionNumber,
          questionCount: data.question_count,
          answeredCount: data.answered_count,
          evaluationStatus: data.evaluation_status,
        });
        enterServerState(data.state, data.position);
        return;
      }
      case "state":
        if (message.data.state === "transitioning") publish({ sending: false });
        enterServerState(message.data.state, message.data.position);
        return;
      case "interviewer.turn": {
        const turn = message.data;
        // An answer held for another turn at this position (a follow-up dropped on reconnect) is not
        // sent to the core question that replaced it.
        if (pending && pending.turn !== turnKey(turn)) clearPending();
        addEntry({
          id: `question-${turnKey(turn)}`,
          speaker: "interviewer",
          text: turn.text,
        });
        publish({
          turn,
          position: turn.position,
          questionNumber: turn.question_number,
          questionCount: turn.question_count,
          phase: transition(view.phase, "ASSETS_READY"),
        });
        // A turn re-sent after a reconnect that already played is acknowledged without replaying it.
        if (played.has(turnKey(turn))) reportPlayed(turn);
        else playback.play(turn, () => reportPlayed(turn));
        return;
      }
      case "answer.accepted": {
        const data = message.data;
        if (view.sessionType === "practice" && data.transcript !== undefined)
          addEntry({
            id: `answer-${data.position}`,
            speaker: "candidate",
            text: data.transcript,
          });
        if (pending?.position === data.position) clearPending();
        publish({
          answeredCount: Math.max(view.answeredCount, data.answered_count),
        });
        return;
      }
      case "session.completed":
        playback.stop();
        publish({
          evaluationStatus: message.data.evaluation_status,
          phase: reconcile(view.phase, "completed"),
        });
        return;
      case "ack":
        // A remembered answer id: the server already has this answer.
        if (pending && message.reply_to === pending.id) clearPending();
        return;
      case "error": {
        const { code, fatal } = message.data;
        if (fatal || !pending || message.reply_to !== pending.id) return;
        if (code === "INTERNAL_ERROR") {
          unconfirmed = false;
          publish({ sending: false, canResend: true, issue: code });
        } else if (answerAgainCodes.has(code)) clearPending(code);
        return;
      }
      case "pong":
        return;
    }
  }
  function onStatus(connection: ConnectionStatus) {
    const lost = view.connection === "connected" && connection !== "connected";
    publish({ connection });
    if (!lost) return;
    playback.stop();
    if (pending) unconfirmed = true;
    publish({ phase: transition(view.phase, "CONNECTION_LOST") });
  }
  function onEnd(end: TransportEnd) {
    playback.stop();
    const phase =
      end.reason === "completed"
        ? reconcile(view.phase, "completed")
        : end.reason === "superseded" || end.reason === "not-eligible"
          ? transition(view.phase, "TERMINATE")
          : transition(view.phase, "FAIL");
    publish({ end, phase, sending: false });
  }
  /** An answer is taken while listening, and held through a reconnect of the same question. */
  function answerable() {
    return (
      !pending &&
      view.leaving !== "submitting" &&
      view.position !== null &&
      (view.phase === "LISTENING" || view.phase === "RECONNECTING")
    );
  }
  function connect() {
    publish({ phase: transition(view.phase, "START") });
    void transport.connect();
  }

  return {
    getSnapshot: () => view,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    start() {
      if (unsubscribe) return;
      const subscriptions = [
        transport.subscribe(handle),
        transport.subscribeStatus(onStatus),
        transport.subscribeEnd(onEnd),
      ];
      unsubscribe = () => subscriptions.forEach((stop) => stop());
      connect();
    },
    /** Manual reconnect after an end that does not reconnect by itself (4001, gave up, …). */
    retry() {
      if (!view.end || view.end.reason === "completed") return;
      publish({
        end: null,
        issue: null,
        phase: transition(view.phase, "RESET"),
      });
      connect();
    },
    submitText(text: string) {
      const trimmed = text.trim();
      if (!answerable() || !trimmed || trimmed.length > MAX_ANSWER_TEXT_LENGTH)
        return false;
      if (view.position === null) return false;
      pending = {
        id: newId(),
        position: view.position,
        turn: view.turn && turnKey(view.turn),
        kind: "text",
        text: trimmed,
      };
      sendPending();
      return true;
    },
    submitAudio(audio: RecordedAnswer) {
      if (!answerable() || view.position === null || audio.chunks.length === 0)
        return false;
      pending = {
        id: newId(),
        position: view.position,
        turn: view.turn && turnKey(view.turn),
        kind: "audio",
        audio,
      };
      sendPending();
      return true;
    },
    /** Leaves: ends the session on the server now. The answers stored so far are evaluated and the rest
     * stay unanswered; the room then shows the end and opens the report. An answer still in flight is
     * dropped. Resolves true once the session ended. */
    async leave() {
      if (view.leaving === "submitting" || view.phase === "COMPLETED")
        return false;
      pending = null;
      unconfirmed = false;
      playback.stop();
      publish({
        leaving: "submitting",
        sending: false,
        canResend: false,
        issue: null,
      });
      let ended: EndedSession;
      try {
        ended = await endSession();
      } catch (error) {
        if (!unsubscribe) return false;
        // The recruitment window closed meanwhile: the session cannot be ended or continued.
        if (readApiError(error).code === "APPLICATION_NOT_ELIGIBLE") {
          publish({
            leaving: "idle",
            end: { reason: "not-eligible" },
            phase: transition(view.phase, "TERMINATE"),
          });
          transport.disconnect();
          return false;
        }
        publish({ leaving: "failed" });
        return false;
      }
      if (!unsubscribe) return false;
      publish({
        leaving: "idle",
        phase: "COMPLETED",
        end: { reason: "completed" },
        evaluationStatus: ended.evaluation_status,
      });
      transport.disconnect();
      return true;
    },
    /** Sends the failed answer again with the same id (allowed: failed commands are not remembered). */
    resend() {
      if (pending && view.canResend && view.phase === "LISTENING")
        sendPending();
    },
    stop() {
      unsubscribe?.();
      unsubscribe = null;
      transport.disconnect();
      playback.stop();
      pending = null;
      unconfirmed = false;
      view = INITIAL;
    },
  };
}
export type InterviewSession = ReturnType<typeof createInterviewSession>;
