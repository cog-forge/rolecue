// @vitest-environment node
import { AxiosError, type AxiosResponse } from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  RecordStatus,
  RecordingUploadGrant,
} from "@/features/interview/recording/recording-api";
import { createMemoryRecordingStore } from "@/features/interview/recording/recording-buffer";
import {
  createRecordingUploader,
  type UploadApi,
  type UploadSnapshot,
} from "@/features/interview/recording/recording-upload";
import { deferred } from "../audio/media-fakes";
import { readWebm, webmFile } from "./webm-builder";

const interview = "interview-a";
const grant: RecordingUploadGrant = {
  upload_url: "https://storage.example.test/put",
  method: "PUT",
  headers: { "Content-Type": "video/webm" },
  expires_at: "2026-11-03T08:15:00Z",
};
function apiError(status: number, code?: string) {
  return new AxiosError("request failed", "ERR", undefined, undefined, {
    status,
    data: code ? { success: false, error: { code } } : undefined,
  } as AxiosResponse);
}
const networkError = () => new AxiosError("Network Error", "ERR_NETWORK");

/** The recording API as the server behaves: complete verifies the object the last PUT stored. */
function fakeServer({ failPuts = 0 }: { failPuts?: number } = {}) {
  const server = {
    status: "none" as RecordStatus,
    stored: false,
    failPuts,
    bodies: [] as Blob[],
    ids: [] as string[],
  };
  const api: UploadApi = {
    status: vi.fn(async (id) => {
      server.ids.push(id);
      return server.status;
    }),
    grant: vi.fn(async (id) => {
      server.ids.push(id);
      if (server.status === "ready")
        throw apiError(409, "RECORDING_NOT_ALLOWED");
      server.status = "uploading";
      server.stored = false;
      return grant;
    }),
    put: vi.fn(async (_grant, blob, onProgress) => {
      onProgress(0.5);
      if (server.failPuts > 0) {
        server.failPuts--;
        throw networkError();
      }
      server.bodies.push(blob);
      server.stored = true;
      onProgress(1);
    }),
    complete: vi.fn(async (id: string): Promise<RecordStatus> => {
      server.ids.push(id);
      if (server.status === "ready") return "ready";
      if (server.status !== "uploading")
        throw apiError(409, "RECORDING_NOT_ALLOWED");
      if (server.stored) {
        server.status = "ready";
        return "ready";
      }
      server.status = "failed";
      throw apiError(404, "RECORDING_NOT_FOUND");
    }),
  };
  return { server, api };
}

const delays: number[] = [];
async function setup(
  api: UploadApi,
  slices: Uint8Array<ArrayBuffer>[][] = [
    [
      webmFile({
        clusters: [{ time: 0, blocks: [{ track: 1, time: 0, tag: 1 }] }],
      }),
    ],
  ],
) {
  const store = createMemoryRecordingStore();
  const buffer = store.open("candidate-1", interview);
  for (const [segment, chunks] of slices.entries())
    for (const [seq, chunk] of chunks.entries())
      await buffer.append(
        segment + 1,
        seq,
        "video/webm;codecs=vp8,opus",
        new Blob([chunk]),
      );
  const uploader = createRecordingUploader({
    interviewId: interview,
    buffer,
    api,
    delay: async (ms) => {
      delays.push(ms);
    },
  });
  const seen: UploadSnapshot[] = [];
  uploader.subscribe(() => seen.push(uploader.getSnapshot()));
  return { uploader, buffer, store, seen };
}
beforeEach(() => {
  delays.length = 0;
});

describe("upload", () => {
  it("shows progress, retries an interrupted upload, then is ready and clears the buffer", async () => {
    const { server, api } = fakeServer({ failPuts: 1 });
    const { uploader, buffer, seen } = await setup(api);
    await uploader.run();
    const statuses = seen.map((snapshot) => snapshot.status);
    expect(statuses).toEqual(
      expect.arrayContaining([
        "checking",
        "preparing",
        "uploading",
        "retrying",
        "ready",
      ]),
    );
    expect(statuses.indexOf("retrying")).toBeLessThan(
      statuses.lastIndexOf("uploading"),
    );
    expect(seen.some((snapshot) => snapshot.progress === 0.5)).toBe(true);
    expect(uploader.getSnapshot()).toMatchObject({
      status: "ready",
      attempt: 2,
    });
    expect(delays).toEqual([2_000]);
    expect(server.status).toBe("ready");
    expect(api.grant).toHaveBeenCalledWith(interview, {
      contentType: "video/webm",
      size: server.bodies[0].size,
    });
    expect(await buffer.load()).toEqual([]);
    expect(uploader.unsettled()).toBe(false);
  });

  it("ends failed after the last attempt, with the server failed and the copy kept for a retry", async () => {
    const { server, api } = fakeServer({ failPuts: 3 });
    const { uploader, buffer } = await setup(api);
    await uploader.run();
    expect(uploader.getSnapshot()).toMatchObject({
      status: "failed",
      reason: "network",
      attempt: 3,
      abandoned: false,
    });
    expect(api.grant).toHaveBeenCalledTimes(3);
    expect(api.complete).toHaveBeenCalledTimes(3);
    expect(delays).toEqual([2_000, 5_000]);
    expect(server.status).toBe("failed");
    expect(await buffer.load()).toHaveLength(1);

    // Later, from the room or the completion screen.
    await uploader.run();
    expect(uploader.getSnapshot().status).toBe("ready");
    expect(server.status).toBe("ready");
  });

  it("asks completion even when the PUT reply is lost, so an object that arrived is ready", async () => {
    const { server, api } = fakeServer();
    vi.mocked(api.put).mockImplementationOnce(async (_grant, blob) => {
      server.bodies.push(blob);
      server.stored = true;
      throw networkError();
    });
    const { uploader } = await setup(api);
    await uploader.run();
    expect(uploader.getSnapshot()).toMatchObject({
      status: "ready",
      attempt: 1,
    });
    expect(api.grant).toHaveBeenCalledTimes(1);
  });

  it("is settled only once completion answers", async () => {
    const { api } = fakeServer();
    const put = deferred<void>();
    vi.mocked(api.put).mockImplementationOnce(() => put.promise);
    const { uploader } = await setup(api);
    const running = uploader.run();
    await vi.waitFor(() => expect(api.put).toHaveBeenCalled());
    expect(uploader.unsettled()).toBe(true);
    put.resolve();
    await running;
    expect(uploader.unsettled()).toBe(false);
  });

  it("does not upload again a recording the server already has", async () => {
    const { server, api } = fakeServer();
    server.status = "ready";
    const { uploader, buffer } = await setup(api);
    await uploader.run();
    expect(uploader.getSnapshot().status).toBe("ready");
    expect(api.grant).not.toHaveBeenCalled();
    expect(await buffer.load()).toEqual([]);
  });

  it("says when this browser has no recording instead of reporting none or ready", async () => {
    const { api } = fakeServer();
    const { uploader } = await setup(api, []);
    await uploader.run();
    expect(uploader.getSnapshot().status).toBe("missing");
    expect(api.grant).not.toHaveBeenCalled();
  });

  it("does not retry an upload the server refuses", async () => {
    const { api } = fakeServer();
    vi.mocked(api.grant).mockRejectedValue(apiError(400, "VALIDATION_ERROR"));
    const { uploader } = await setup(api);
    await uploader.run();
    expect(uploader.getSnapshot()).toMatchObject({
      status: "failed",
      reason: "rejected",
    });
    expect(api.grant).toHaveBeenCalledTimes(1);
  });
});

