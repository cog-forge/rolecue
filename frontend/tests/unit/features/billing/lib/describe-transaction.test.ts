import { describe, expect, it } from "vitest";
import { describeTransaction } from "@/features/billing/lib/describe-transaction";

describe("describeTransaction", () => {
  it("turns ledger wording into what the person did", () => {
    expect(describeTransaction("Deposit: Standard (100000 VND)")).toEqual({
      kind: "topup",
      title: "Standard pack",
      detail: "100.000 ₫",
    });
    expect(
      describeTransaction(
        "Practice interview 5c1f2e8a-0000-4000-8000-000000000001",
      ),
    ).toEqual({ kind: "practice_interview", title: "Practice interview" });
    expect(
      describeTransaction(
        "Interview capacity of job posting 7b0c4d2e-0000-4000-8000-000000000002, 20 slots",
      ),
    ).toEqual({
      kind: "interview_slots",
      title: "Interview slots",
      detail: "20 slots",
    });
    expect(
      describeTransaction(
        "Refund of unused interview capacity of job posting 7b0c4d2e-0000-4000-8000-000000000001, 1 slots",
      ),
    ).toEqual({
      kind: "refund",
      title: "Refund for unused interview slots",
      detail: "1 slot",
    });
    // New rows carry the posting title; commas and colons inside it do not confuse the slot count.
    expect(
      describeTransaction(
        "Interview capacity of job posting 7b0c4d2e-0000-4000-8000-000000000002: Backend: Go, Postgres, 20 slots",
      ),
    ).toEqual({
      kind: "interview_slots",
      title: "Interview slots: Backend: Go, Postgres",
      detail: "20 slots",
    });
    expect(
      describeTransaction(
        "Refund of unused interview capacity of job posting 7b0c4d2e-0000-4000-8000-000000000001: Data Engineer, 3 slots",
      ),
    ).toEqual({
      kind: "refund",
      title: "Refund for unused slots: Data Engineer",
      detail: "3 slots",
    });
    expect(describeTransaction("Avatar inventory, 1 slots")).toEqual({
      kind: "avatar_storage",
      title: "Avatar storage",
      detail: "1 slot",
    });
  });

  it("does not repeat 'pack' when the package name already says it", () => {
    expect(describeTransaction("Deposit: Starter Pack (50000 VND)").title).toBe(
      "Starter Pack",
    );
  });

  it("keeps anything it does not recognise as written", () => {
    expect(describeTransaction("Manual adjustment")).toEqual({
      kind: "other",
      title: "Manual adjustment",
    });
    expect(describeTransaction("")).toEqual({
      kind: "other",
      title: "Coin transaction",
    });
    expect(describeTransaction(undefined)).toEqual({
      kind: "other",
      title: "Coin transaction",
    });
  });
});
