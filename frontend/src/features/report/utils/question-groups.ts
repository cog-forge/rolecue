/** A stored turn as the report and the Recruiter's evidence read it (KAN-56). */
type GroupableTurn = { kind: "core" | "follow_up"; question_number: number };

export type QuestionGroup<T> = { core: T; followUps: T[] };

/**
 * Groups turns (in position order) under their core question: each follow-up follows the core
 * question with the same `question_number`. A follow-up without its core question (not expected)
 * stands on its own rather than being dropped.
 */
export function groupByQuestion<T extends GroupableTurn>(
  turns: readonly T[],
): QuestionGroup<T>[] {
  const groups: QuestionGroup<T>[] = [];
  for (const turn of turns) {
    const last = groups.at(-1);
    if (
      turn.kind === "follow_up" &&
      last?.core.question_number === turn.question_number
    )
      last.followUps.push(turn);
    else groups.push({ core: turn, followUps: [] });
  }
  return groups;
}
