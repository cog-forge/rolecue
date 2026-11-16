import type { Roadmap, RoadmapItem } from "../api/roadmap";

// Suggestions guide practice; the server's learning status remains authoritative.
export function nextPracticeTopic(roadmap: Roadmap): RoadmapItem | undefined {
  if (roadmap.generation_status !== "ready") return undefined;
  const priority = (item: RoadmapItem) =>
    item.score !== null && item.evidence_question_count < roadmap.window_size
      ? 0
      : item.learning_status === "in_progress"
        ? 1
        : 2;
  return roadmap.items
    .filter((item) => item.learning_status !== "achieved")
    .sort(
      (a, b) =>
        priority(a) - priority(b) ||
        (a.score ?? 101) - (b.score ?? 101) ||
        a.position - b.position ||
        a.id.localeCompare(b.id),
    )[0];
}
