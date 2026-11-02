"use client";

import { Controller, useWatch, type UseFormReturn } from "react-hook-form";
import { Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  isCustomTarget,
  isKnownTarget,
  proficiencyLevels,
  type RoleReview,
} from "../api/contract";
import { proficiencyLabel, requirementLabel } from "../lib/labels";
import { RequirementDetails } from "./requirement-details";

export function RequirementRow({
  form,
  index,
  disabled,
  onRemove,
}: {
  form: UseFormReturn<RoleReview>;
  index: number;
  disabled: boolean;
  onRemove: () => void;
}) {
  const t = useTranslations("candidate.roleProfiles.requirement");
  const labels = useTranslations("candidate.roleProfiles.labels");
  const item = useWatch({
    control: form.control,
    name: `requirements.${index}`,
  });
  const errors = form.formState.errors.requirements?.[index];
  const prefix = `requirement-${index}`;
  const knownTarget = isKnownTarget(item.target);
  const customTarget = isCustomTarget(item);
  return (
    <fieldset className="min-w-0 border-b py-3">
      <legend className="sr-only">{t("legend", { number: index + 1 })}</legend>
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-2 sm:grid-cols-[minmax(0,1fr)_12rem_auto]">
        <div className="min-w-0">
          <label htmlFor={`${prefix}-name`} className="sr-only">
            {t("nameLabel", { number: String(index + 1).padStart(2, "0") })}
          </label>
          <Input
            id={`${prefix}-name`}
            {...form.register(`requirements.${index}.name`)}
            placeholder={t("namePlaceholder")}
            className="h-11 border-transparent px-2 text-base font-medium shadow-none hover:border-input focus-visible:border-ring"
            aria-invalid={Boolean(errors?.name)}
            aria-describedby={`${prefix}-error`}
          />
        </div>
        <div className="col-start-1 row-start-2 sm:col-start-2 sm:row-start-1">
          <label htmlFor={`${prefix}-target`} className="sr-only">
            {t("proficiencyLabel", { number: index + 1 })}
          </label>
          <Controller
            control={form.control}
            name={`requirements.${index}.target`}
            render={({ field }) => (
              <Select
                value={field.value || undefined}
                disabled={disabled}
                onValueChange={field.onChange}
              >
                <SelectTrigger
                  id={`${prefix}-target`}
                  ref={field.ref}
                  onBlur={field.onBlur}
                  className="w-full bg-muted/40 data-[size=default]:h-11"
                  aria-invalid={Boolean(errors?.target)}
                  aria-describedby={`${prefix}-error`}
                >
                  <SelectValue placeholder={t("proficiencyPlaceholder")} />
                </SelectTrigger>
                <SelectContent position="popper" align="start">
                  {proficiencyLevels.map((value) => (
                    <SelectItem key={value} value={value}>
                      {proficiencyLabel(labels, value)}
                    </SelectItem>
                  ))}
                  {item.target && !knownTarget && (
                    <SelectItem value={item.target}>
                      {customTarget
                        ? labels("savedTarget")
                        : proficiencyLabel(labels, "Unspecified")}
                    </SelectItem>
                  )}
                </SelectContent>
              </Select>
            )}
          />
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="col-start-2 row-start-1 size-11 text-muted-foreground hover:text-destructive sm:col-start-3"
          aria-label={t("remove", { number: index + 1 })}
          disabled={disabled}
          onClick={onRemove}
        >
          <Trash2 className="size-4" aria-hidden="true" />
        </Button>
      </div>
      {customTarget && (
        <p className="mt-2 px-2 text-sm text-muted-foreground">{item.target}</p>
      )}
      <Accordion type="single" collapsible className="mt-1 px-2">
        <AccordionItem value="details">
          <AccordionTrigger
            disabled={disabled}
            aria-label={t("detailsLabel", { number: index + 1 })}
            className="min-h-11 items-center gap-2 text-xs font-normal text-muted-foreground"
          >
            <span className="flex-1">{requirementLabel(labels, item)}</span>
            <span className="font-medium">{t("details")}</span>
          </AccordionTrigger>
          <AccordionContent className="space-y-4 pt-3">
            <RequirementDetails
              form={form}
              index={index}
              disabled={disabled}
              customTarget={customTarget}
            />
            {item.evidenceText && (
              <blockquote className="border-l-2 pl-3 leading-relaxed text-muted-foreground">
                <span className="mb-1 block text-xs font-medium">
                  {t("fromJd")}
                </span>
                “{item.evidenceText}”
              </blockquote>
            )}
          </AccordionContent>
        </AccordionItem>
      </Accordion>
      {(errors?.name || errors?.target || errors?.category) && (
        <p
          id={`${prefix}-error`}
          role="alert"
          className="mt-2 px-2 text-xs text-destructive"
        >
          {errors.name?.message ??
            errors.target?.message ??
            errors.category?.message}
        </p>
      )}
    </fieldset>
  );
}
