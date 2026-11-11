import { describe, expect, it } from "vitest";
import {
  avoidTopics,
  effectiveFollowUps,
  guidelineFormSchema,
  guidelinesEnvelopeSchema,
  toFormValues,
  toRequestValues,
  type GuidelineValues,
} from "@/features/admin/guidelines/schemas/guidelines-schema";

const defaults: GuidelineValues = {
  tone: "neutral",
  question_length: "standard",
  follow_up_max: null,
  follow_up_focus: "standard",
  avoid_topics: [],
};

describe("guideline form mapping", () => {
  it("turns a null maximum into the default choice and back", () => {
    expect(toFormValues(defaults).follow_up_max).toBe("default");
    expect(toRequestValues(toFormValues(defaults))).toEqual(defaults);
  });

  it("keeps a zero maximum distinct from the default", () => {
    const values = { ...defaults, follow_up_max: 0 };
    expect(toFormValues(values).follow_up_max).toBe("0");
    expect(toRequestValues(toFormValues(values)).follow_up_max).toBe(0);
  });

  it("sends the topics in catalog order whatever order they were picked in", () => {
    const form = {
      ...toFormValues(defaults),
      avoid_topics: ["location_nationality", "compensation"] as const,
    };
    expect(
      toRequestValues({ ...form, avoid_topics: [...form.avoid_topics] }),
    ).toMatchObject({ avoid_topics: ["compensation", "location_nationality"] });
    expect(avoidTopics).toHaveLength(5);
  });

  it("refuses values outside the vocabulary", () => {
    const form = toFormValues(defaults);
    expect(guidelineFormSchema.safeParse(form).success).toBe(true);
    expect(
      guidelineFormSchema.safeParse({ ...form, tone: "sarcastic" }).success,
    ).toBe(false);
    expect(
      guidelineFormSchema.safeParse({ ...form, follow_up_max: "4" }).success,
    ).toBe(false);
    expect(
      guidelineFormSchema.safeParse({ ...form, avoid_topics: ["salary"] })
        .success,
    ).toBe(false);
  });
});

describe("effective follow-ups", () => {
  it("lowers the server ceiling and never raises it", () => {
    expect(effectiveFollowUps("default", 2)).toBe(2);
    expect(effectiveFollowUps("1", 3)).toBe(1);
    expect(effectiveFollowUps("3", 1)).toBe(1);
    expect(effectiveFollowUps("0", 3)).toBe(0);
  });
});

describe("guidelines response", () => {
  const response = {
    success: true,
    data: {
      version: 0,
      values: defaults,
      defaults,
      allowed: { tone: ["neutral"] },
      effective_follow_up_ceiling: 1,
      updated_at: null,
      updated_by_name: "",
    },
  };

  it("parses the nothing-saved-yet answer", () => {
    expect(guidelinesEnvelopeSchema.parse(response).data.version).toBe(0);
  });

  it("parses a saved version with a Go timestamp", () => {
    const saved = guidelinesEnvelopeSchema.parse({
      ...response,
      data: {
        ...response.data,
        version: 3,
        updated_at: "2026-11-11T13:05:55.020123+07:00",
        updated_by_name: "Ada Admin",
      },
    });
    expect(saved.data.updated_by_name).toBe("Ada Admin");
  });

  it("rejects a value the form could not show", () => {
    expect(
      guidelinesEnvelopeSchema.safeParse({
        ...response,
        data: { ...response.data, values: { ...defaults, tone: "rude" } },
      }).success,
    ).toBe(false);
  });
});
