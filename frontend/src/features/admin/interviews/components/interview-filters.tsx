"use client";

import { useId, useMemo } from "react";
import { useTranslations } from "next-intl";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
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
  createInterviewFilterSchema,
  type InterviewFilterValues,
} from "../schemas/interview-filters";
import type { AdminInterviewQuery } from "../api/interviews";

const emptyFilters: InterviewFilterValues = {
  type: "all",
  candidateId: "",
  jobPostingId: "",
};

export function InterviewFilters({
  query,
  disabled,
  onApply,
}: {
  query: AdminInterviewQuery;
  disabled: boolean;
  onApply: (values: InterviewFilterValues) => void;
}) {
  const t = useTranslations("admin.interviews.filters");
  const v = useTranslations("admin.interviews.validation");
  const schema = useMemo(() => createInterviewFilterSchema(v), [v]);
  const types = useTranslations("admin.interviews.type");
  const ids = useId();
  const {
    register,
    control,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<InterviewFilterValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      type: query.type ?? "all",
      candidateId: query.candidateId ?? "",
      jobPostingId: query.jobPostingId ?? "",
    },
  });
  const clear = () => {
    reset(emptyFilters);
    onApply(emptyFilters);
  };
  return (
    <form
      aria-label={t("label")}
      noValidate
      onSubmit={handleSubmit((values) => onApply(values))}
      className="grid gap-3 sm:grid-cols-[12rem_minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start"
    >
      <div className="flex flex-col gap-2">
        <label htmlFor={`${ids}-type`} className="text-xs font-medium">
          {t("type")}
        </label>
        <Controller
          control={control}
          name="type"
          render={({ field }) => (
            <Select
              value={field.value}
              disabled={disabled}
              onValueChange={field.onChange}
            >
              <SelectTrigger
                id={`${ids}-type`}
                className="w-full bg-background shadow-none data-[size=default]:h-11"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem value="all">{t("allTypes")}</SelectItem>
                <SelectItem value="practice">{types("practice")}</SelectItem>
                <SelectItem value="recruitment">
                  {types("recruitment")}
                </SelectItem>
              </SelectContent>
            </Select>
          )}
        />
      </div>
      <div className="flex flex-col gap-2">
        <label htmlFor={`${ids}-candidate`} className="text-xs font-medium">
          {t("candidateId")}
        </label>
        <Input
          id={`${ids}-candidate`}
          className="h-11 bg-background shadow-none"
          placeholder={t("candidatePlaceholder")}
          autoComplete="off"
          disabled={disabled}
          aria-invalid={errors.candidateId ? true : undefined}
          aria-describedby={
            errors.candidateId ? `${ids}-candidate-error` : undefined
          }
          {...register("candidateId")}
        />
        {errors.candidateId && (
          <p id={`${ids}-candidate-error`} className="text-xs text-destructive">
            {errors.candidateId.message}
          </p>
        )}
      </div>
      <div className="flex flex-col gap-2">
        <label htmlFor={`${ids}-posting`} className="text-xs font-medium">
          {t("jobPostingId")}
        </label>
        <Input
          id={`${ids}-posting`}
          className="h-11 bg-background shadow-none"
          placeholder={t("jobPostingPlaceholder")}
          autoComplete="off"
          disabled={disabled}
          aria-invalid={errors.jobPostingId ? true : undefined}
          aria-describedby={
            errors.jobPostingId ? `${ids}-posting-error` : undefined
          }
          {...register("jobPostingId")}
        />
        {errors.jobPostingId && (
          <p id={`${ids}-posting-error`} className="text-xs text-destructive">
            {errors.jobPostingId.message}
          </p>
        )}
      </div>
      <div className="flex flex-wrap gap-2 sm:pt-6">
        <Button type="submit" className="h-11" disabled={disabled}>
          {t("apply")}
        </Button>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={disabled}
          onClick={clear}
        >
          {t("clear")}
        </Button>
      </div>
    </form>
  );
}
