"use client";

import { useTranslations } from "next-intl";
import { BookOpen } from "lucide-react";
import { ActionSearchBar } from "@/components/ui/action-search-bar";
import type { Rubric } from "../schemas/rubric-schema";

export function RubricPicker({
  id,
  rubrics,
  value,
  onChange,
  disabled = false,
  allowClear = false,
}: {
  id: string;
  rubrics: Rubric[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  allowClear?: boolean;
}) {
  const t = useTranslations("common.rubricReview");
  return (
    <div className="min-w-0 space-y-2">
      <label htmlFor={id} className="text-xs font-medium text-muted-foreground">
        {t("rubric")}
      </label>
      <ActionSearchBar
        id={id}
        value={value}
        onValueChange={onChange}
        disabled={disabled}
        label={t("rubric")}
        placeholder={t("choose")}
        searchPlaceholder={t("search")}
        emptyMessage={t("noMatches")}
        unavailableLabel={t("unavailable")}
        options={[
          ...(allowClear ? [{ value: "", label: t("none") }] : []),
          ...rubrics.map((rubric) => ({
            value: rubric.id,
            label: rubric.name,
            icon: <BookOpen className="size-4" />,
          })),
        ]}
      />
    </div>
  );
}
