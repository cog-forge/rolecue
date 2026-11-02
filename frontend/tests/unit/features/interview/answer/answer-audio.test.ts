import { describe, expect, it } from "vitest";
import {
  answerAudioMessages,
  encodeAnswerAudio,
} from "@/features/interview/answer/answer-audio";

const KiB = 1024;
const decode = (base64: string) =>
  Uint8Array.from(atob(base64), (char) => char.charCodeAt(0));

describe("encodeAnswerAudio", () => {
  it("splits one recording into chunks of at most 256 KiB that round-trip", async () => {
    const bytes = new Uint8Array(600 * KiB).map((_, index) => index % 251);
    const answer = await encodeAnswerAudio(
      new Blob([bytes], { type: "audio/webm" }),
      "audio/webm;codecs=opus",
    );
    expect(answer.mimeType).toBe("audio/webm;codecs=opus");
    expect(answer.bytes).toBe(600 * KiB);
    const decoded = answer.chunks.map(decode);
    expect(decoded.map((chunk) => chunk.length)).toEqual([
      256 * KiB,
      256 * KiB,
      88 * KiB,
    ]);
    const joined = new Uint8Array(answer.bytes);
    let offset = 0;
    for (const chunk of decoded) {
      joined.set(chunk, offset);
      offset += chunk.length;
    }
    // Element-wise toEqual on 600 KiB is slow; compare the bytes directly.
    expect(joined.every((value, index) => value === bytes[index])).toBe(true);
  });

  it("keeps a small recording in one chunk", async () => {
    const answer = await encodeAnswerAudio(
      new Blob([new Uint8Array([1, 2, 3])]),
      "audio/ogg",
    );
    expect(answer.chunks).toEqual(["AQID"]);
  });
});

describe("answerAudioMessages", () => {
  it("shares one id, numbers chunks, sends mime_type first and final last", () => {
    const messages = answerAudioMessages("c-9", 4, {
      mimeType: "audio/webm;codecs=opus",
      chunks: ["AA==", "AQ==", "Ag=="],
      bytes: 3,
    });
    expect(messages).toEqual([
      {
        id: "c-9",
        type: "answer.audio",
        data: {
          position: 4,
          chunk: 0,
          final: false,
          mime_type: "audio/webm;codecs=opus",
          audio: "AA==",
        },
      },
      {
        id: "c-9",
        type: "answer.audio",
        data: { position: 4, chunk: 1, final: false, audio: "AQ==" },
      },
      {
        id: "c-9",
        type: "answer.audio",
        data: { position: 4, chunk: 2, final: true, audio: "Ag==" },
      },
    ]);
  });
  it("marks a single chunk as final", () => {
    const [only] = answerAudioMessages("c-1", 1, {
      mimeType: "audio/mp4",
      chunks: ["AA=="],
      bytes: 1,
    });
    expect(only.type === "answer.audio" && only.data).toMatchObject({
      chunk: 0,
      final: true,
      mime_type: "audio/mp4",
    });
  });
});
