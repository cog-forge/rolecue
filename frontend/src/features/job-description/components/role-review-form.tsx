"use client";

import { useRef, useState } from "react";
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
  reviewSchema,
  type RoleProfile,
  type RoleReview,
} from "../api/contract";
import { roleError } from "../api/transport";
import { RequirementRow } from "./requirement-row";

export function RoleReviewForm({
  profile,
  busy,
  onSave,
  onConfirm,
  audience = "candidate",
}: {
  profile: RoleProfile;
  audience?: "candidate" | "recruiter";
  busy: boolean;
  onSave: (review: RoleReview) => Promise<RoleProfile>;
  onConfirm: (review: RoleReview) => Promise<RoleProfile>;
}) {
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
        { message: "Select a seniority level before confirming." },
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
        <p>
          Edit the role and requirements directly below. What you confirm is the{" "}
          {audience === "recruiter"
            ? "role context saved for this job posting."
            : "role context saved for your practice."}
        </p>
      </div>
      <fieldset disabled={disabled} className="min-w-0 space-y-7">
        <legend className="sr-only">Review Role Profile</legend>
        <div className="grid gap-5 sm:grid-cols-[minmax(0,1fr)_14rem]">
          <div className="space-y-2">
            <label
              htmlFor="role-title"
              className="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
            >
              Role title
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
              Seniority
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
                    <SelectItem value="unspecified">Choose a level</SelectItem>
                    <SelectItem value="intern">Intern</SelectItem>
                    <SelectItem value="junior">Junior</SelectItem>
                    <SelectItem value="mid">Mid</SelectItem>
                    <SelectItem value="senior">Senior</SelectItem>
                    <SelectItem value="lead">Lead</SelectItem>
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
                Technical requirements{" "}
                <span className="ml-2 text-sm font-normal text-muted-foreground">
                  {rows.fields.length}
                </span>
              </h2>
              <p className="text-sm text-muted-foreground">
                Set the proficiency this role calls for. Open details to adjust
                classification or priority. Kind groups an ability, a named
                technology, or domain knowledge; category describes the type of
                technical skill, such as a language, framework, database, or
                tool.
              </p>
            </div>
            <Button
              type="button"
              variant="outline"
              className="h-11"
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
              <Plus className="size-4" aria-hidden="true" /> Add requirement
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
            {audience === "recruiter"
              ? "Guide the interview focus."
              : "Make the practice your own."}
          </label>
          <p
            id="refinement-help"
            className="text-sm leading-relaxed text-muted-foreground"
          >
            Add a focus or exclusion, such as “Focus on caching and concurrency”
            or “Exclude C#”. Your note guides interview preparation. Saving or
            confirming saves this note and your current edits. Confirm saves the
            review first, then prepares interview content.
          </p>
          <Textarea
            id="refinement-note"
            {...form.register("refinementNote")}
            rows={4}
            placeholder="What would you like to focus on?"
            aria-invalid={Boolean(form.formState.errors.refinementNote)}
            aria-describedby="refinement-help refinement-error"
          />
          <div className="flex flex-wrap justify-between gap-3 text-xs text-muted-foreground">
            <p id="refinement-error" className="text-destructive">
              {form.formState.errors.refinementNote?.message}
            </p>
            <span>{[...note].length} / 2,000</span>
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
            {audience === "recruiter"
              ? "Confirmation prepares interview content. You’ll review the saved core questions next."
              : "Confirmation prepares and saves interview content for this role."}
          </p>
          <p
            role="status"
            aria-live="polite"
            className="text-xs text-muted-foreground"
          >
            {disabled
              ? action === "confirm"
                ? "Preparing interview content…"
                : "Saving your review…"
              : form.formState.isDirty
                ? "You have unsaved changes."
                : "Your latest review is saved."}
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button
            type="submit"
            variant="outline"
            disabled={disabled}
            className="h-11"
          >
            {action === "save" ? (
              <LoaderCircle
                className="size-4 motion-safe:animate-spin"
                aria-hidden="true"
              />
            ) : (
              <Save className="size-4" aria-hidden="true" />
            )}
            {action === "save" ? "Saving…" : "Save review"}
          </Button>
          <Button
            type="button"
            disabled={disabled}
            className="h-11"
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
              ? "Confirming…"
              : audience === "recruiter"
                ? "Confirm requirements"
                : "Confirm role"}
          </Button>
        </div>
      </footer>
    </form>
  );
}
