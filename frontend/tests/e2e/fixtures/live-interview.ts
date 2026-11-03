import type { Page, WebSocketRoute } from "@playwright/test";

/** A scripted stand-in for the live interview server (docs/interview-ws-protocol.md). */
export const liveUrl = /\/interviews\/[^/]+\/live$/;

export type ClientFrame = {
  v: number;
  id?: string;
  type: string;
  data: Record<string, unknown>;
};
/** A core question; `followUp` adds one follow-up turn after it (KAN-56). */
type Question = { text: string; audio?: string; followUp?: string };

/** A short silent WAV, base64. Real enough for the browser to play and fire `ended`. */
export function silentWav(seconds = 0.4, rate = 8000) {
  const samples = Math.round(seconds * rate);
  const buffer = Buffer.alloc(44 + samples * 2);
  buffer.write("RIFF", 0);
  buffer.writeUInt32LE(36 + samples * 2, 4);
  buffer.write("WAVEfmt ", 8);
  buffer.writeUInt32LE(16, 16);
  buffer.writeUInt16LE(1, 20);
  buffer.writeUInt16LE(1, 22);
  buffer.writeUInt32LE(rate, 24);
  buffer.writeUInt32LE(rate * 2, 28);
  buffer.writeUInt16LE(2, 32);
  buffer.writeUInt16LE(16, 34);
  buffer.write("data", 36);
  buffer.writeUInt32LE(samples * 2, 40);
  return buffer.toString("base64");
}

export async function mockLiveServer(
  page: Page,
  {
    sessionId,
    questions,
    sessionType = "practice",
    dropWhenListeningAt,
  }: {
    sessionId: string;
    questions: readonly Question[];
    sessionType?: "practice" | "recruitment";
    /** Close the first socket with 1011 once the server listens at this position. */
    dropWhenListeningAt?: number;
  },
) {
  // The conversation: each core question, then its follow-up when it has one.
  const turns = questions.flatMap((question, index) => [
    { ...question, kind: "core", questionNumber: index + 1 },
    ...(question.followUp
      ? [
          {
            text: question.followUp,
            audio: question.audio,
            kind: "follow_up",
            questionNumber: index + 1,
          },
        ]
      : []),
  ]);
  const server = {
    connections: 0,
    frames: [] as ClientFrame[][],
    position: 1,
    // The turn being asked; it runs ahead of `position` when a follow-up was dropped on reconnect.
    index: 0,
    completed: false,
  };
  let seq = 0;
  let dropped = false;
  // Like the engine, accepted command ids are remembered for the whole stream, across reconnects.
  const accepted = new Set<string>();
  await page.routeWebSocket(liveUrl, (ws: WebSocketRoute) => {
    const frames: ClientFrame[] = [];
    server.frames.push(frames);
    server.connections++;
    let state = "interviewer_speaking";
    const send = (type: string, data: unknown, replyTo?: string) =>
      ws.send(
        JSON.stringify({
          v: 1,
          stream: "e2e-stream",
          seq: ++seq,
          type,
          ...(replyTo ? { reply_to: replyTo } : {}),
          data,
        }),
      );
    const setState = (next: string, replyTo?: string) => {
      state = next;
      send(
        "state",
        {
          state: next,
          position: ["transitioning", "completed"].includes(next)
            ? null
            : server.position,
        },
        replyTo,
      );
    };
    const turn = () => {
      const question = turns[server.index];
      send("interviewer.turn", {
        position: server.position,
        question_number: question.questionNumber,
        kind: question.kind,
        question_count: questions.length,
        text: question.text,
        audio: question.audio
          ? { mime_type: "audio/wav", data: question.audio }
          : null,
        audio_status: question.audio ? "ready" : "unavailable",
        visemes: [],
        blend_shapes: [],
        words: [],
        cues: [],
      });
    };
    const answered = (id: string, transcript: string) => {
      accepted.add(id);
      setState("processing", id);
      send(
        "answer.accepted",
        {
          position: server.position,
          answered_count: server.position,
          duplicate: false,
          ...(sessionType === "practice" ? { transcript } : {}),
        },
        id,
      );
      setState("transitioning");
      if (server.index === turns.length - 1) {
        server.completed = true;
        setState("completed");
        send("session.completed", { evaluation_status: "pending" });
        void ws.close({ code: 1000, reason: "interview completed" });
        return;
      }
      server.position++;
      server.index++;
      setState("interviewer_speaking");
      turn();
    };
    // Like the engine, a follow-up that was never answered is dropped on reconnect (§4.2): the next
    // core question takes its position.
    if (
      server.connections > 1 &&
      turns[server.index].kind === "follow_up" &&
      server.index < turns.length - 1
    )
      server.index++;
    send("session.sync", {
      session_id: sessionId,
      session_type: sessionType,
      state,
      question_count: questions.length,
      answered_count: server.position - 1,
      position: server.position,
      question_number: turns[server.index].questionNumber,
      resumed: server.connections > 1,
      evaluation_status: null,
      server_time: new Date().toISOString(),
    });
    turn();
    ws.onMessage((raw) => {
      const frame = JSON.parse(String(raw)) as ClientFrame;
      frames.push(frame);
      const position = frame.data.position;
      if (frame.type === "ping") return send("pong", {}, frame.id);
      if (frame.id && accepted.has(frame.id))
        return send("ack", { duplicate: true }, frame.id);
      if (frame.type === "turn.played") {
        if (state !== "interviewer_speaking" || position !== server.position)
          return send("ack", { duplicate: true }, frame.id);
        if (frame.id) accepted.add(frame.id);
        setState("listening", frame.id);
        if (dropWhenListeningAt === server.position && !dropped) {
          dropped = true;
          void ws.close({ code: 1011, reason: "e2e: engine restarted" });
        }
        return;
      }
      if (state !== "listening" || position !== server.position) return;
      if (frame.type === "answer.text")
        answered(String(frame.id), String(frame.data.text));
      if (frame.type === "answer.audio" && frame.data.final === true)
        answered(String(frame.id), "Spoken answer from the e2e server.");
    });
  });
  return server;
}
