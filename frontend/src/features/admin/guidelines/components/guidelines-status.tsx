import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Guidelines } from "../schemas/guidelines-schema";

export function GuidelinesStatus({ guidelines }: { guidelines: Guidelines }) {
  const t = useTranslations("admin.interviewGuidelines.status");
  const { date } = useFormatters();
  const saved = guidelines.version > 0;
  const savedDate = guidelines.updated_at
    ? date(guidelines.updated_at, "localDateTime")
    : null;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-xs text-muted-foreground">
      <Badge variant="outline" className="h-6 gap-1.5 rounded-md font-normal">
        <span
          aria-hidden
          className="size-1.5 rounded-full bg-workspace-accent"
        />
        {saved ? t("version", { version: guidelines.version }) : t("notSaved")}
      </Badge>
      {!saved && <span>{t("notSavedHint")}</span>}
      {savedDate && (
        <span>
          {guidelines.updated_by_name
            ? t("savedBy", {
                name: guidelines.updated_by_name,
                date: savedDate,
              })
            : t("savedAt", { date: savedDate })}
        </span>
      )}
    </div>
  );
}
