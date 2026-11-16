import { Circle, CircleCheck, CircleDashed } from "lucide-react";
import { useTranslations } from "next-intl";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import type { RoadmapItem } from "../api/roadmap";

export function TopicStatus({ item }: { item: RoadmapItem }) {
  const t = useTranslations("candidate.roadmap");
  const Icon =
    item.learning_status === "achieved"
      ? CircleCheck
      : item.learning_status === "in_progress"
        ? CircleDashed
        : Circle;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-xs font-medium",
        item.learning_status === "achieved"
          ? "text-emerald-700 dark:text-emerald-400"
          : "text-muted-foreground",
      )}
    >
      <Icon className="size-3.5 shrink-0" aria-hidden="true" />
      {t(`status.${item.learning_status}`)}
    </span>
  );
}
export function TopicScore({
  item,
  windowSize,
  threshold,
}: {
  item: RoadmapItem;
  windowSize: number;
  threshold: number;
}) {
  const t = useTranslations("candidate.roadmap");
  const fmt = useFormatters();
  return (
    <div className="space-y-1 text-sm">
      <p className="font-medium tabular-nums">
        {item.score === null
          ? t("notEvaluated")
          : t("score", { score: fmt.number(item.score) })}
      </p>
      {item.score !== null && item.evidence_question_count < windowSize && (
        <p className="text-xs text-muted-foreground">{t("provisional")}</p>
      )}
      {item.score !== null &&
        item.score >= threshold &&
        item.evidence_question_count >= windowSize &&
        item.learning_status !== "achieved" && (
          <p className="text-xs text-amber-800 dark:text-amber-300">
            {t("belowThreshold", { threshold })}
          </p>
        )}
    </div>
  );
}
