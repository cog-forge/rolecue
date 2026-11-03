import { beforeEach, describe, expect, it, vi } from "vitest";
import { createInterviewSession } from "@/features/interview/session/interview-session";
import type {
  ConnectionStatus,
  InterviewTransport,
  TransportEnd,
} from "@/features/interview/transport/interview-transport";
import {
  parseServerMessage,
  type ClientMessage,
  type InterviewerTurn,
  type ServerMessage,
} from "@/features/interview/transport/protocol";
import { serverMessage, syncData, textTurn } from "../transport/fake-socket";

function fakeTransport() {
  const messages = new Set<(message: ServerMessage) => void>();
  const statuses = new Set<(status: ConnectionStatus) => void>();
  const ends = new Set<(end: TransportEnd) => void>();
  let seq = 0;
  const transport = {
    open: true,
    sent: [] as ClientMessage[],
    connect: vi.fn(async () => {}),
    disconnect: vi.fn(),
    send(message: ClientMessage) {
      if (!transport.open) return false;
      transport.sent.push(message);
      return true;
    },
    subscribe(listener: (message: ServerMessage) => void) {
      messages.add(listener);
      return () => messages.delete(listener);
    },
    subscribeStatus(listener: (status: ConnectionStatus) => void) {
      statuses.add(listener);
      return () => statuses.delete(listener);
    },
    subscribeEnd(listener: (end: TransportEnd) => void) {
      ends.add(listener);
      return () => ends.delete(listener);
    },
    emit(type: string, data: unknown, replyTo?: string) {
      const parsed = parseServerMessage(
        serverMessage(type, data, ++seq, { replyTo }),
      );
      if (!parsed.success) throw new Error(parsed.error);
      messages.forEach((listener) => listener(parsed.message));
    },
    status(status: ConnectionStatus) {
      transport.open = status === "connected";
      statuses.forEach((listener) => listener(status));
    },
    end(end: TransportEnd) {
      transport.open = false;
      statuses.forEach((listener) => listener("disconnected"));
      ends.forEach((listener) => listener(end));
    },
  };
  const typed: InterviewTransport<ServerMessage, ClientMessage> = transport;
  return { transport, typed };
}

function setup(sessionType: "practice" | "recruitment" = "practice") {
  const { transport, typed } = fakeTransport();
  const done: (() => void)[] = [];
  const playback = {
    play: vi.fn((turn: InterviewerTurn, onDone: () => void) => {
      void turn;
      done.push(onDone);
    }),
    stop: vi.fn(),
  };
  let ids = 0;
  const session = createInterviewSession({
    transport: typed,
    playback,
    newId: () => `id-${++ids}`,
  });
  const view = () => session.getSnapshot();
  /** Connect, sync on question `position`, play its turn to the end and listen. */
  function toListening(position = 1, answered = position - 1) {
    transport.status("connected");
    transport.emit(
      "session.sync",
      syncData({
        session_type: sessionType,
        position,
        answered_count: answered,
      }),
    );
    transport.emit("interviewer.turn", textTurn(position));
    done.at(-1)?.();
    transport.emit("state", { state: "listening", position });
  }
  session.start();
  return { session, transport, playback, done, view, toListening };
}
const sentOf = (sent: ClientMessage[], type: ClientMessage["type"]) =>
  sent.filter((message) => message.type === type);

let t: ReturnType<typeof setup>;
beforeEach(() => {
  t = setup();
});

