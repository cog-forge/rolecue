import { describe, expect, it } from "vitest";
import {
  coinPackageListEnvelopeSchema,
  coinPackageSchema,
  depositCheckoutEnvelopeSchema,
  depositCheckoutRequestSchema,
  transactionPageEnvelopeSchema,
  transactionSchema,
  walletEnvelopeSchema,
  walletSchema,
} from "@/features/billing/schemas/wallet-schema";

describe("wallet-schema", () => {
  it("validates wallet domain models and envelopes", () => {
    const rawWallet = {
      id: "11111111-1111-4111-8111-111111111111",
      user_id: "22222222-2222-4222-8222-222222222222",
      balance: 150,
    };
    const parsedWallet = walletSchema.parse(rawWallet);
    expect(parsedWallet.balance).toBe(150);

    const envelope = walletEnvelopeSchema.parse({
      success: true,
      data: rawWallet,
    });
    expect(envelope.success).toBe(true);
    expect(envelope.data.id).toBe(rawWallet.id);
  });

  it("validates coin packages and package lists", () => {
    const rawPackage = {
      id: "33333333-3333-4333-8333-333333333333",
      name: "Starter Package",
      price_vnd: 50000,
      description: "Includes 100 coins for practice",
      coin_amount: 100,
      currency: "VND",
      is_active: true,
      created_at: "2026-10-10T00:00:00Z",
    };
    const parsed = coinPackageSchema.parse(rawPackage);
    expect(parsed.price_vnd).toBe(50000);
    expect(parsed.coin_amount).toBe(100);

    const listEnvelope = coinPackageListEnvelopeSchema.parse({
      success: true,
      data: [rawPackage],
    });
    expect(listEnvelope.data).toHaveLength(1);
    expect(listEnvelope.data[0].name).toBe("Starter Package");
  });

  it("validates transaction statuses and paginated transaction ledger", () => {
    const rawTx = {
      id: "44444444-4444-4444-8444-444444444444",
      from: null,
      to: "22222222-2222-4222-8222-222222222222",
      amount: 100,
      currency: "COIN",
      description: "Deposit: Starter Package",
      status: "success",
      payos_order_code: 987654321,
      created_at: "2026-10-11T12:00:00Z",
    };
    const parsed = transactionSchema.parse(rawTx);
    expect(parsed.status).toBe("success");
    expect(parsed.payos_order_code).toBe(987654321);

    const pageEnvelope = transactionPageEnvelopeSchema.parse({
      success: true,
      data: {
        items: [
          rawTx,
          {
            id: "55555555-5555-4555-8555-555555555555",
            from: "22222222-2222-4222-8222-222222222222",
            to: null,
            amount: 20,
            currency: "COIN",
            description: "Mock Interview Session",
            status: "pending",
            payos_order_code: null,
          },
        ],
        total: 2,
        page: 1,
        page_size: 20,
      },
    });
    expect(pageEnvelope.data.items).toHaveLength(2);
    expect(pageEnvelope.data.items[1].status).toBe("pending");
  });

  it("validates deposit checkout requests and responses", () => {
    const rawReq = {
      package_id: "33333333-3333-4333-8333-333333333333",
      return_url: "https://example.com/billing?status=success",
      cancel_url: "https://example.com/billing?cancel=true",
    };
    const parsedReq = depositCheckoutRequestSchema.parse(rawReq);
    expect(parsedReq.package_id).toBe(rawReq.package_id);

    const rawResp = {
      checkout_url: "https://pay.payos.vn/web/123456",
      order_code: 123456,
      amount: 50000,
      coin_amount: 100,
      transaction_id: "66666666-6666-4666-8666-666666666666",
    };
    const parsedEnvelope = depositCheckoutEnvelopeSchema.parse({
      success: true,
      data: rawResp,
    });
    expect(parsedEnvelope.data.checkout_url).toBe(
      "https://pay.payos.vn/web/123456",
    );
    expect(parsedEnvelope.data.order_code).toBe(123456);
  });
});
