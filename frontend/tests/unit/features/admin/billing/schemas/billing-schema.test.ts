import { describe, expect, it } from "vitest";
import {
  adminTransactionSchema,
  adminTransactionPageSchema,
  billingSummarySchema,
} from "@/features/admin/billing/schemas/billing-schema";

describe("admin billing schemas", () => {
  it("validates admin transaction schema", () => {
    const raw = {
      id: "9f029054-9457-41ec-b2f5-bfe4fc41a27e",
      from: "",
      to: "33333333-3333-4333-8333-333333333333",
      user_id: "33333333-3333-4333-8333-333333333333",
      user_name: "Alice Nguyen",
      user_email: "alice@example.com",
      type: "topup",
      package_or_feature: "Pro Pack",
      paid_vnd: 50000,
      coin_amount: 100,
      amount: 100,
      currency: "VND",
      description: "Deposit: Pro Pack (50000 VND)",
      status: "success",
      payos_order_code: 100000000001,
    };

    const parsed = adminTransactionSchema.parse(raw);
    expect(parsed.id).toBe(raw.id);
    expect(parsed.paid_vnd).toBe(50000);
    expect(parsed.coin_amount).toBe(100);
    expect(parsed.status).toBe("success");
    expect(parsed.payos_order_code).toBe(100000000001);
  });

  it("keeps the transaction created_at and tolerates an older API without it", () => {
    const base = {
      id: "tx-1",
      amount: 100,
      status: "success",
    };

    expect(
      adminTransactionSchema.parse({
        ...base,
        created_at: "2026-10-27T06:16:40Z",
      }).created_at,
    ).toBe("2026-10-27T06:16:40Z");
    expect(adminTransactionSchema.parse(base).created_at).toBe("");
  });

  it("validates admin transaction page schema", () => {
    const page = {
      items: [
        {
          id: "tx-1",
          amount: 100,
          currency: "VND",
          status: "pending",
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    };

    const parsed = adminTransactionPageSchema.parse(page);
    expect(parsed.total).toBe(1);
    expect(parsed.items).toHaveLength(1);
  });

  it("validates billing summary schema", () => {
    const summary = {
      total_revenue: 150000,
      successful_deposits: 3,
      total_deposits: 4,
      success_count: 5,
      pending_count: 2,
      failed_count: 1,
      cancelled_count: 1,
      total_transactions: 9,
      average_deposit: 50000,
      unique_payers: 2,
      coins_sold: 300,
      coins_spent: 40,
      coins_refunded: 10,
      coins_net: 270,
      success_rate: 55.6,
      package_breakdown: [
        {
          package_name: "Starter Pack",
          revenue_vnd: 150000,
          deposit_count: 3,
          coins_sold: 300,
        },
      ],
      feature_breakdown: [
        {
          feature_name: "Practice interview charge",
          coins_spent: 40,
          spend_count: 2,
        },
      ],
    };

    const parsed = billingSummarySchema.parse(summary);
    expect(parsed.total_revenue).toBe(150000);
    expect(parsed.successful_deposits).toBe(3);
    expect(parsed.total_deposits).toBe(4);
    expect(parsed.total_transactions).toBe(9);
    expect(parsed.package_breakdown).toHaveLength(1);
    expect(parsed.feature_breakdown).toHaveLength(1);
  });
});