describe("server-driven phases", () => {
  it("follows the server from connect to listening", () => {
    expect(t.view().phase).toBe("CONNECTING");
    expect(t.transport.connect).toHaveBeenCalledTimes(1);
    t.transport.status("connected");
    expect(t.view().phase).toBe("CONNECTING");
    t.transport.emit("session.sync", syncData({ question_count: 8 }));
    expect(t.view()).toMatchObject({
      phase: "ASSET_LOADING",
      sessionType: "practice",
      position: 1,
      questionCount: 8,
    });
    t.transport.emit("interviewer.turn", textTurn(1, { question_count: 8 }));
    expect(t.view().phase).toBe("INTERVIEWER_SPEAKING");
    expect(t.playback.play).toHaveBeenCalledTimes(1);
    expect(t.view().transcript).toEqual([
      { id: "question-1:core:1", speaker: "interviewer", text: "Question 1?" },
    ]);
    expect(t.transport.sent).toEqual([]);
    t.done[0]();
    expect(t.transport.sent).toEqual([
      { id: "id-1", type: "turn.played", data: { position: 1 } },
    ]);
    t.transport.emit("state", { state: "listening", position: 1 });
    expect(t.view().phase).toBe("LISTENING");
  });

  it("walks an answer through processing, transitioning and the next turn", () => {
    t.toListening();
    expect(t.session.submitText("  My answer  ")).toBe(true);
    expect(sentOf(t.transport.sent, "answer.text")).toEqual([
      {
        id: "id-2",
        type: "answer.text",
        data: { position: 1, text: "My answer" },
      },
    ]);
    expect(t.view().sending).toBe(true);
    t.transport.emit("state", { state: "processing", position: 1 }, "id-2");
    expect(t.view().phase).toBe("PROCESSING_ANSWER");
    t.transport.emit(
      "answer.accepted",
      {
        position: 1,
        answered_count: 1,
        duplicate: false,
        transcript: "My answer",
      },
      "id-2",
    );
    expect(t.view()).toMatchObject({ answeredCount: 1, sending: false });
    t.transport.emit("state", { state: "transitioning", position: null });
    expect(t.view().phase).toBe("TRANSITIONING");
    t.transport.emit("state", { state: "interviewer_speaking", position: 2 });
    expect(t.view()).toMatchObject({
      phase: "INTERVIEWER_SPEAKING",
      position: 2,
    });
    t.transport.emit("interviewer.turn", textTurn(2));
    expect(t.playback.play).toHaveBeenCalledTimes(2);
    expect(t.view().transcript.map((entry) => entry.id)).toEqual([
      "question-1:core:1",
      "answer-1",
      "question-2:core:2",
    ]);
  });

  it("answers a follow-up at its own position while progress stays on its core question", () => {
    t.toListening();
    t.session.submitText("Token bucket");
    t.transport.emit(
      "answer.accepted",
      { position: 1, answered_count: 1, duplicate: false },
      "id-2",
    );
    t.transport.emit("state", { state: "transitioning", position: null });
    t.transport.emit("state", { state: "interviewer_speaking", position: 2 });
    t.transport.emit(
      "interviewer.turn",
      textTurn(2, { kind: "follow_up", question_number: 1 }),
    );
    expect(t.view()).toMatchObject({ position: 2, questionNumber: 1 });
    expect(t.view().turn?.kind).toBe("follow_up");
    t.done.at(-1)?.();
    t.transport.emit("state", { state: "listening", position: 2 });
    expect(t.session.submitText("Because of bursts")).toBe(true);
    expect(sentOf(t.transport.sent, "answer.text").at(-1)).toMatchObject({
      data: { position: 2, text: "Because of bursts" },
    });
  });

  it("drops a follow-up lost in a reconnect and plays the core question that takes its position", () => {
    t.toListening();
    t.session.submitText("Token bucket");
    t.transport.emit(
      "answer.accepted",
      { position: 1, answered_count: 1, duplicate: false },
      "id-2",
    );
    t.transport.emit(
      "interviewer.turn",
      textTurn(2, {
        kind: "follow_up",
        question_number: 1,
        text: "Why a bucket?",
      }),
    );
    t.done.at(-1)?.();
    t.transport.emit("state", { state: "listening", position: 2 });
    // The follow-up answer goes out, then the socket drops before it is stored.
    expect(t.session.submitText("Because of bursts")).toBe(true);
    t.transport.status("reconnecting");
    t.transport.status("connected");
    t.transport.emit(
      "session.sync",
      syncData({ position: 2, question_number: 2, answered_count: 1 }),
    );
    const plays = t.playback.play.mock.calls.length;
    t.transport.emit(
      "interviewer.turn",
      textTurn(2, { question_number: 2, text: "Question 2?" }),
    );
    // Same position, another turn: it is played, not skipped as already heard, and the held answer is
    // not sent to it.
    expect(t.playback.play).toHaveBeenCalledTimes(plays + 1);
    expect(t.view()).toMatchObject({ questionNumber: 2, sending: false });
    t.done.at(-1)?.();
    t.transport.emit("state", { state: "listening", position: 2 });
    expect(
      t.transport.sent.filter(
        (message) =>
          message.type === "answer.text" &&
          message.data.text === "Because of bursts",
      ),
    ).toHaveLength(1);
    expect(t.view().transcript.map((entry) => entry.text)).toEqual([
      "Question 1?",
      "Why a bucket?",
      "Question 2?",
    ]);
  });

  it("completes on session.completed", () => {
    t.toListening(3, 2);
    t.transport.emit("state", { state: "transitioning", position: null });
    t.transport.emit("state", { state: "completed", position: null });
    t.transport.emit("session.completed", { evaluation_status: "pending" });
    expect(t.view()).toMatchObject({
      phase: "COMPLETED",
      evaluationStatus: "pending",
    });
    t.transport.end({ reason: "completed" });
    expect(t.view().phase).toBe("COMPLETED");
  });
});

