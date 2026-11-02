import type {
  ConnectionStatus,
  InterviewTransport,
  TransportEnd,
} from "./interview-transport";
import {
  INTERVIEW_SUBPROTOCOL,
  createStreamOrder,
  encodeClientMessage,
  parseServerMessage,
  type ClientMessage,
  type ServerMessage,
} from "./protocol";
import {
  RECONNECT_MAX_ATTEMPTS,
  closeDecision,
  reconnectDelay,
  sessionCheckDecision,
  type CloseDecision,
  type SessionCheck,
} from "./reconnect-policy";

/** §6: the server pings every 25 s and drops a peer after 60 s of silence. */
export const CLIENT_PING_MS = 25_000;
/** Longer than the server's 60 s: while it transcribes and stores an answer (30 s STT plus 30 s
 * persistence by default) it does not answer pings, and that is not a dead connection.
 */
export const SILENCE_TIMEOUT_MS = 90_000;
/** A handshake that neither opens nor fails is treated as a lost connection. */
export const HANDSHAKE_TIMEOUT_MS = 15_000;
const CONNECTING = 0;
const OPEN = 1;

export function liveInterviewUrl(apiBaseUrl: string, interviewId: string) {
  const base = apiBaseUrl.replace(/\/+$/, "").replace(/^http/, "ws");
  return `${base}/interviews/${encodeURIComponent(interviewId)}/live`;
}

export interface WebSocketTransportOptions {
  url: string;
  /** GET /interviews/{id}, used only after a connection is lost or refused (close 1006). */
  checkSession: (signal: AbortSignal) => Promise<SessionCheck>;
  random?: () => number;
  maxAttempts?: number;
  createSocket?: (url: string, protocols: string[]) => WebSocket;
}

