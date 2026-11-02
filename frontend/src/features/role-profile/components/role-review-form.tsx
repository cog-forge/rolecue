"use client";

import { useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Controller, useFieldArray, useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowRight, Check, LoaderCircle, Plus, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  createReviewSchema,
  type RoleProfile,
  type RoleReview,
} from "../api/contract";
import { seniorityLabel } from "../lib/labels";
import { useRoleError } from "../lib/use-role-error";
import { RequirementRow } from "./requirement-row";

export function RoleReviewForm({
  profile,
  busy,
  onSave,
  onConfirm,
}: {
  profile: RoleProfile;
  busy: boolean;
  onSave: (review: RoleReview) => Promise<RoleProfile>;
  onConfirm: (review: RoleReview) => Promise<RoleProfile>;
}) {
  const t = useTranslations("candidate.roleProfiles");
  const labels = useTranslations("candidate.roleProfiles.labels");
  const roleError = useRoleError();
  const reviewSchema = useMemo(
    () => createReviewSchema((key, values) => t(`validation.${key}`, values)),
    [t],
  );
  const form = useForm<RoleReview>({
    resolver: zodResolver(reviewSchema),
    defaultValues: {
      title: profile.title,
      seniorityLevel: profile.seniorityLevel,
      requirements: profile.requirements,
      refinementNote: profile.refinementNote,
    },
  });
  const rows = useFieldArray({ control: form.control, name: "requirements" });
  const note = useWatch({ control: form.control, name: "refinementNote" });
  const lock = useRef(false);
  const [action, setAction] = useState<"save" | "confirm" | null>(null);
  const submit = async (values: RoleReview, confirming: boolean) => {
    if (lock.current) return;
    if (confirming && !values.seniorityLevel) {
      form.setError(
        "seniorityLevel",
        { message: t("review.seniorityRequired") },
        { shouldFocus: true },
      );
      return;
    }
    lock.current = true;
    setAction(confirming ? "confirm" : "save");
    form.clearErrors("root");
    try {
      const saved = await (confirming ? onConfirm(values) : onSave(values));
      form.reset({
        title: saved.title,
        seniorityLevel: saved.seniorityLevel,
        requirements: saved.requirements,
        refinementNote: saved.refinementNote,
      });
    } catch (error) {
      form.setError("root", { message: roleError(error) });
    } finally {
      lock.current = false;
      setAction(null);
    }
  };
  const disabled = busy || form.formState.isSubmitting;
  return (
    <form
      onSubmit={(event) =>
        void form.handleSubmit((values) => submit(values, false))(event)
      }
      className="space-y-7"
    >
      <div className="flex items-start gap-3 text-sm text-muted-foreground">
        <Check
          className="mt-0.5 size-4 shrink-0 text-workspace-accent"
          aria-hidden="true"
        />
        <p>{t("review.intro")}</p>
      </div>
      <fieldset disabled={disabled} className="min-w-0 space-y-7">
        <legend className="sr-only">{t("review.legend")}</legend>
        <div className="grid gap-5 sm:grid-cols-[minmax(0,1fr)_14rem]">
          <div className="space-y-2">
            <label
              htmlFor="role-title"
              className="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
            >
              {t("review.roleTitle")}
            </label>
            <Input
              id="role-title"
              className="h-11 bg-card"
              {...form.register("title")}
              aria-invalid={Boolean(form.formState.errors.title)}
              aria-describedby="role-title-error"
            />
            <p id="role-title-error" className="text-xs text-destructive">
              {form.formState.errors.title?.message}
            </p>
          </div>
          <div className="space-y-2">
            <label
              htmlFor="role-seniority"
              className="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
            >
              {t("review.seniority")}
            </label>
            <Controller
              control={form.control}
              name="seniorityLevel"
              render={({ field }) => (
                <Select
                  value={field.value ?? "unspecified"}
                  disabled={disabled}
                  onValueChange={(value) =>
                    field.onChange(value === "unspecified" ? null : value)
                  }
                >
                  <SelectTrigger
                    id="role-seniority"
                    ref={field.ref}
                    onBlur={field.onBlur}
                    className="w-full bg-card data-[size=default]:h-11"
                    aria-invalid={Boolean(form.formState.errors.seniorityLevel)}
                    aria-describedby="role-seniority-error"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start">
                    <SelectItem value="unspecified">
                      {t("review.chooseLevel")}
                    </SelectItem>
                    {(
                      ["intern", "junior", "mid", "senior", "lead"] as const
                    ).map((level) => (
                      <SelectItem key={level} value={level}>
                        {seniorityLabel(labels, level)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            <p
              id="role-seniority-error"
              role={form.formState.errors.seniorityLevel ? "alert" : undefined}
              className="text-xs text-destructive"
            >
              {form.formState.errors.seniorityLevel?.message}
            </p>
          </div>
        </div>
        <section>
          <div className="flex flex-wrap items-center justify-between gap-3 border-b pb-4">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold tracking-tight">
                {t("review.requirementsTitle")}{" "}
                <span className="ml-2 text-sm font-normal text-muted-foreground">
                  {rows.fields.length}
                </span>
              </h2>
              <p className="text-sm text-muted-foreground">
                {t("review.requirementsHelp")}
              </p>
            </div>
            <Button
              type="button"
              variant="outline"
              className="h-auto min-h-11 py-2 whitespace-normal"
              disabled={rows.fields.length >= 200}
              onClick={() =>
                rows.append({
                  name: "",
                  target: "Unspecified",
                  kind: "skill",
                  category: "other",
                  type: null,
                  evidenceText: null,
                })
              }
            >
              <Plus className="size-4" aria-hidden="true" />{" "}
              {t("review.addRequirement")}
            </Button>
          </div>
          {rows.fields.map((field, index) => (
            <RequirementRow
              key={field.id}
              form={form}
              index={index}
              disabled={disabled}
              onRemove={() => rows.remove(index)}
            />
          ))}
          {form.formState.errors.requirements?.root?.message && (
            <p role="alert" className="mt-3 text-sm text-destructive">
              {form.formState.errors.requirements.root.message}
            </p>
          )}
          {form.formState.errors.requirements?.message && (
            <p role="alert" className="mt-3 text-sm text-destructive">
              {form.formState.errors.requirements.message}
            </p>
          )}
        </section>
        <div className="space-y-3 rounded-xl border bg-card p-5 sm:p-6">
          <label htmlFor="refinement-note" className="font-semibold">
            {t("review.noteLabel")}
          </label>
          <p
            id="refinement-help"
            className="text-sm leading-relaxed text-muted-foreground"
          >
            {t("review.noteHelp")}
          </p>
          <Textarea
            id="refinement-note"
            {...form.register("refinementNote")}
            rows={4}
            placeholder={t("review.notePlaceholder")}
            aria-invalid={Boolean(form.formState.errors.refinementNote)}
            aria-describedby="refinement-help refinement-error"
          />
          <div className="flex flex-wrap justify-between gap-3 text-xs text-muted-foreground">
            <p id="refinement-error" className="text-destructive">
              {form.formState.errors.refinementNote?.message}
            </p>
            <span>
              {t("review.noteCounter", { count: String([...note].length) })}
            </span>
          </div>
        </div>
      </fieldset>
      {form.formState.errors.root && (
        <p
          role="alert"
          className="rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm"
        >
          {form.formState.errors.root.message}
        </p>
      )}
      <footer className="flex flex-wrap items-center justify-between gap-5 border-t pt-6">
        <div className="max-w-md space-y-1">
          <p className="text-sm text-muted-foreground">
            {t("review.confirmNote")}
          </p>
          <p
            role="status"
            aria-live="polite"
            className="text-xs text-muted-foreground"
          >
            {disabled
              ? action === "confirm"
                ? t("review.status.confirming")
                : t("review.status.saving")
              : form.formState.isDirty
                ? t("review.status.unsaved")
                : t("review.status.saved")}
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button
            type="submit"
            variant="outline"
            disabled={disabled}
            className="h-auto min-h-11 py-2 whitespace-normal"
          >
            {action === "save" ? (
              <LoaderCircle
                className="size-4 motion-safe:animate-spin"
                aria-hidden="true"
              />
            ) : (
              <Save className="size-4" aria-hidden="true" />
            )}
            {action === "save" ? t("review.saveBusy") : t("review.save")}
          </Button>
          <Button
            type="button"
            disabled={disabled}
            className="h-auto min-h-11 py-2 whitespace-normal"
            onClick={() =>
              void form.handleSubmit((values) => submit(values, true))()
            }
          >
            {action === "confirm" ? (
              <LoaderCircle
                className="size-4 motion-safe:animate-spin"
                aria-hidden="true"
              />
            ) : (
              <ArrowRight className="size-4" aria-hidden="true" />
            )}
            {action === "confirm"
              ? t("review.confirmBusy")
              : t("review.confirm")}
          </Button>
        </div>
      </footer>
    </form>
  );
}