describe("answers", () => {
  it("refuses answers outside listening, empty or too long", () => {
    expect(t.session.submitText("Early")).toBe(false);
    t.toListening();
    expect(t.session.submitText("   ")).toBe(false);
    expect(t.session.submitText("x".repeat(8001))).toBe(false);
    expect(t.session.submitText("x".repeat(8000))).toBe(true);
    expect(t.session.submitText("Second")).toBe(false);
    expect(sentOf(t.transport.sent, "answer.text")).toHaveLength(1);
  });

  it("sends a recording as chunks that share one id", () => {
    t.toListening();
    expect(
      t.session.submitAudio({
        mimeType: "audio/webm;codecs=opus",
        chunks: ["AAAA", "BBBB"],
        bytes: 6,
      }),
    ).toBe(true);
    expect(sentOf(t.transport.sent, "answer.audio")).toEqual([
      {
        id: "id-2",
        type: "answer.audio",
        data: {
          position: 1,
          chunk: 0,
          final: false,
          mime_type: "audio/webm;codecs=opus",
          audio: "AAAA",
        },
      },
      {
        id: "id-2",
        type: "answer.audio",
        data: { position: 1, chunk: 1, final: true, audio: "BBBB" },
      },
    ]);
  });

  it("shows practice transcripts only", () => {
    const recruitment = setup("recruitment");
    recruitment.toListening();
    recruitment.session.submitText("Secret");
    recruitment.transport.emit(
      "answer.accepted",
      { position: 1, answered_count: 1, duplicate: false, transcript: "Leak" },
      "id-2",
    );
    expect(recruitment.view().transcript.map((entry) => entry.speaker)).toEqual(
      ["interviewer"],
    );
    t.toListening();
    t.session.submitText("Mine");
    t.transport.emit(
      "answer.accepted",
      { position: 1, answered_count: 1, duplicate: false, transcript: "Mine" },
      "id-2",
    );
    expect(t.view().transcript.at(-1)).toEqual({
      id: "answer-1",
      speaker: "candidate",
      text: "Mine",
    });
  });

  it("clears the answer with localized-copy codes the Candidate answers again after", () => {
    t.toListening();
    t.session.submitText("Hello");
    t.transport.emit(
      "error",
      { code: "STT_NO_SPEECH", message: "x", retryable: true, fatal: false },
      "id-2",
    );
    t.transport.emit("state", { state: "listening", position: 1 }, "id-2");
    expect(t.view()).toMatchObject({
      issue: "STT_NO_SPEECH",
      sending: false,
      canResend: false,
      phase: "LISTENING",
    });
    expect(sentOf(t.transport.sent, "answer.text")).toHaveLength(1);
    expect(t.session.submitText("Again")).toBe(true);
    expect(t.view().issue).toBeNull();
    expect(sentOf(t.transport.sent, "answer.text").at(-1)?.id).toBe("id-3");
  });

  it("offers to resend with the same id after INTERNAL_ERROR", () => {
    t.toListening();
    t.session.submitText("Hello");
    t.transport.emit(
      "error",
      { code: "INTERNAL_ERROR", message: "x", retryable: true, fatal: false },
      "id-2",
    );
    t.transport.emit("state", { state: "listening", position: 1 }, "id-2");
    expect(t.view()).toMatchObject({ canResend: true, sending: false });
    // No automatic resend.
    expect(sentOf(t.transport.sent, "answer.text")).toHaveLength(1);
    expect(t.session.submitText("Other")).toBe(false);
    t.session.resend();
    const answers = sentOf(t.transport.sent, "answer.text");
    expect(answers).toHaveLength(2);
    expect(answers[1]).toEqual(answers[0]);
    expect(t.view()).toMatchObject({ canResend: false, sending: true });
  });

  it("ignores errors for other commands and fatal errors", () => {
    t.toListening();
    t.session.submitText("Hello");
    t.transport.emit(
      "error",
      { code: "INVALID_STATE", message: "x", retryable: false, fatal: false },
      "someone-else",
    );
    t.transport.emit(
      "error",
      { code: "INTERNAL_ERROR", message: "x", retryable: true, fatal: true },
      "id-2",
    );
    expect(t.view()).toMatchObject({ sending: true, issue: null });
  });

  it("a duplicate ack for the pending answer clears it", () => {
    t.toListening();
    t.session.submitText("Hello");
    t.transport.emit("ack", { duplicate: true }, "id-2");
    expect(t.view().sending).toBe(false);
    expect(t.session.submitText("Next")).toBe(true);
  });
});

