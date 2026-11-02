import { describe, expect, it } from "vitest";
import { createTranslator } from "next-intl";
import { describeTransaction } from "@/features/billing/lib/describe-transaction";
import { formattersFor, messagesFor } from "@tests/helpers/intl";

const t = createTranslator({
  locale: "vi",
  messages: messagesFor("vi"),
  namespace: "candidate.billing.describe",
});
const fmt = formattersFor("vi");

describe("describeTransaction in Vietnamese", () => {
  it("translates the labels and keeps posting titles and package names as stored", () => {
    expect(
      describeTransaction("Deposit: Standard (100000 VND)", t, fmt),
    ).toEqual({ kind: "topup", title: "Gói Standard", detail: "100.000 ₫" });
    expect(
      describeTransaction("Deposit: Starter Pack (50000 VND)", t, fmt).title,
    ).toBe("Starter Pack");
    expect(
      describeTransaction(
        "Practice interview 5c1f2e8a-0000-4000-8000-000000000001",
        t,
        fmt,
      ),
    ).toEqual({ kind: "practice_interview", title: "Phỏng vấn luyện tập" });
    expect(
      describeTransaction(
        "Interview capacity of job posting 7b0c4d2e-0000-4000-8000-000000000002: Backend: Go, Postgres, 20 slots",
        t,
        fmt,
      ),
    ).toEqual({
      kind: "interview_slots",
      title: "Lượt phỏng vấn: Backend: Go, Postgres",
      detail: "20 lượt",
    });
    expect(
      describeTransaction(
        "Refund of unused interview capacity of job posting 7b0c4d2e-0000-4000-8000-000000000001, 1 slots",
        t,
        fmt,
      ),
    ).toEqual({
      kind: "refund",
      title: "Hoàn lượt phỏng vấn chưa dùng",
      detail: "1 lượt",
    });
    expect(describeTransaction("Avatar inventory, 3 slots", t, fmt)).toEqual({
      kind: "avatar_storage",
      title: "Dung lượng lưu trữ avatar",
      detail: "3 ô",
    });
    expect(describeTransaction("Manual adjustment", t, fmt).title).toBe(
      "Manual adjustment",
    );
    expect(describeTransaction("", t, fmt).title).toBe("Giao dịch xu");
  });
});
