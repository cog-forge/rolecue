import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { TransportEnd } from "@/features/interview/transport/interview-transport";
import type { ServerMessage } from "@/features/interview/transport/protocol";
import {
  createScriptedTransport,
  scriptedQuestions,
} from "@/features/interview/transport/scripted-transport";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

it("plays the protocol for a text-only practice interview without any network", async () => {
  const transport = createScriptedTransport({
    sessionId: "aaaaaaaa-aaaa-4aaa-8aaa-000000000249",
    delayMs: 10,
  });
  const messages: ServerMessage[] = [];
  const ends: TransportEnd[] = [];
  transport.subscribe((message) => messages.push(message));
  transport.subscribeEnd((end) => ends.push(end));
  void transport.connect();
  await vi.advanceTimersByTimeAsync(10);
  expect(messages.map((message) => message.type)).toEqual([
    "session.sync",
    "interviewer.turn",
  ]);
  for (let position = 1; position <= scriptedQuestions.length; position++) {
    transport.send({
      id: `p${position}`,
      type: "turn.played",
      data: { position },
    });
    transport.send({
      id: `a${position}`,
      type: "answer.text",
      data: { position, text: "Answer" },
    });
    await vi.advanceTimersByTimeAsync(20);
  }
  expect(
    messages.filter((message) => message.type === "answer.accepted"),
  ).toHaveLength(scriptedQuestions.length);
  expect(messages.at(-1)?.type).toBe("session.completed");
  expect(ends).toEqual([{ reason: "completed" }]);
  expect(messages.map((message) => message.seq)).toEqual(
    messages.map((_, index) => index + 1),
  );
});