describe("reconnect", () => {
  it("stops audio and enters RECONNECTING when the connection drops", () => {
    t.toListening();
    t.playback.stop.mockClear();
    t.transport.status("reconnecting");
    expect(t.view().phase).toBe("RECONNECTING");
    expect(t.playback.stop).toHaveBeenCalled();
  });

  it("reports a re-sent, already played turn with a fresh id and no replay", () => {
    t.toListening();
    t.transport.status("reconnecting");
    t.transport.status("connected");
    expect(t.view().phase).toBe("RECONNECTING");
    t.transport.emit("session.sync", syncData({ resumed: true }));
    expect(t.view().phase).toBe("ASSET_LOADING");
    t.transport.emit("interviewer.turn", textTurn(1));
    expect(t.playback.play).toHaveBeenCalledTimes(1);
    expect(sentOf(t.transport.sent, "turn.played")).toEqual([
      { id: "id-1", type: "turn.played", data: { position: 1 } },
      // The server remembers accepted ids across reconnects; the old id would only be acked.
      { id: "id-2", type: "turn.played", data: { position: 1 } },
    ]);
    expect(t.view().transcript).toHaveLength(1);
  });

  it("replays a turn whose playback was interrupted", () => {
    t.transport.status("connected");
    t.transport.emit("session.sync", syncData());
    t.transport.emit("interviewer.turn", textTurn(1));
    t.transport.status("reconnecting");
    t.transport.status("connected");
    t.transport.emit("session.sync", syncData({ resumed: true }));
    t.transport.emit("interviewer.turn", textTurn(1));
    expect(t.playback.play).toHaveBeenCalledTimes(2);
    expect(t.transport.sent).toEqual([]);
  });

  it("resends an interrupted answer with the same id once the server listens again", () => {
    t.toListening();
    t.session.submitText("Hello");
    t.transport.status("reconnecting");
    t.transport.status("connected");
    t.transport.emit("session.sync", syncData({ resumed: true }));
    t.transport.emit("interviewer.turn", textTurn(1));
    expect(sentOf(t.transport.sent, "answer.text")).toHaveLength(1);
    t.transport.emit("state", { state: "listening", position: 1 });
    const answers = sentOf(t.transport.sent, "answer.text");
    expect(answers).toHaveLength(2);
    expect(answers[1]).toEqual(answers[0]);
  });

  it("drops the pending answer when the sync shows it was stored", () => {
    t.toListening();
    t.session.submitText("Hello");
    t.transport.status("reconnecting");
    t.transport.status("connected");
    t.transport.emit(
      "session.sync",
      syncData({ position: 2, answered_count: 1, resumed: true }),
    );
    expect(t.view()).toMatchObject({ sending: false, position: 2 });
    t.transport.emit("interviewer.turn", textTurn(2));
    t.done.at(-1)?.();
    t.transport.emit("state", { state: "listening", position: 2 });
    expect(sentOf(t.transport.sent, "answer.text")).toHaveLength(1);
    expect(t.session.submitText("Second")).toBe(true);
  });

  it("holds an answer given while reconnecting and sends it when listening", () => {
    t.toListening();
    t.transport.status("reconnecting");
    expect(t.session.submitText("Held")).toBe(true);
    expect(sentOf(t.transport.sent, "answer.text")).toEqual([]);
    expect(t.view().sending).toBe(true);
    t.transport.status("connected");
    t.transport.emit("session.sync", syncData({ resumed: true }));
    t.transport.emit("interviewer.turn", textTurn(1));
    t.transport.emit("state", { state: "listening", position: 1 });
    expect(sentOf(t.transport.sent, "answer.text")).toEqual([
      {
        id: "id-2",
        type: "answer.text",
        data: { position: 1, text: "Held" },
      },
    ]);
  });
});

