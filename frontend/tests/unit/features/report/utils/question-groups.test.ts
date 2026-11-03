import { describe, expect, it } from "vitest";
import { groupByQuestion } from "@/features/report/utils/question-groups";

const turn = (
  position: number,
  question_number: number,
  kind: "core" | "follow_up" = "core",
) => ({ position, question_number, kind });

describe("groupByQuestion", () => {
  it("puts each follow-up under its core question", () => {
    const groups = groupByQuestion([
      turn(1, 1),
      turn(2, 1, "follow_up"),
      turn(3, 2),
      turn(4, 3),
      turn(5, 3, "follow_up"),
      turn(6, 3, "follow_up"),
    ]);
    expect(
      groups.map(({ core, followUps }) => [
        core.position,
        followUps.map((item) => item.position),
      ]),
    ).toEqual([
      [1, [2]],
      [3, []],
      [4, [5, 6]],
    ]);
  });

  it("keeps a session without follow-ups as one group per question", () => {
    expect(groupByQuestion([turn(1, 1), turn(2, 2)])).toHaveLength(2);
    expect(groupByQuestion([])).toEqual([]);
  });

  it("never drops a follow-up whose core question is missing", () => {
    const groups = groupByQuestion([turn(2, 1, "follow_up"), turn(3, 2)]);
    expect(groups.map(({ core }) => core.position)).toEqual([2, 3]);
  });
});
