import type { Transaction } from "../schemas/wallet-schema";

// Descriptions are user-controlled display text, never evidence of money direction or trade type.
export function classifyTransaction(tx: Transaction, me: string) {
  const outgoing = tx.from === me;
  const incoming = tx.to === me;
  if (tx.currency === "COIN" && tx.from && tx.to && (outgoing || incoming)) {
    return { kind: "trade" as const, positive: incoming && !outgoing };
  }
  if (tx.payos_order_code != null)
    return { kind: "topup" as const, positive: incoming };
  if (incoming && tx.from == null)
    return { kind: "refund" as const, positive: true };
  return { kind: "spend" as const, positive: incoming && !outgoing };
}
