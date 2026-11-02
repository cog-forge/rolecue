import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ConnectionStatus,
  TransportEnd,
} from "@/features/interview/transport/interview-transport";
import type { ServerMessage } from "@/features/interview/transport/protocol";
import type { SessionCheck } from "@/features/interview/transport/reconnect-policy";
import {
  CLIENT_PING_MS,
  HANDSHAKE_TIMEOUT_MS,
  SILENCE_TIMEOUT_MS,
  createWebSocketTransport,
  liveInterviewUrl,
} from "@/features/interview/transport/websocket-transport";
import { FakeSocket, serverMessage, syncData } from "./fake-socket";

const url = "ws://api.test/interviews/abc/live";
function setup(check: SessionCheck = "active", maxAttempts?: number) {
  const checkSession = vi.fn(async (signal: AbortSignal) => {
    void signal;
    return check;
  });
  const transport = createWebSocketTransport({
    url,
    checkSession,
    random: () => 0,
    maxAttempts,
    createSocket: FakeSocket.create,
  });
  const messages: ServerMessage[] = [];
  const statuses: ConnectionStatus[] = [];
  const ends: TransportEnd[] = [];
  transport.subscribe((message) => messages.push(message));
  transport.subscribeStatus((status) => statuses.push(status));
  transport.subscribeEnd((end) => ends.push(end));
  return { transport, checkSession, messages, statuses, ends };
}
const pong = (seq: number, stream?: string) =>
  serverMessage("pong", {}, seq, { stream });

beforeEach(() => {
  vi.useFakeTimers();
  FakeSocket.reset();
});
afterEach(() => {
  vi.useRealTimers();
});

it("builds the live URL from the API base", () => {
  expect(liveInterviewUrl("http://localhost:8140/", "a b")).toBe(
    "ws://localhost:8140/interviews/a%20b/live",
  );
  expect(liveInterviewUrl("https://api.rolecue.test", "id")).toBe(
    "wss://api.rolecue.test/interviews/id/live",
  );
});

it("opens nothing before connect, then offers the subprotocol and reports status", async () => {
  const { transport, statuses } = setup();
  expect(FakeSocket.instances).toHaveLength(0);
  const connected = transport.connect();
  const socket = FakeSocket.latest();
  expect(socket.url).toBe(url);
  expect(socket.protocols).toEqual(["rolecue.interview.v1"]);
  expect(statuses).toEqual(["connecting"]);
  socket.open();
  await connected;
  expect(statuses).toEqual(["connecting", "connected"]);
});

it("delivers valid messages in order and drops invalid or stale ones", () => {
  const { transport, messages } = setup();
  void transport.connect();
  const socket = FakeSocket.latest();
  socket.open();
  vi.spyOn(console, "warn").mockImplementation(() => {});
  socket.receive(serverMessage("session.sync", syncData(), 1));
  socket.receive("not json");
  socket.receive(serverMessage("mystery", {}, 2));
  socket.receive(pong(2));
  socket.receive(pong(2));
  socket.receive(pong(1));
  socket.receive(pong(3));
  expect(messages.map((message) => message.seq)).toEqual([1, 2, 3]);
});

it("keeps seq order across a reconnect and accepts a new stream", async () => {
  const { transport, messages, checkSession } = setup("active");
  void transport.connect();
  FakeSocket.latest().open();
  FakeSocket.latest().receive(pong(5));
  FakeSocket.latest().drop(1006);
  await vi.advanceTimersByTimeAsync(0);
  expect(checkSession).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(1000);
  expect(FakeSocket.instances).toHaveLength(2);
  const second = FakeSocket.latest();
  second.open();
  second.receive(pong(4));
  second.receive(pong(6));
  second.receive(pong(1, "stream-2"));
  expect(messages.map((message) => [message.stream, message.seq])).toEqual([
    ["stream-1", 5],
    ["stream-1", 6],
    ["stream-2", 1],
  ]);
});

