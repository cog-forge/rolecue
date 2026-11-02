export type ConnectionStatus =
  "disconnected" | "connecting" | "connected" | "reconnecting";
export type ParseResult<T> =
  { success: true; message: T } | { success: false; error: string };
export type ProtocolParser<T> = (raw: unknown) => ParseResult<T>;
/** Why a transport stopped for good. It never reconnects by itself after one of these. */
export type TransportEnd =
  | { reason: "completed" }
  | { reason: "superseded" }
  | { reason: "not-eligible" }
  | { reason: "protocol" }
  | { reason: "denied" }
  | { reason: "not-found" }
  | { reason: "gave-up" };
export interface InterviewTransport<Incoming, Outgoing> {
  /** Opens the connection, or reopens it after an end. Resolves once the first attempt settles. */
  connect(signal?: AbortSignal): Promise<void>;
  disconnect(): void;
  /** False when the socket is not open; the caller decides whether to resend after the next sync. */
  send(message: Outgoing): boolean;
  subscribe(listener: (message: Incoming) => void): () => void;
  subscribeStatus(listener: (status: ConnectionStatus) => void): () => void;
  subscribeEnd(listener: (end: TransportEnd) => void): () => void;
}
