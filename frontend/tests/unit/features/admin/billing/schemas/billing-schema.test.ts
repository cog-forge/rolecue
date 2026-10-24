import { describe, expect, it } from "vitest";
import {
  adminTransactionSchema,
  adminTransactionPageSchema,
  revenueStatsSchema,
  revenueReportSchema,
} from "@/features/admin/billing/schemas/billing-schema";
import { resolveParty } from "@/features/admin/billing/components/transaction-table";

describe("admin billing schemas", () => {
  it("validates admin transaction schema", () => {
    const raw = {
      id: "9f029054-9457-41ec-b2f5-bfe4fc41a27e",
      from: null,
      to: "33333333-3333-4333-8333-333333333333",
      amount: 500,
      currency: "COIN",
      description: "Deposit: Pro Pack (50000 VND)",
      status: "success",
      payos_order_code: 100000000001,
      created_at: "2026-10-21T10:00:00Z",
      updated_at: "2026-10-21T10:05:00Z",
    };

    const parsed = adminTransactionSchema.parse(raw);
    expect(parsed.id).toBe(raw.id);
    expect(parsed.amount).toBe(500);
    expect(parsed.status).toBe("success");
    expect(parsed.payos_order_code).toBe(100000000001);
  });

  it("validates admin transaction page schema", () => {
    const page = {
      items: [
        {
          id: "tx-1",
          amount: 100,
          currency: "COIN",
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

  it("validates revenue stats schema", () => {
    const stats = {
      total_revenue: 15000,
      successful_count: 30,
      pending_count: 5,
      failed_count: 2,
      cancelled_count: 1,
      total_count: 38,
    };

    const parsed = revenueStatsSchema.parse(stats);
    expect(parsed.total_revenue).toBe(15000);
    expect(parsed.successful_count).toBe(30);
    expect(parsed.total_count).toBe(38);
  });

  it("validates revenue report schema", () => {
    const report = {
      stats: {
        total_revenue: 2500,
        successful_count: 5,
        pending_count: 1,
        failed_count: 0,
        cancelled_count: 0,
        total_count: 6,
      },
      summary: [
        {
          period: "2026-10-21T00:00:00Z",
          revenue: 1000,
          successful_deposits: 2,
        },
      ],
    };

    const parsed = revenueReportSchema.parse(report);
    expect(parsed.stats.total_revenue).toBe(2500);
    expect(parsed.summary).toHaveLength(1);
    expect(parsed.summary[0].revenue).toBe(1000);
  });

  describe("resolveParty", () => {
    it("resolves null/empty from as Banking (PayOS) for deposits", () => {
      expect(resolveParty(null, "from", true, false)).toEqual({
        label: "Banking (PayOS)",
        isSystemOrBanking: true,
      });
      expect(resolveParty("", "from", true, false)).toEqual({
        label: "Banking (PayOS)",
        isSystemOrBanking: true,
      });
      expect(
        resolveParty(
          "00000000-0000-0000-0000-000000000000",
          "from",
          true,
          false,
        ),
      ).toEqual({
        label: "Banking (PayOS)",
        isSystemOrBanking: true,
      });
    });

    it("resolves null/empty to as System for spending", () => {
      expect(resolveParty(null, "to", false, false)).toEqual({
        label: "System",
        isSystemOrBanking: true,
      });
      expect(
        resolveParty(
          "00000000-0000-0000-0000-000000000000",
          "to",
          false,
          false,
        ),
      ).toEqual({
        label: "System",
        isSystemOrBanking: true,
      });
    });

    it("resolves null/empty from as System for refunds", () => {
      expect(resolveParty(null, "from", false, true)).toEqual({
        label: "System",
        isSystemOrBanking: true,
      });
    });

    it("preserves real user IDs", () => {
      const userId = "a7daf95f-5c68-4ef4-ad62-5166079dbff1";
      expect(resolveParty(userId, "to", true, false)).toEqual({
        label: userId,
        isSystemOrBanking: false,
      });
      expect(resolveParty(userId, "from", false, false)).toEqual({
        label: userId,
        isSystemOrBanking: false,
      });
    });
  });
});