describe("close codes", () => {
  it("1006 checks the session, then reconnects with backoff when it is still open", async () => {
    const { transport, checkSession, statuses } = setup("active");
    void transport.connect();
    FakeSocket.latest().open();
    FakeSocket.latest().drop(1006);
    expect(statuses.at(-1)).toBe("reconnecting");
    await vi.advanceTimersByTimeAsync(0);
    expect(checkSession).toHaveBeenCalledTimes(1);
    expect(FakeSocket.instances).toHaveLength(1);
    // random() = 0 → the first delay is half of 500 ms.
    await vi.advanceTimersByTimeAsync(249);
    expect(FakeSocket.instances).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(FakeSocket.instances).toHaveLength(2);
    FakeSocket.latest().open();
    expect(statuses.at(-1)).toBe("connected");
  });

  it.each([
    ["denied", "denied"],
    ["not-found", "not-found"],
    ["completed", "completed"],
  ] as const)(
    "1006 with a %s session ends without reconnecting",
    async (check, reason) => {
      const { transport, ends, statuses } = setup(check);
      void transport.connect();
      FakeSocket.latest().drop(1006);
      await vi.advanceTimersByTimeAsync(60_000);
      expect(ends).toEqual([{ reason }]);
      expect(statuses.at(-1)).toBe("disconnected");
      expect(FakeSocket.instances).toHaveLength(1);
    },
  );

  it.each([
    [4000, "protocol"],
    [4001, "superseded"],
    [4003, "not-eligible"],
    [1009, "protocol"],
    [1000, "completed"],
  ] as const)("%i ends without a check or reconnect", async (code, reason) => {
    const { transport, ends, checkSession } = setup();
    void transport.connect();
    FakeSocket.latest().open();
    FakeSocket.latest().drop(code);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(ends).toEqual([{ reason }]);
    expect(checkSession).not.toHaveBeenCalled();
    expect(FakeSocket.instances).toHaveLength(1);
  });

  it.each([1001, 1011])(
    "%i reconnects without a session check",
    async (code) => {
      const { transport, checkSession } = setup();
      void transport.connect();
      FakeSocket.latest().open();
      FakeSocket.latest().drop(code);
      await vi.advanceTimersByTimeAsync(250);
      expect(checkSession).not.toHaveBeenCalled();
      expect(FakeSocket.instances).toHaveLength(2);
    },
  );

  it("gives up after the maximum number of attempts", async () => {
    const { transport, ends } = setup("unreachable", 3);
    void transport.connect();
    for (let index = 0; index < 3; index++) {
      FakeSocket.latest().drop(1006);
      await vi.advanceTimersByTimeAsync(10_000);
    }
    expect(FakeSocket.instances).toHaveLength(4);
    expect(ends).toEqual([]);
    FakeSocket.latest().drop(1006);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(ends).toEqual([{ reason: "gave-up" }]);
    expect(FakeSocket.instances).toHaveLength(4);
  });

  it("resets the attempt count once a reconnect serves the session", async () => {
    const { transport, ends } = setup("active", 2);
    void transport.connect();
    for (let index = 0; index < 5; index++) {
      const socket = FakeSocket.latest();
      socket.open();
      socket.receive(serverMessage("session.sync", syncData(), index + 1));
      socket.drop(1011);
      await vi.advanceTimersByTimeAsync(10_000);
    }
    expect(ends).toEqual([]);
    expect(FakeSocket.instances).toHaveLength(6);
  });

  it("keeps backing off when the server opens and closes again without serving", async () => {
    const { transport, ends } = setup("active", 2);
    void transport.connect();
    for (let index = 0; index < 3; index++) {
      FakeSocket.latest().open();
      FakeSocket.latest().drop(1011);
      await vi.advanceTimersByTimeAsync(20_000);
    }
    expect(ends).toEqual([{ reason: "gave-up" }]);
    expect(FakeSocket.instances).toHaveLength(3);
  });
});

