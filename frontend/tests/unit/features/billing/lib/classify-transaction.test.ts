import { describe, expect, it } from "vitest";
import { classifyTransaction } from "@/features/billing/lib/classify-transaction";
import type { Transaction } from "@/features/billing/schemas/wallet-schema";
const trade: Transaction = {
  id: "trade",
  from: "buyer",
  to: "seller",
  amount: 20,
  currency: "COIN",
  status: "success",
  description: "Refund Bot",
};
describe("structured wallet classification", () => {
  it.each([
    "Refund Bot",
    "Deposit: Pack (10000 VND)",
    "spend refund deposit",
    null,
  ])(
    "ignores description %s when classifying a seller's income",
    (description) => {
      expect(classifyTransaction({ ...trade, description }, "seller")).toEqual({
        kind: "trade",
        positive: true,
      });
      expect(classifyTransaction({ ...trade, description }, "buyer")).toEqual({
        kind: "trade",
        positive: false,
      });
    },
  );
  it("requires both parties, COIN and the current user's participation", () => {
    expect(
      classifyTransaction({ ...trade, currency: "VND" }, "seller").kind,
    ).not.toBe("trade");
    expect(classifyTransaction({ ...trade, from: null }, "seller").kind).toBe(
      "refund",
    );
    expect(classifyTransaction({ ...trade, to: null }, "buyer").kind).toBe(
      "spend",
    );
    expect(classifyTransaction(trade, "someone-else").kind).not.toBe("trade");
  });
  it("keeps PayOS top-ups and system refunds separate", () => {
    expect(
      classifyTransaction(
        { ...trade, from: null, payos_order_code: 123 },
        "seller",
      ),
    ).toEqual({ kind: "topup", positive: true });
    expect(
      classifyTransaction(
        { ...trade, from: null, payos_order_code: null },
        "seller",
      ),
    ).toEqual({ kind: "refund", positive: true });
  });
});
