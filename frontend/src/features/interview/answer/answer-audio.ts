import {
  MAX_AUDIO_CHUNK_BYTES,
  type ClientMessage,
} from "../transport/protocol";

/** §5 recommended recorder settings; the first supported type wins. */
export const ANSWER_MIME_TYPES = [
  "audio/webm;codecs=opus",
  "audio/ogg;codecs=opus",
  "audio/mp4",
] as const;
export const ANSWER_BITS_PER_SECOND = 32_000;

export interface RecordedAnswer {
  mimeType: string;
  /** Base64 chunks of at most 256 KiB decoded each, in order. */
  chunks: readonly string[];
  bytes: number;
}

function toBase64(bytes: Uint8Array) {
  let binary = "";
  for (let index = 0; index < bytes.length; index += 0x8000)
    binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000));
  return btoa(binary);
}

/** One whole recording split into chunks of at most `maxBytes` decoded bytes. */
export async function encodeAnswerAudio(
  blob: Blob,
  mimeType: string,
  maxBytes = MAX_AUDIO_CHUNK_BYTES,
): Promise<RecordedAnswer> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  const chunks: string[] = [];
  for (let start = 0; start < bytes.length; start += maxBytes)
    chunks.push(toBase64(bytes.subarray(start, start + maxBytes)));
  return { mimeType, chunks, bytes: bytes.length };
}

/** The chunk messages of one answer: the same id, mime_type on chunk 0 only, final on the last. */
export function answerAudioMessages(
  id: string,
  position: number,
  answer: RecordedAnswer,
): ClientMessage[] {
  return answer.chunks.map((audio, chunk) => ({
    id,
    type: "answer.audio",
    data: {
      position,
      chunk,
      final: chunk === answer.chunks.length - 1,
      ...(chunk === 0 ? { mime_type: answer.mimeType } : {}),
      audio,
    },
  }));
}