describe("ends", () => {
  it.each([
    ["superseded", "TERMINATED_EARLY"],
    ["not-eligible", "TERMINATED_EARLY"],
    ["protocol", "FATAL_ERROR"],
    ["denied", "FATAL_ERROR"],
    ["not-found", "FATAL_ERROR"],
    ["gave-up", "FATAL_ERROR"],
  ] as const)("%s → %s", (reason, phase) => {
    t.toListening();
    t.playback.stop.mockClear();
    t.transport.end({ reason });
    expect(t.view()).toMatchObject({ phase, end: { reason } });
    expect(t.playback.stop).toHaveBeenCalled();
  });

  it("retry() reconnects after a takeover; never after completion", () => {
    t.toListening();
    t.transport.end({ reason: "superseded" });
    t.session.retry();
    expect(t.view()).toMatchObject({ phase: "CONNECTING", end: null });
    expect(t.transport.connect).toHaveBeenCalledTimes(2);
    t.transport.status("connected");
    t.transport.emit("session.sync", syncData({ resumed: true }));
    expect(t.view().phase).toBe("ASSET_LOADING");

    const done = setup();
    done.transport.end({ reason: "completed" });
    done.session.retry();
    expect(done.transport.connect).toHaveBeenCalledTimes(1);
    expect(done.view().phase).toBe("COMPLETED");
  });

  it("stop() disconnects, silences and stops listening", () => {
    t.toListening();
    t.session.stop();
    expect(t.transport.disconnect).toHaveBeenCalledTimes(1);
    expect(t.playback.stop).toHaveBeenCalled();
    expect(t.view().phase).toBe("IDLE");
    t.transport.emit("state", { state: "processing", position: 1 });
    expect(t.view().phase).toBe("IDLE");
  });
});