it("sends only while open", () => {
  const { transport } = setup();
  const message = {
    id: "c-1",
    type: "turn.played" as const,
    data: { position: 1 },
  };
  expect(transport.send(message)).toBe(false);
  void transport.connect();
  const socket = FakeSocket.latest();
  expect(transport.send(message)).toBe(false);
  socket.open();
  expect(transport.send(message)).toBe(true);
  expect(socket.sentMessages()).toEqual([{ v: 1, ...message }]);
});

it("pings every 25 s and treats a long silence as a lost connection", async () => {
  const { transport, checkSession, statuses } = setup("active");
  void transport.connect();
  const socket = FakeSocket.latest();
  socket.open();
  await vi.advanceTimersByTimeAsync(CLIENT_PING_MS);
  expect(socket.sentMessages()).toEqual([{ v: 1, type: "ping", data: {} }]);
  socket.receive(pong(1));
  await vi.advanceTimersByTimeAsync(SILENCE_TIMEOUT_MS - 1);
  expect(checkSession).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(1);
  expect(socket.closedWith).toBe(1000);
  expect(statuses.at(-1)).toBe("reconnecting");
  expect(checkSession).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(250);
  expect(FakeSocket.instances).toHaveLength(2);
});

it("treats a handshake that never opens as a lost connection", async () => {
  const { transport, checkSession } = setup("active");
  void transport.connect();
  const socket = FakeSocket.latest();
  await vi.advanceTimersByTimeAsync(HANDSHAKE_TIMEOUT_MS - 1);
  expect(checkSession).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(1);
  expect(socket.closedWith).toBe(1000);
  expect(checkSession).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(250);
  expect(FakeSocket.instances).toHaveLength(2);
});

it("disconnect() closes, stops timers and never reconnects or ends", async () => {
  const { transport, ends, statuses } = setup();
  void transport.connect();
  const socket = FakeSocket.latest();
  socket.open();
  transport.disconnect();
  expect(socket.closedWith).toBe(1000);
  expect(socket.onclose).toBeNull();
  expect(statuses.at(-1)).toBe("disconnected");
  await vi.advanceTimersByTimeAsync(120_000);
  expect(socket.sent).toEqual([]);
  expect(FakeSocket.instances).toHaveLength(1);
  expect(ends).toEqual([]);
});

it("disconnect() during a session check cancels the reconnect", async () => {
  let resolve: (check: SessionCheck) => void = () => {};
  const checkSession = vi.fn(
    (signal: AbortSignal) =>
      new Promise<SessionCheck>((done) => {
        void signal;
        resolve = done;
      }),
  );
  const transport = createWebSocketTransport({
    url,
    checkSession,
    createSocket: FakeSocket.create,
  });
  void transport.connect();
  FakeSocket.latest().drop(1006);
  expect(checkSession.mock.calls[0][0].aborted).toBe(false);
  transport.disconnect();
  expect(checkSession.mock.calls[0][0].aborted).toBe(true);
  resolve("active");
  await vi.advanceTimersByTimeAsync(60_000);
  expect(FakeSocket.instances).toHaveLength(1);
});

it("can connect again after an end", async () => {
  const { transport, ends, statuses } = setup();
  void transport.connect();
  FakeSocket.latest().open();
  FakeSocket.latest().drop(4001);
  expect(ends).toEqual([{ reason: "superseded" }]);
  void transport.connect();
  expect(FakeSocket.instances).toHaveLength(2);
  expect(statuses.at(-1)).toBe("reconnecting");
  FakeSocket.latest().open();
  expect(statuses.at(-1)).toBe("connected");
});

it("an aborted signal disconnects", () => {
  const { transport, statuses } = setup();
  const controller = new AbortController();
  void transport.connect(controller.signal);
  FakeSocket.latest().open();
  controller.abort();
  expect(statuses.at(-1)).toBe("disconnected");
  expect(FakeSocket.latest().closedWith).toBe(1000);
});
