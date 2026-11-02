import type { InterviewerTurn } from "@/features/interview/transport/protocol";

/** Test double for the browser WebSocket. Use `FakeSocket.create` as the transport's `createSocket`,
 * or `vi.stubGlobal("WebSocket", FakeSocket)` for components that construct a real transport.
 */
export class FakeSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: FakeSocket[] = [];
  static reset() {
    FakeSocket.instances = [];
  }
  static latest() {
    const socket = FakeSocket.instances.at(-1);
    if (!socket) throw new Error("No socket was created.");
    return socket;
  }
  /** The single cast to the DOM type lives here, so tests stay assertion-free. */
  static create(url: string, protocols: string[]) {
    return new FakeSocket(url, protocols) as unknown as WebSocket;
  }
  readyState = FakeSocket.CONNECTING;
  protocol = "";
  sent: string[] = [];
  closedWith: number | null = null;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  readonly protocols: string[];
  constructor(
    readonly url: string,
    protocols: string | string[] = [],
  ) {
    this.protocols = typeof protocols === "string" ? [protocols] : protocols;
    FakeSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close(code = 1000) {
    this.closedWith = code;
    this.readyState = FakeSocket.CLOSED;
  }
  open() {
    this.readyState = FakeSocket.OPEN;
    this.protocol = this.protocols[0] ?? "";
    this.onopen?.(new Event("open"));
  }
  receive(message: unknown) {
    const data =
      typeof message === "string" ? message : JSON.stringify(message);
    this.onmessage?.(new MessageEvent("message", { data }));
  }
  drop(code = 1006) {
    this.readyState = FakeSocket.CLOSED;
    this.onclose?.(new CloseEvent("close", { code }));
  }
  sentMessages() {
    return this.sent.map(
      (frame) =>
        JSON.parse(frame) as { v: number; type: string } & Record<
          string,
          unknown
        >,
    );
  }
}

export function serverMessage(
  type: string,
  data: unknown,
  seq: number,
  { stream = "stream-1", replyTo }: { stream?: string; replyTo?: string } = {},
) {
  return {
    v: 1,
    stream,
    seq,
    type,
    ...(replyTo ? { reply_to: replyTo } : {}),
    data,
  };
}

export function textTurn(
  position: number,
  overrides: Partial<InterviewerTurn> = {},
): InterviewerTurn {
  return {
    position,
    kind: "core",
    question_count: 3,
    text: `Question ${position}?`,
    audio: null,
    audio_status: "unavailable",
    visemes: [],
    blend_shapes: [],
    words: [],
    cues: [],
    ...overrides,
  };
}

export function syncData(
  overrides: Partial<{
    session_type: "practice" | "recruitment";
    state: string;
    question_count: number;
    answered_count: number;
    position: number | null;
    resumed: boolean;
    evaluation_status: string | null;
  }> = {},
) {
  return {
    session_id: "aaaaaaaa-aaaa-4aaa-8aaa-000000000249",
    session_type: "practice",
    state: "interviewer_speaking",
    question_count: 3,
    answered_count: 0,
    position: 1,
    resumed: false,
    evaluation_status: null,
    server_time: "2026-11-02T10:00:00Z",
    ...overrides,
  };
}