describe("reload", () => {
  it("uploads one file holding the segments recorded before and after the reload", async () => {
    const { server, api } = fakeServer();
    const before = webmFile({
      clusters: [
        {
          time: 0,
          blocks: [
            { track: 1, time: 0, tag: 11 },
            { track: 1, time: 60, tag: 12 },
          ],
        },
      ],
    });
    const after = webmFile({
      clusters: [{ time: 0, blocks: [{ track: 1, time: 0, tag: 21 }] }],
    });
    // The first segment arrived as two slices, the way MediaRecorder hands them over.
    const { uploader } = await setup(api, [
      [before.subarray(0, 40), before.subarray(40)],
      [after],
    ]);
    await uploader.run();
    expect(uploader.getSnapshot()).toMatchObject({
      status: "ready",
      partial: false,
    });
    const uploaded = readWebm(
      new Uint8Array(await server.bodies[0].arrayBuffer()),
    );
    expect(uploaded.segments).toBe(1);
    expect(uploaded.blocks.map((block) => [block.tag, block.time])).toEqual([
      [11, 0],
      [12, 60],
      [21, 120],
    ]);
  });
});

describe("giving up", () => {
  it("leaves the recording failed on the server and deletes the copy in this browser", async () => {
    const { server, api } = fakeServer({ failPuts: 3 });
    const { uploader, buffer } = await setup(api);
    await uploader.run();
    await uploader.giveUp();
    expect(uploader.getSnapshot()).toMatchObject({
      status: "failed",
      abandoned: true,
    });
    expect(server.status).toBe("failed");
    expect(await buffer.load()).toEqual([]);
  });

  it("completes an upload the server still has as uploading, so it ends failed", async () => {
    const { server, api } = fakeServer({ failPuts: 3 });
    vi.mocked(api.complete).mockRejectedValue(networkError());
    const { uploader } = await setup(api);
    await uploader.run();
    expect(server.status).toBe("uploading");
    vi.mocked(api.complete).mockImplementation(async () => {
      server.status = "failed";
      throw apiError(404, "RECORDING_NOT_FOUND");
    });
    await uploader.giveUp();
    expect(server.status).toBe("failed");
    expect(uploader.getSnapshot()).toMatchObject({
      status: "failed",
      abandoned: true,
    });
  });

  it("keeps the copy when the server cannot be reached", async () => {
    const { api } = fakeServer({ failPuts: 3 });
    const { uploader, buffer } = await setup(api);
    await uploader.run();
    vi.mocked(api.status).mockRejectedValue(networkError());
    await uploader.giveUp();
    expect(uploader.getSnapshot()).toMatchObject({
      status: "failed",
      abandoned: false,
    });
    expect(await buffer.load()).toHaveLength(1);
  });
});

it("calls the API only for its own interview and reads only its own recording", async () => {
  const { server, api } = fakeServer();
  const { uploader, store } = await setup(api, []);
  await store.open("candidate-1", "interview-b").append(
    1,
    0,
    "video/webm",
    new Blob([
      webmFile({
        clusters: [{ time: 0, blocks: [{ track: 1, time: 0, tag: 1 }] }],
      }),
    ]),
  );
  await store.open("candidate-2", interview).append(
    1,
    0,
    "video/webm",
    new Blob([
      webmFile({
        clusters: [{ time: 0, blocks: [{ track: 1, time: 0, tag: 2 }] }],
      }),
    ]),
  );
  await uploader.run();
  expect(uploader.getSnapshot().status).toBe("missing");
  expect(new Set(server.ids)).toEqual(new Set([interview]));
  expect(api.put).not.toHaveBeenCalled();
  expect(await store.open("candidate-1", "interview-b").load()).toHaveLength(1);
});
