import { expect, it } from "vitest";
import {
  createIndexedDbRecordingStore,
  createMemoryRecordingStore,
} from "@/features/interview/recording/recording-buffer";

const slice = (text: string) => new Blob([text], { type: "video/webm" });
async function texts(chunks: Blob[]) {
  return Promise.all(chunks.map((chunk) => chunk.text()));
}

it("keeps slices per account and interview, in segment and slice order", async () => {
  const store = createMemoryRecordingStore();
  const mine = store.open("candidate-1", "interview-a");
  await mine.append(200, 1, "video/webm", slice("b2"));
  await mine.append(100, 0, "video/webm", slice("a1"));
  await mine.append(200, 0, "video/webm", slice("b1"));
  await store
    .open("candidate-1", "interview-b")
    .append(1, 0, "video/webm", slice("x"));
  await store
    .open("candidate-2", "interview-a")
    .append(1, 0, "video/webm", slice("y"));

  const segments = await mine.load();
  expect(segments.map((segment) => segment.segment)).toEqual([100, 200]);
  expect(await texts(segments[0].chunks)).toEqual(["a1"]);
  expect(await texts(segments[1].chunks)).toEqual(["b1", "b2"]);
  expect(segments[1].bytes).toBe(4);

  await mine.clear();
  expect(await mine.load()).toEqual([]);
  expect(await store.open("candidate-1", "interview-b").load()).toHaveLength(1);
  expect(await store.open("candidate-2", "interview-a").load()).toHaveLength(1);
});

it("falls back to memory for this page when IndexedDB is unavailable", async () => {
  const store = createIndexedDbRecordingStore({ factory: undefined });
  const buffer = store.open("candidate-1", "interview-a");
  await buffer.append(1, 0, "video/webm", slice("kept"));
  const [segment] = await buffer.load();
  expect(await texts(segment.chunks)).toEqual(["kept"]);
});
