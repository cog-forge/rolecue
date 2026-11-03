import { describe, expect, it } from "vitest";
import {
  createStreamOrder,
  encodeClientMessage,
  newCommandId,
  parseServerMessage,
} from "@/features/interview/transport/protocol";
import { serverMessage, syncData, textTurn } from "./fake-socket";

const parse = (value: unknown) => parseServerMessage(JSON.stringify(value));

describe("parseServerMessage", () => {
  it("accepts every v1 server message from the contract", () => {
    const messages = [
      serverMessage("session.sync", syncData(), 1),
      serverMessage(
        "session.sync",
        syncData({
          state: "completed",
          position: null,
          evaluation_status: "pending",
        }),
        2,
      ),
      serverMessage("state", { state: "listening", position: 4 }, 3),
      serverMessage("state", { state: "transitioning", position: null }, 4),
      serverMessage("interviewer.turn", textTurn(1), 5),
      serverMessage(
        "interviewer.turn",
        textTurn(2, {
          audio: { mime_type: "audio/mpeg", data: "SUQzBA==" },
          audio_status: "ready",
          visemes: [
            { offset_ms: 0, id: 0 },
            { offset_ms: 85, id: 12 },
          ],
          blend_shapes: [{ offset_ms: 0, values: [0, 0.5, 1] }],
          words: [{ offset_ms: 50, duration_ms: 300, text: "Walk" }],
          cues: [{ offset_ms: 0, kind: "emotion", name: "happy" }],
        }),
        6,
      ),
      serverMessage(
        "interviewer.turn",
        textTurn(3, { audio_status: "no_voice" }),
        7,
      ),
      serverMessage(
        "answer.accepted",
        { position: 4, answered_count: 4, duplicate: false, transcript: "Hi" },
        8,
        { replyTo: "c-17" },
      ),
      serverMessage(
        "answer.accepted",
        { position: 4, answered_count: 4, duplicate: true },
        9,
      ),
      serverMessage("session.completed", { evaluation_status: "pending" }, 10),
      serverMessage("ack", { duplicate: true }, 11, { replyTo: "c-1" }),
      serverMessage(
        "error",
        {
          code: "STT_NO_SPEECH",
          message: "no speech",
          retryable: true,
          fatal: false,
        },
        12,
      ),
      serverMessage("pong", {}, 13),
    ];
    for (const message of messages) {
      const result = parse(message);
      expect(result.success, JSON.stringify(message)).toBe(true);
    }
    const accepted = parse(messages[7]);
    expect(accepted.success && accepted.message.reply_to).toBe("c-17");
  });

  it("rejects frames that are not v1 server messages", () => {
    expect(parseServerMessage("{not json").success).toBe(false);
    expect(parse({ ...serverMessage("pong", {}, 1), v: 2 }).success).toBe(
      false,
    );
    expect(parse(serverMessage("session.resume", {}, 1)).success).toBe(false);
    expect(
      parse(serverMessage("state", { state: "listening" }, 1)).success,
    ).toBe(false);
    expect(
      parse(serverMessage("state", { state: "dancing", position: 1 }, 1))
        .success,
    ).toBe(false);
    expect(parse({ v: 1, type: "pong", data: {} }).success).toBe(false);
    expect(parse(serverMessage("pong", {}, 0)).success).toBe(false);
    expect(
      parse(
        serverMessage(
          "interviewer.turn",
          textTurn(1, {
            audio: { mime_type: "audio/mpeg", data: "not base64!!" },
            audio_status: "ready",
          }),
          1,
        ),
      ).success,
    ).toBe(false);
  });
});

describe("follow-ups (KAN-56)", () => {
  it("reads a follow-up turn with its core question number", () => {
    const result = parse(
      serverMessage(
        "interviewer.turn",
        textTurn(5, { kind: "follow_up", question_number: 3 }),
        1,
      ),
    );
    expect(result.success && result.message.data).toMatchObject({
      position: 5,
      question_number: 3,
      kind: "follow_up",
      question_count: 3,
    });
    const sync = parse(
      serverMessage(
        "session.sync",
        syncData({ position: 5, question_number: 3, answered_count: 4 }),
        2,
      ),
    );
    expect(sync.success && sync.message.data).toMatchObject({
      position: 5,
      question_number: 3,
    });
  });

  it("requires the question number", () => {
    const turn: Record<string, unknown> = { ...textTurn(1) };
    delete turn.question_number;
    expect(parse(serverMessage("interviewer.turn", turn, 1)).success).toBe(
      false,
    );
    const sync: Record<string, unknown> = { ...syncData() };
    delete sync.question_number;
    expect(parse(serverMessage("session.sync", sync, 1)).success).toBe(false);
    expect(
      parse(
        serverMessage(
          "interviewer.turn",
          textTurn(1, { kind: "probe" as "core" }),
          1,
        ),
      ).success,
    ).toBe(false);
  });
});

describe("createStreamOrder", () => {
  it("drops repeated or older messages on a stream", () => {
    const order = createStreamOrder();
    expect(order.accept({ stream: "a", seq: 1 })).toBe(true);
    expect(order.accept({ stream: "a", seq: 2 })).toBe(true);
    expect(order.accept({ stream: "a", seq: 2 })).toBe(false);
    expect(order.accept({ stream: "a", seq: 1 })).toBe(false);
    // A gap across a reconnect is normal.
    expect(order.accept({ stream: "a", seq: 9 })).toBe(true);
  });
  it("accepts a new stream that restarts at 1", () => {
    const order = createStreamOrder();
    order.accept({ stream: "a", seq: 40 });
    expect(order.accept({ stream: "b", seq: 1 })).toBe(true);
    expect(order.accept({ stream: "b", seq: 1 })).toBe(false);
    expect(order.accept({ stream: "b", seq: 2 })).toBe(true);
  });
});

describe("client messages", () => {
  it("adds the protocol version to every command", () => {
    expect(
      JSON.parse(
        encodeClientMessage({
          id: "c-1",
          type: "turn.played",
          data: { position: 2 },
        }),
      ),
    ).toEqual({ v: 1, id: "c-1", type: "turn.played", data: { position: 2 } });
    expect(JSON.parse(encodeClientMessage({ type: "ping", data: {} }))).toEqual(
      { v: 1, type: "ping", data: {} },
    );
  });
  it("creates ids the server accepts, fresh each time", () => {
    const first = newCommandId();
    expect(first).toMatch(/^[A-Za-z0-9_-]{1,64}$/);
    expect(newCommandId()).not.toBe(first);
  });
});
