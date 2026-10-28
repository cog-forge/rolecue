import type { RevenueReport } from "@/features/admin/billing/schemas/billing-schema";

// 1–27 Oct 2026 against 1–27 Oct 2025, as the API returns it for the review seed.
export const octoberReport: RevenueReport = {
  compare: "previous_year",
  granularity: "day",
  window: { from: "2026-09-30T17:00:00Z", to: "2026-10-27T17:00:00Z" },
  comparison_window: {
    from: "2025-09-30T17:00:00Z",
    to: "2025-10-27T17:00:00Z",
  },
  current: {
    revenue_vnd: 200000,
    successful_topups: 3,
    topup_attempts: 6,
    payment_success_rate: 50,
    paying_users: 2,
    new_payers: 0,
    average_order_vnd: 66666,
    coins_sold: 200,
    coins_spent: 36,
    coins_refunded: 4,
  },
  comparison: {
    revenue_vnd: 150000,
    successful_topups: 2,
    topup_attempts: 3,
    payment_success_rate: 66.7,
    paying_users: 2,
    new_payers: 0,
    average_order_vnd: 75000,
    coins_sold: 150,
    coins_spent: 35,
    coins_refunded: 0,
  },
  series: Array.from({ length: 27 }, (_, i) => {
    const day = String(i + 1).padStart(2, "0");
    const current = { 5: 100000, 21: 50000, 22: 50000 }[i + 1] ?? 0;
    const comparison = { 3: 50000, 12: 100000 }[i + 1] ?? 0;
    return {
      start: `2026-10-${day}T00:00:00+07:00`,
      comparison_start: `2025-10-${day}T00:00:00+07:00`,
      revenue_vnd: current,
      comparison_revenue_vnd: comparison,
    };
  }),
  by_role: [
    {
      key: "recruiter",
      revenue_vnd: 150000,
      comparison_revenue_vnd: 100000,
      topups: 2,
    },
    {
      key: "candidate",
      revenue_vnd: 50000,
      comparison_revenue_vnd: 50000,
      topups: 1,
    },
  ],
  by_package: [
    {
      key: "Standard",
      revenue_vnd: 100000,
      comparison_revenue_vnd: 100000,
      topups: 1,
    },
    {
      key: "Starter",
      revenue_vnd: 100000,
      comparison_revenue_vnd: 50000,
      topups: 2,
    },
  ],
  spend_by_feature: [
    { feature: "interview_slots", coins: 20, comparison_coins: 30, charges: 1 },
    {
      feature: "practice_interview",
      coins: 15,
      comparison_coins: 5,
      charges: 3,
    },
    { feature: "avatar_storage", coins: 1, comparison_coins: 0, charges: 1 },
  ],
  outstanding_coins: 1258,
  outstanding_value_vnd: 1258000,
};
