import { describe, expect, it } from "vitest";
import { createRubricFormSchema } from "@/features/rubrics/rubric-schema";
const metric = "11111111-1111-4111-8111-111111111111";
const schema = createRubricFormSchema("required", "invalid", [metric]);
const valid = {
  name: "  Concept  ",
  description: "",
  question_type: "concept_explanation",
  criteria: [
    {
      metric_id: metric,
      name: "Accuracy",
      max_points: 3,
      scoring_guide: "0: absent; 1: partial; 2: mostly correct; 3: complete",
    },
  ],
};
describe("rubric form contract", () => {
  it("trims content and accepts catalog metrics", () =>
    expect(schema.parse(valid).name).toBe("Concept"));
  it.each([0, 11, 1.5, NaN])("rejects invalid maximum %s", (max_points) =>
    expect(
      schema.safeParse({
        ...valid,
        criteria: [{ ...valid.criteria[0], max_points }],
      }).success,
    ).toBe(false),
  );
  it("rejects unknown metric IDs", () =>
    expect(
      schema.safeParse({
        ...valid,
        criteria: [
          {
            ...valid.criteria[0],
            metric_id: "22222222-2222-4222-8222-222222222222",
          },
        ],
      }).success,
    ).toBe(false));
  it("requires one to ten criteria", () => {
    for (const criteria of [
      [],
      Array.from({ length: 11 }, () => valid.criteria[0]),
    ])
      expect(schema.safeParse({ ...valid, criteria }).success).toBe(false);
  });
  it("rejects blank guides, overlong names and invalid question types", () => {
    expect(
      schema.safeParse({
        ...valid,
        criteria: [{ ...valid.criteria[0], scoring_guide: " " }],
      }).success,
    ).toBe(false);
    expect(schema.safeParse({ ...valid, name: "a".repeat(121) }).success).toBe(
      false,
    );
    expect(
      schema.safeParse({ ...valid, question_type: "Invalid Type" }).success,
    ).toBe(false);
  });
  it("preserves criterion identities on update", () =>
    expect(
      schema.parse({
        ...valid,
        criteria: [{ ...valid.criteria[0], id: metric }],
      }).criteria[0].id,
    ).toBe(metric));
});
