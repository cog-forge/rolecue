import { z } from "zod";
import type { ParseResult } from "./interview-transport";

/** Live interview protocol v1. The contract is docs/interview-ws-protocol.md (KAN-52, follow-ups KAN-56); nothing here is invented. */
export const INTERVIEW_SUBPROTOCOL = "rolecue.interview.v1";
export const PROTOCOL_VERSION = 1;
/** §6: one decoded audio chunk. */
export const MAX_AUDIO_CHUNK_BYTES = 256 * 1024;
/** §6: one answer, decoded. */
export const MAX_ANSWER_AUDIO_BYTES = 4 * 1024 * 1024;
export const MAX_ANSWER_TEXT_LENGTH = 8000;
/** §6: product rule, 3 minutes per spoken answer. */
export const MAX_ANSWER_DURATION_MS = 3 * 60 * 1000;

const count = z.number().int().nonnegative();
const position = z.number().int().positive();
const offset = z.number().int().nonnegative();

export const serverStateSchema = z.enum([
  "interviewer_speaking",
  "listening",
  "processing",
  "transitioning",
  "completed",
]);
export const evaluationStatusSchema = z.enum([
  "pending",
  "completed",
  "failed",
]);

const sessionSyncSchema = z.object({
  session_id: z.uuid(),
  session_type: z.enum(["practice", "recruitment"]),
  state: serverStateSchema,
  question_count: count,
  answered_count: count,
  position: position.nullable(),
  question_number: position.nullable(),
  resumed: z.boolean(),
  evaluation_status: evaluationStatusSchema.nullable(),
  server_time: z.string(),
});
const stateChangeSchema = z.object({
  state: serverStateSchema,
  position: position.nullable(),
});
// §4.1: `position` is the turn's place in the conversation (answers name it); `question_number` is the
// core question it belongs to, which a follow-up shares. Progress counts core questions.
export const interviewerTurnSchema = z.object({
  position,
  question_number: position,
  kind: z.enum(["core", "follow_up"]),
  question_count: count,
  text: z.string(),
  audio: z
    .object({ mime_type: z.string().min(1), data: z.base64() })
    .nullable(),
  audio_status: z.enum(["ready", "unavailable", "no_voice"]),
  visemes: z.array(z.object({ offset_ms: offset, id: z.number().int() })),
  blend_shapes: z.array(
    z.object({ offset_ms: offset, values: z.array(z.number()) }),
  ),
  words: z.array(
    z.object({ offset_ms: offset, duration_ms: offset, text: z.string() }),
  ),
  // §4 cues (KAN-250): the client performs only the names it knows; others are ignored, never fatal.
  cues: z.array(
    z.object({
      offset_ms: offset,
      kind: z.enum(["emotion", "gesture"]),
      name: z.string().min(1),
    }),
  ),
});
const answerAcceptedSchema = z.object({
  position,
  answered_count: count,
  duplicate: z.boolean(),
  transcript: z.string().optional(),
});
const sessionCompletedSchema = z.object({
  evaluation_status: evaluationStatusSchema,
});
const ackSchema = z.object({ duplicate: z.boolean() });
const errorSchema = z.object({
  code: z.string().min(1),
  message: z.string(),
  retryable: z.boolean(),
  fatal: z.boolean(),
});

const envelope = {
  v: z.literal(PROTOCOL_VERSION),
  stream: z.string().min(1),
  seq: z.number().int().positive(),
  reply_to: z.string().optional(),
};
export const serverMessageSchema = z.discriminatedUnion("type", [
  z.object({
    ...envelope,
    type: z.literal("session.sync"),
    data: sessionSyncSchema,
  }),
  z.object({ ...envelope, type: z.literal("state"), data: stateChangeSchema }),
  z.object({
    ...envelope,
    type: z.literal("interviewer.turn"),
    data: interviewerTurnSchema,
  }),
  z.object({
    ...envelope,
    type: z.literal("answer.accepted"),
    data: answerAcceptedSchema,
  }),
  z.object({
    ...envelope,
    type: z.literal("session.completed"),
    data: sessionCompletedSchema,
  }),
  z.object({ ...envelope, type: z.literal("ack"), data: ackSchema }),
  z.object({ ...envelope, type: z.literal("error"), data: errorSchema }),
  z.object({ ...envelope, type: z.literal("pong"), data: z.object({}) }),
]);

export type ServerState = z.infer<typeof serverStateSchema>;
export type EvaluationStatus = z.infer<typeof evaluationStatusSchema>;
export type ServerMessage = z.infer<typeof serverMessageSchema>;
export type SessionSync = z.infer<typeof sessionSyncSchema>;
export type InterviewerTurn = z.infer<typeof interviewerTurnSchema>;
export type AnswerAccepted = z.infer<typeof answerAcceptedSchema>;
export type ServerError = z.infer<typeof errorSchema>;

/** Client commands (§5). `id` is required except on ping; repeat it to retry the same command. */
export type ClientMessage =
  | { id: string; type: "turn.played"; data: { position: number } }
  | {
      id: string;
      type: "answer.text";
      data: { position: number; text: string };
    }
  | {
      id: string;
      type: "answer.audio";
      data: {
        position: number;
        chunk: number;
        final: boolean;
        mime_type?: string;
        audio: string;
      };
    }
  | { id?: string; type: "ping"; data: Record<string, never> };

/** Decodes one text frame. Anything that is not JSON or not a v1 server message is rejected. */
export function parseServerMessage(raw: unknown): ParseResult<ServerMessage> {
  let value: unknown = raw;
  if (typeof raw === "string") {
    try {
      value = JSON.parse(raw);
    } catch {
      return { success: false, error: "Frame is not JSON." };
    }
  }
  const result = serverMessageSchema.safeParse(value);
  return result.success
    ? { success: true, message: result.data }
    : { success: false, error: z.prettifyError(result.error) };
}

export function encodeClientMessage(message: ClientMessage) {
  return JSON.stringify({ v: PROTOCOL_VERSION, ...message });
}

/** §2: 1 to 64 characters of A-Z a-z 0-9 _ -. A UUID fits. */
export function newCommandId() {
  return crypto.randomUUID();
}

/** `seq` grows by one per message on a stream; a new stream restarts it. Older or repeated messages are dropped. */
export function createStreamOrder() {
  let stream: string | null = null;
  let last = 0;
  return {
    accept(message: { stream: string; seq: number }) {
      if (message.stream !== stream) {
        stream = message.stream;
        last = message.seq;
        return true;
      }
      if (message.seq <= last) return false;
      last = message.seq;
      return true;
    },
  };
}