/** Native WebSocket runtime. Owns its socket, timers and listeners; nothing runs before connect(). */
export function createWebSocketTransport(
  options: WebSocketTransportOptions,
): InterviewTransport<ServerMessage, ClientMessage> {
  const random = options.random ?? Math.random;
  const maxAttempts = options.maxAttempts ?? RECONNECT_MAX_ATTEMPTS;
  const createSocket =
    options.createSocket ??
    ((url: string, protocols: string[]) => new WebSocket(url, protocols));
  const messageListeners = new Set<(message: ServerMessage) => void>();
  const statusListeners = new Set<(status: ConnectionStatus) => void>();
  const endListeners = new Set<(end: TransportEnd) => void>();
  // Ordering survives reconnects: seq continues on the same stream (§2).
  const order = createStreamOrder();
  let status: ConnectionStatus = "disconnected";
  let socket: WebSocket | null = null;
  let running = false;
  let connectedOnce = false;
  let attempt = 0;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let pingTimer: ReturnType<typeof setInterval> | null = null;
  let silenceTimer: ReturnType<typeof setTimeout> | null = null;
  let check: AbortController | null = null;
  let settleFirst: (() => void) | null = null;

  function setStatus(next: ConnectionStatus) {
    if (status === next) return;
    status = next;
    statusListeners.forEach((listener) => listener(next));
  }
  function clearTimers() {
    if (retryTimer) clearTimeout(retryTimer);
    if (pingTimer) clearInterval(pingTimer);
    if (silenceTimer) clearTimeout(silenceTimer);
    retryTimer = pingTimer = silenceTimer = null;
  }
  function settle() {
    settleFirst?.();
    settleFirst = null;
  }
  function releaseSocket() {
    const current = socket;
    socket = null;
    if (pingTimer) clearInterval(pingTimer);
    if (silenceTimer) clearTimeout(silenceTimer);
    pingTimer = silenceTimer = null;
    if (!current) return;
    current.onopen =
      current.onmessage =
      current.onclose =
      current.onerror =
        null;
    if (current.readyState === CONNECTING || current.readyState === OPEN)
      current.close(1000);
  }
  function end(value: TransportEnd) {
    running = false;
    clearTimers();
    releaseSocket();
    setStatus("disconnected");
    settle();
    endListeners.forEach((listener) => listener(value));
  }
  function armSilence() {
    if (silenceTimer) clearTimeout(silenceTimer);
    silenceTimer = setTimeout(() => {
      // A dead peer (sleep, network loss) may never fire close; drop it and recover like 1006.
      releaseSocket();
      lost(1006);
    }, SILENCE_TIMEOUT_MS);
  }
  function apply(decision: CloseDecision) {
    if (!running) return;
    if (decision.action === "end") end(decision.end);
    else if (decision.action === "reconnect") schedule();
    else void verify();
  }
  function lost(code: number) {
    if (!running) return;
    setStatus(connectedOnce ? "reconnecting" : "connecting");
    apply(closeDecision(code));
  }
  async function verify() {
    check?.abort();
    const controller = new AbortController();
    check = controller;
    let result: SessionCheck = "unreachable";
    try {
      result = await options.checkSession(controller.signal);
    } catch {
      result = "unreachable";
    }
    if (controller.signal.aborted || check !== controller) return;
    check = null;
    apply(sessionCheckDecision(result));
  }
  function schedule() {
    if (attempt >= maxAttempts) {
      end({ reason: "gave-up" });
      return;
    }
    settle();
    const delay = reconnectDelay(attempt, random);
    attempt++;
    retryTimer = setTimeout(() => {
      retryTimer = null;
      open();
    }, delay);
  }
  function open() {
    if (!running) return;
    setStatus(connectedOnce ? "reconnecting" : "connecting");
    let current: WebSocket;
    try {
      current = createSocket(options.url, [INTERVIEW_SUBPROTOCOL]);
    } catch {
      lost(1006);
      return;
    }
    socket = current;
    silenceTimer = setTimeout(() => {
      releaseSocket();
      lost(1006);
    }, HANDSHAKE_TIMEOUT_MS);
    current.onopen = () => {
      connectedOnce = true;
      setStatus("connected");
      settle();
      armSilence();
      pingTimer = setInterval(() => {
        if (current.readyState === OPEN)
          current.send(encodeClientMessage({ type: "ping", data: {} }));
      }, CLIENT_PING_MS);
    };
    current.onmessage = (event: MessageEvent) => {
      armSilence();
      // §2: the server sends JSON text frames only.
      if (typeof event.data !== "string") return;
      const parsed = parseServerMessage(event.data);
      if (!parsed.success) {
        console.warn("[interview] Ignored an invalid server message.");
        return;
      }
      // Backoff resets only once the server really serves the session, not on a bare open that
      // closes again (a crash loop would otherwise reconnect at the shortest delay forever).
      attempt = 0;
      if (!order.accept(parsed.message)) return;
      messageListeners.forEach((listener) => listener(parsed.message));
    };
    current.onclose = (event: CloseEvent) => {
      if (socket !== current) return;
      releaseSocket();
      lost(event.code);
    };
  }
  function disconnect() {
    running = false;
    check?.abort();
    check = null;
    clearTimers();
    releaseSocket();
    setStatus("disconnected");
    settle();
  }
  return {
    connect(signal) {
      if (signal?.aborted) return Promise.resolve();
      signal?.addEventListener("abort", disconnect, { once: true });
      if (running) return Promise.resolve();
      running = true;
      attempt = 0;
      const first = new Promise<void>((resolve) => {
        settleFirst = resolve;
      });
      open();
      return first;
    },
    disconnect,
    send(message) {
      if (socket?.readyState !== OPEN) return false;
      socket.send(encodeClientMessage(message));
      return true;
    },
    subscribe(listener) {
      messageListeners.add(listener);
      return () => messageListeners.delete(listener);
    },
    subscribeStatus(listener) {
      statusListeners.add(listener);
      return () => statusListeners.delete(listener);
    },
    subscribeEnd(listener) {
      endListeners.add(listener);
      return () => endListeners.delete(listener);
    },
  };
}
