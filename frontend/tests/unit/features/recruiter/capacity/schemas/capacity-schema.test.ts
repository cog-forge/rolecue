import { describe, expect, it } from "vitest";
import {
  capacityStateSchema,
  fundCapacitySchema,
} from "@/features/recruiter/capacity/schemas/capacity-schema";

const state = {
  interview_slot: 5,
  funded: true,
  price_per_slot: 1,
  cv_approvals: 2,
  consumed: 1,
  available_to_approve: 3,
  refundable: 4,
  posting_status: "open",
  refunded: null,
};

describe("capacityStateSchema", () => {
  it("accepts the CapacityState contract", () => {
    expect(capacityStateSchema.parse(state)).toEqual(state);
  });

  it("accepts an unfunded posting with no slot", () => {
    expect(
      capacityStateSchema.parse({
        ...state,
        interview_slot: null,
        funded: false,
      }).interview_slot,
    ).toBeNull();
  });

  it("rejects an unknown posting status", () => {
    expect(() =>
      capacityStateSchema.parse({ ...state, posting_status: "archived" }),
    ).toThrow();
  });
});

describe("fundCapacitySchema", () => {
  it.each([
    ["1", 1],
    [" 12 ", 12],
    ["10000", 10_000],
  ])("turns %j into the slot count %i", (input, expected) => {
    expect(fundCapacitySchema.parse({ slots: input }).slots).toBe(expected);
  });

  it.each([
    ["", "Enter a whole number of slots."],
    ["0", "Fund at least 1 slot."],
    ["10001", "Fund at most 10,000 slots."],
    ["1.5", "Enter a whole number of slots."],
    ["-2", "Enter a whole number of slots."],
  ])("rejects %j", (input, message) => {
    const result = fundCapacitySchema.safeParse({ slots: input });
    expect(result.success).toBe(false);
    if (!result.success) expect(result.error.issues[0]?.message).toBe(message);
  });
});
