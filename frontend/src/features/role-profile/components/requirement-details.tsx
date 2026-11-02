"use client";

import { Controller, useWatch, type UseFormReturn } from "react-hook-form";
import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { categorySchema, type RoleReview } from "../api/contract";
import { categoryLabel, kindLabel, priorityLabel } from "../lib/labels";

export function RequirementDetails({
  form,
  index,
  disabled,
  customTarget,
}: {
  form: UseFormReturn<RoleReview>;
  index: number;
  disabled: boolean;
  customTarget: boolean;
}) {
  const t = useTranslations("candidate.roleProfiles.requirement");
  const labels = useTranslations("candidate.roleProfiles.labels");
  const prefix = `requirement-${index}`;
  const error = form.formState.errors.requirements?.[index];
  const kind = useWatch({
    control: form.control,
    name: `requirements.${index}.kind`,
  });
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <div className="space-y-2">
        <label
          htmlFor={`${prefix}-kind`}
          className="text-xs text-muted-foreground"
        >
          {t("kind")}
        </label>
        <Controller
          control={form.control}
          name={`requirements.${index}.kind`}
          render={({ field }) => (
            <Select
              value={field.value}
              disabled={disabled}
              onValueChange={(
                value: RoleReview["requirements"][number]["kind"],
              ) => {
                field.onChange(value);
                form.setValue(
                  `requirements.${index}.category`,
                  value === "skill" ? "other" : null,
                  { shouldDirty: true, shouldValidate: true },
                );
              }}
            >
              <SelectTrigger
                id={`${prefix}-kind`}
                ref={field.ref}
                onBlur={field.onBlur}
                className="w-full data-[size=default]:h-11"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                {(["skill", "technology", "domain_knowledge"] as const).map(
                  (kind) => (
                    <SelectItem key={kind} value={kind}>
                      {kindLabel(labels, kind)}
                    </SelectItem>
                  ),
                )}
              </SelectContent>
            </Select>
          )}
        />
      </div>
      {kind === "skill" && (
        <div className="space-y-2">
          <label
            htmlFor={`${prefix}-category`}
            className="text-xs text-muted-foreground"
          >
            {t("category")}
          </label>
          <Controller
            control={form.control}
            name={`requirements.${index}.category`}
            render={({ field }) => (
              <Select
                value={field.value ?? undefined}
                disabled={disabled}
                onValueChange={field.onChange}
              >
                <SelectTrigger
                  id={`${prefix}-category`}
                  ref={field.ref}
                  onBlur={field.onBlur}
                  className="w-full data-[size=default]:h-11"
                  aria-invalid={Boolean(error?.category)}
                >
                  <SelectValue placeholder={t("categoryPlaceholder")} />
                </SelectTrigger>
                <SelectContent position="popper" align="start">
                  {categorySchema.options.map((value) => (
                    <SelectItem key={value} value={value}>
                      {categoryLabel(labels, value)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
      )}
      <div className="space-y-2">
        <label
          htmlFor={`${prefix}-type`}
          className="text-xs text-muted-foreground"
        >
          {t("priority")}
        </label>
        <Controller
          control={form.control}
          name={`requirements.${index}.type`}
          render={({ field }) => (
            <Select
              value={field.value ?? "unspecified"}
              disabled={disabled}
              onValueChange={(value) =>
                field.onChange(value === "unspecified" ? null : value)
              }
            >
              <SelectTrigger
                id={`${prefix}-type`}
                ref={field.ref}
                onBlur={field.onBlur}
                className="w-full data-[size=default]:h-11"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                {(["unspecified", "required", "preferred"] as const).map(
                  (priority) => (
                    <SelectItem key={priority} value={priority}>
                      {priorityLabel(labels, priority)}
                    </SelectItem>
                  ),
                )}
              </SelectContent>
            </Select>
          )}
        />
      </div>
      {customTarget && (
        <div className="space-y-2 sm:col-span-2 xl:col-span-3">
          <label
            htmlFor={`${prefix}-saved-target`}
            className="text-xs text-muted-foreground"
          >
            {t("savedTarget")}
          </label>
          <Input
            id={`${prefix}-saved-target`}
            {...form.register(`requirements.${index}.target`)}
            className="h-11"
            aria-invalid={Boolean(error?.target)}
          />
          <p className="text-xs text-muted-foreground">
            {t("savedTargetHelp")}
          </p>
        </div>
      )}
    </div>
  );
}
