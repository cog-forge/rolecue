"use client";

import { useRef, useState, type ChangeEvent, type ReactNode } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Controller,
  useFieldArray,
  useForm,
  useWatch,
  type Control,
  type FieldError,
  type UseFormRegisterReturn,
} from "react-hook-form";
import { FileUp, LoaderCircle, Plus, Sparkles, Trash2 } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  extractRequirements,
  jobPostingKeys,
  listAvatars,
  listVoices,
} from "../api/job-postings";
import {
  levels,
  postingFormSchema,
  requirementTargets,
  runeLength,
  TEXT_MAX,
  TEXT_MIN,
  type JobPosting,
  type Level,
  type PostingValues,
  type RequirementTarget,
} from "../schemas/job-posting-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { Notice } from "./notice";

const levelText: Record<Level, string> = {
  intern: "Intern",
  junior: "Junior",
  mid: "Mid-level",
  senior: "Senior",
  lead: "Lead",
};
const targetText: Record<RequirementTarget, string> = {
  programming_language: "Programming language",
  framework: "Framework",
  database: "Database",
  tool: "Tool",
  technology: "Technology",
  other: "Other skill",
  technologies: "Technology (general)",
  domain_knowledge: "Domain knowledge",
};
// Uploaded JD files are read in the browser as plain text; the posting stores text only.
const TEXT_FILE_TYPES = ".txt,.md,text/plain,text/markdown";
const TEXT_FILE_MAX_BYTES = 512 * 1024;

export type PostingFormLocks = Readonly<{
  textEditable: boolean;
  requirementsLocked: boolean;
}>;

export function PostingForm({
  mode,
  posting,
  defaultValues,
  locks,
  submitting,
  submitError,
  onSubmit,
  extraActions,
}: {
  mode: "create" | "edit";
  posting?: JobPosting;
  defaultValues: PostingValues;
  locks: PostingFormLocks;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (values: PostingValues, requirementsChanged: boolean) => void;
  extraActions?: (state: { dirty: boolean }) => ReactNode;
}) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const form = useForm<PostingValues>({
    resolver: zodResolver(postingFormSchema),
    defaultValues,
    mode: "onTouched",
  });
  const {
    control,
    register,
    handleSubmit,
    setValue,
    getValues,
    setError,
    clearErrors,
    formState: { errors, isDirty, dirtyFields },
  } = form;
  const rows = useFieldArray({ control, name: "requirements" });
  const fileInput = useRef<HTMLInputElement>(null);
  const [fileError, setFileError] = useState<string | null>(null);

  const avatars = useQuery({
    queryKey: jobPostingKeys.avatars(user.id),
    queryFn: ({ signal }) => listAvatars(signal),
    retry: false,
  });
  const voices = useQuery({
    queryKey: jobPostingKeys.voices(user.id),
    queryFn: ({ signal }) => listVoices(signal),
    retry: false,
  });

  const extract = useMutation({
    mutationFn: (rawText: string) => extractRequirements(rawText),
    retry: false,
    onError: (error) => {
      if (isAccessDenied(error))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    onSuccess: (suggestion) => {
      if (!getValues("title").trim() && suggestion.title)
        setValue("title", suggestion.title, {
          shouldDirty: true,
          shouldValidate: true,
        });
      if (suggestion.seniority_level)
        setValue("seniority_level", suggestion.seniority_level, {
          shouldDirty: true,
          shouldValidate: true,
        });
      setValue("requirements", suggestion.requirements, {
        shouldDirty: true,
        shouldValidate: true,
      });
      setValue("confirmed", false, { shouldDirty: true });
      gooeyToast.success("Requirements suggested", {
        description:
          suggestion.requirements.length > 0
            ? `${suggestion.requirements.length} requirements are ready to review.`
            : "We didn't find any requirements. Add them yourself below.",
      });
    },
  });

  const rawText = useWatch({ control, name: "raw_text" });
  const requirementValues = useWatch({ control, name: "requirements" });
  const textLength = runeLength(rawText.trim());
  const requirementsChanged =
    mode === "create" ||
    Boolean(dirtyFields.requirements) ||
    Boolean(dirtyFields.seniority_level);
  const busy = submitting || extract.isPending;
  const questionCount = posting?.core_question_count ?? 0;

  const loadFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (file.size > TEXT_FILE_MAX_BYTES) {
      setFileError("That file is too large. Use a text file under 512 KB.");
      return;
    }
    try {
      const text = await file.text();
      setFileError(null);
      setValue("raw_text", text, {
        shouldDirty: true,
        shouldValidate: true,
        shouldTouch: true,
      });
    } catch {
      setFileError("We couldn't read that file. Paste the text instead.");
    }
  };

  const submit = handleSubmit((values) => {
    if (submitting) return;
    if (requirementsChanged && !values.confirmed) {
      setError("confirmed", {
        type: "manual",
        message:
          "Confirm that you reviewed the requirements before submitting.",
      });
      return;
    }
    onSubmit(values, requirementsChanged);
  });

  const extractMessage = (() => {
    if (!extract.isError) return null;
    const { message } = apiError(extract.error);
    return `${message ?? "We couldn't read requirements from this text."} Your text is still here. Try again, or add the requirements yourself.`;
  })();

  return (
    <form
      aria-label={mode === "create" ? "Create job posting" : "Edit job posting"}
      noValidate
      onSubmit={(event) => void submit(event)}
      className="space-y-6"
    >
      <SectionCard
        step={1}
        title="Job description"
        contentClassName="space-y-3"
        description={
          <>
            {locks.textEditable
              ? "Paste the job description or load it from a text file. RoleCue suggests the requirements from it."
              : "The text is locked while the posting is pending. It can be corrected after a rejection."}
          </>
        }
      >
        {locks.textEditable ? (
          <>
            <div className="space-y-2">
              <label htmlFor="posting-raw-text" className="text-sm font-medium">
                Job description text (required)
              </label>
              <Textarea
                id="posting-raw-text"
                rows={12}
                disabled={busy}
                className="min-h-64"
                aria-invalid={!!errors.raw_text}
                aria-describedby={describedBy(
                  "posting-raw-text",
                  true,
                  errors.raw_text,
                )}
                {...register("raw_text")}
              />
              <p
                id="posting-raw-text-hint"
                className="text-xs text-muted-foreground tabular-nums"
              >
                {textLength.toLocaleString("en")} /{" "}
                {TEXT_MAX.toLocaleString("en")} characters, at least {TEXT_MIN}.
              </p>
              <FieldErrorText id="posting-raw-text" error={errors.raw_text} />
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <input
                ref={fileInput}
                type="file"
                accept={TEXT_FILE_TYPES}
                className="sr-only"
                tabIndex={-1}
                aria-hidden
                onChange={(event) => void loadFile(event)}
              />
              <Button
                type="button"
                variant="outline"
                className="h-11"
                disabled={busy}
                onClick={() => fileInput.current?.click()}
              >
                <FileUp aria-hidden />
                Load from text file
              </Button>
              <Button
                type="button"
                variant="outline"
                className="h-11"
                disabled={
                  busy || locks.requirementsLocked || textLength < TEXT_MIN
                }
                aria-busy={extract.isPending}
                aria-describedby="posting-extract-hint"
                onClick={() => extract.mutate(getValues("raw_text"))}
              >
                {extract.isPending ? (
                  <LoaderCircle
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden
                  />
                ) : (
                  <Sparkles aria-hidden />
                )}
                {extract.isPending
                  ? "Reading requirements…"
                  : "Suggest requirements"}
              </Button>
            </div>
            <p
              id="posting-extract-hint"
              className="text-xs text-muted-foreground"
            >
              Suggesting again replaces the requirements below. Nothing is saved
              until you submit.
            </p>
            {fileError && (
              <p role="alert" className="text-sm text-destructive">
                {fileError}
              </p>
            )}
            {extractMessage && (
              <p role="alert" className="text-sm text-destructive">
                {extractMessage}
              </p>
            )}
          </>
        ) : (
          <div className="max-h-72 overflow-y-auto rounded-xl border bg-muted/40 p-4 text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
            {rawText}
          </div>
        )}
      </SectionCard>

      <SectionCard
        step={2}
        title="Role and requirements"
        contentClassName="space-y-5"
        description={
          <>
            Candidates are interviewed on these requirements, and the question
            bank is built from them.
          </>
        }
      >
        <TextField
          id="posting-title"
          label="Title (required)"
          error={errors.title}
          disabled={busy}
          registration={register("title")}
        />
        {locks.requirementsLocked ? (
          <LockedRequirements posting={posting} />
        ) : (
          <>
            {mode === "edit" && questionCount > 0 && (
              <Notice tone="warning">
                <p>
                  Changing the job description text, seniority or requirements
                  deletes the {questionCount} questions in this posting&apos;s
                  question bank. You can generate them again afterwards.
                </p>
              </Notice>
            )}
            <div className="space-y-2">
              <label
                htmlFor="posting-seniority"
                className="text-sm font-medium"
              >
                Seniority (required)
              </label>
              <Controller
                control={control}
                name="seniority_level"
                render={({ field }) => (
                  <Select
                    value={field.value || undefined}
                    disabled={busy}
                    onValueChange={(value) => field.onChange(value)}
                  >
                    <SelectTrigger
                      id="posting-seniority"
                      className="w-full max-w-xs bg-background shadow-none data-[size=default]:h-11"
                      aria-invalid={!!errors.seniority_level}
                      aria-describedby={describedBy(
                        "posting-seniority",
                        false,
                        errors.seniority_level,
                      )}
                      onBlur={field.onBlur}
                    >
                      <SelectValue placeholder="Choose seniority" />
                    </SelectTrigger>
                    <SelectContent position="popper" align="start">
                      {levels.map((level) => (
                        <SelectItem key={level} value={level}>
                          {levelText[level]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              <FieldErrorText
                id="posting-seniority"
                error={errors.seniority_level}
              />
            </div>

            <fieldset
              className="space-y-3"
              aria-describedby="posting-requirements-error"
            >
              <legend className="text-sm font-medium">
                Requirements (at least one)
              </legend>
              {rows.fields.length === 0 ? (
                <p className="rounded-xl border border-dashed bg-muted/30 p-4 text-sm text-muted-foreground">
                  No requirements yet. Use Suggest requirements above, or add
                  them yourself.
                </p>
              ) : (
                <ul className="space-y-3">
                  {rows.fields.map((row, index) => {
                    const nameError = errors.requirements?.[index]?.name;
                    const label =
                      requirementValues[index]?.name.trim() ||
                      `requirement ${index + 1}`;
                    return (
                      <li
                        key={row.id}
                        className="grid gap-2 rounded-xl border border-workspace-accent/15 bg-workspace-accent/[0.03] p-3 transition-colors duration-200 animate-in fade-in slide-in-from-left-1 hover:border-workspace-accent/30 motion-reduce:animate-none sm:grid-cols-[minmax(0,1fr)_12rem_9rem_auto] sm:items-start"
                      >
                        <div className="space-y-1">
                          <label
                            htmlFor={`requirement-${index}-name`}
                            className="text-xs font-medium"
                          >
                            Requirement {index + 1}
                          </label>
                          <Input
                            id={`requirement-${index}-name`}
                            className="h-11"
                            disabled={busy}
                            aria-invalid={!!nameError}
                            aria-describedby={
                              nameError
                                ? `requirement-${index}-name-error`
                                : undefined
                            }
                            {...register(`requirements.${index}.name`)}
                          />
                          <FieldErrorText
                            id={`requirement-${index}-name`}
                            error={nameError}
                          />
                        </div>
                        <RowSelect
                          id={`requirement-${index}-target`}
                          label="Kind"
                          control={control}
                          name={`requirements.${index}.target`}
                          disabled={busy}
                          options={requirementTargets.map((target) => ({
                            value: target,
                            label: targetText[target],
                          }))}
                        />
                        <RowSelect
                          id={`requirement-${index}-type`}
                          label="Priority"
                          control={control}
                          name={`requirements.${index}.type`}
                          disabled={busy}
                          options={[
                            { value: "required", label: "Required" },
                            { value: "preferred", label: "Preferred" },
                          ]}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          className="size-11 self-end"
                          disabled={busy}
                          aria-label={`Remove ${label}`}
                          onClick={() => rows.remove(index)}
                        >
                          <Trash2 aria-hidden />
                        </Button>
                      </li>
                    );
                  })}
                </ul>
              )}
              <Button
                type="button"
                variant="outline"
                className="h-11"
                disabled={busy}
                onClick={() =>
                  rows.append({
                    name: "",
                    target: "programming_language",
                    type: "required",
                  })
                }
              >
                <Plus aria-hidden />
                Add requirement
              </Button>
              {errors.requirements?.root?.message ||
              errors.requirements?.message ? (
                <p
                  id="posting-requirements-error"
                  role="alert"
                  className="text-xs text-destructive"
                >
                  {errors.requirements?.root?.message ??
                    errors.requirements?.message}
                </p>
              ) : null}
            </fieldset>

            {requirementsChanged && (
              <div className="space-y-1">
                <div className="flex min-h-11 items-center gap-3">
                  <Controller
                    control={control}
                    name="confirmed"
                    render={({ field }) => (
                      <Checkbox
                        id="posting-confirmed"
                        checked={field.value}
                        disabled={busy}
                        aria-invalid={!!errors.confirmed}
                        aria-describedby={
                          errors.confirmed
                            ? "posting-confirmed-error"
                            : undefined
                        }
                        onCheckedChange={(checked) => {
                          field.onChange(checked === true);
                          if (checked === true) clearErrors("confirmed");
                        }}
                      />
                    )}
                  />
                  <label htmlFor="posting-confirmed" className="text-sm">
                    I reviewed these requirements and they describe the role.
                  </label>
                </div>
                <FieldErrorText
                  id="posting-confirmed"
                  error={errors.confirmed}
                />
              </div>
            )}
          </>
        )}
      </SectionCard>

      <SectionCard
        step={3}
        title="Posting details"
        contentClassName="grid gap-4 sm:grid-cols-2"
        description={<>Optional. Candidates see these on the posting.</>}
      >
        <TextField
          id="posting-location"
          label="Location"
          error={errors.location}
          disabled={busy}
          registration={register("location")}
        />
        <TextField
          id="posting-employment-type"
          label="Employment type"
          hint="For example Full-time or Contract."
          error={errors.employment_type}
          disabled={busy}
          registration={register("employment_type")}
        />
        <TextField
          id="posting-salary-min"
          label="Minimum salary"
          inputMode="numeric"
          error={errors.salary_min}
          disabled={busy}
          registration={register("salary_min")}
        />
        <TextField
          id="posting-salary-max"
          label="Maximum salary"
          inputMode="numeric"
          error={errors.salary_max}
          disabled={busy}
          registration={register("salary_max")}
        />
        <TextField
          id="posting-duration"
          label="Interview length (minutes)"
          inputMode="numeric"
          error={errors.interview_duration}
          disabled={busy}
          registration={register("interview_duration")}
        />
      </SectionCard>

      <SectionCard
        step={4}
        title="Interviewer"
        contentClassName="grid gap-4 sm:grid-cols-2"
        description={
          <>
            Every applicant is interviewed by this model and voice. Both are
            required.
          </>
        }
      >
        <PickerField
          id="posting-avatar"
          label="Interviewer model (required)"
          placeholder="Choose a model"
          control={control}
          name="required_avatar_id"
          error={errors.required_avatar_id}
          disabled={busy}
          loading={avatars.isPending}
          failed={avatars.isError}
          onRetry={() => void avatars.refetch()}
          options={(avatars.data?.items ?? []).map((avatar) => ({
            value: avatar.id,
            label: avatar.name,
          }))}
          current={posting?.required_avatar ?? null}
          emptyText="You don't have an interviewer model yet. Creating one for recruiters isn't available in the web app yet."
        />
        <PickerField
          id="posting-voice"
          label="Interviewer voice (required)"
          placeholder="Choose a voice"
          control={control}
          name="required_voice_id"
          error={errors.required_voice_id}
          disabled={busy}
          loading={voices.isPending}
          failed={voices.isError}
          onRetry={() => void voices.refetch()}
          options={(voices.data?.items ?? []).map((voice) => ({
            value: voice.id,
            label: voice.name,
          }))}
          current={posting?.required_voice ?? null}
          emptyText="No voices are available yet. An administrator adds them to the catalog."
        />
      </SectionCard>

      <SectionCard
        step={5}
        title="Candidates allowed into interview"
        description={
          <>
            How many applicants can take the interview. This is separate from
            how many CVs you receive.
          </>
        }
      >
        <p className="text-sm">
          {posting?.interview_slot != null ? (
            <span className="font-medium tabular-nums">
              {posting.interview_slot}
            </span>
          ) : (
            <span className="text-muted-foreground">
              Not set yet. Setting and funding interview capacity isn&apos;t
              available yet.
            </span>
          )}
        </p>
      </SectionCard>

      {submitError && (
        <Notice role="alert" tone="danger">
          <p className="text-destructive">{submitError}</p>
        </Notice>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Button
          type="submit"
          className="h-11"
          disabled={busy || (mode === "edit" && !isDirty)}
          aria-busy={submitting}
        >
          {submitting && (
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
          )}
          {mode === "create"
            ? submitting
              ? "Submitting…"
              : "Submit for approval"
            : submitting
              ? "Saving…"
              : "Save changes"}
        </Button>
        {extraActions?.({ dirty: isDirty })}
        <Button asChild variant="ghost" className="h-11">
          <Link href={routes.recruiter.jobPostings}>Cancel</Link>
        </Button>
        <p
          role="status"
          aria-live="polite"
          className="text-sm text-muted-foreground"
        >
          {submitting
            ? mode === "create"
              ? "Submitting your posting…"
              : "Saving your changes…"
            : mode === "edit" && isDirty
              ? "You have unsaved changes."
              : ""}
        </p>
      </div>
    </form>
  );
}

function describedBy(id: string, hint: boolean, error?: FieldError) {
  return (
    [hint ? `${id}-hint` : "", error ? `${id}-error` : ""]
      .filter(Boolean)
      .join(" ") || undefined
  );
}

function FieldErrorText({ id, error }: { id: string; error?: FieldError }) {
  if (!error?.message) return null;
  return (
    <p id={`${id}-error`} role="alert" className="text-xs text-destructive">
      {error.message}
    </p>
  );
}

function TextField({
  id,
  label,
  hint,
  error,
  disabled,
  registration,
  inputMode,
}: {
  id: string;
  label: string;
  hint?: string;
  error?: FieldError;
  disabled: boolean;
  registration: UseFormRegisterReturn;
  inputMode?: "numeric";
}) {
  return (
    <div className="space-y-2">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <Input
        id={id}
        inputMode={inputMode}
        disabled={disabled}
        className="h-11"
        aria-invalid={!!error}
        aria-describedby={describedBy(id, !!hint, error)}
        {...registration}
      />
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
      <FieldErrorText id={id} error={error} />
    </div>
  );
}

type FormControl = Control<PostingValues>;

function RowSelect({
  id,
  label,
  control,
  name,
  disabled,
  options,
}: {
  id: string;
  label: string;
  control: FormControl;
  name: `requirements.${number}.target` | `requirements.${number}.type`;
  disabled: boolean;
  options: { value: string; label: string }[];
}) {
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="text-xs font-medium">
        {label}
      </label>
      <Controller
        control={control}
        name={name}
        render={({ field }) => (
          <Select
            value={field.value}
            disabled={disabled}
            onValueChange={field.onChange}
          >
            <SelectTrigger
              id={id}
              className="w-full bg-background shadow-none data-[size=default]:h-11"
              onBlur={field.onBlur}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start">
              {options.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      />
    </div>
  );
}

function PickerField({
  id,
  label,
  placeholder,
  control,
  name,
  error,
  disabled,
  loading,
  failed,
  onRetry,
  options,
  current,
  emptyText,
}: {
  id: string;
  label: string;
  placeholder: string;
  control: FormControl;
  name: "required_avatar_id" | "required_voice_id";
  error?: FieldError;
  disabled: boolean;
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  options: { value: string; label: string }[];
  current: { id: string; name: string | null } | null;
  emptyText: string;
}) {
  // A saved choice that is no longer listed (e.g. a voice deactivated since) still shows, and the
  // backend accepts it unchanged.
  const all =
    current && !options.some((option) => option.value === current.id)
      ? [
          ...options,
          {
            value: current.id,
            label: `${current.name ?? "Unnamed"} (no longer available)`,
          },
        ]
      : options;
  return (
    <div className="space-y-2">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {loading ? (
        <div role="status">
          <Skeleton className="h-11 rounded-md" />
          <span className="sr-only">Loading choices…</span>
        </div>
      ) : failed ? (
        <div role="alert" className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-destructive">
            We couldn&apos;t load the choices.
          </span>
          <Button
            type="button"
            variant="outline"
            className="h-11"
            onClick={onRetry}
          >
            Try again
          </Button>
        </div>
      ) : all.length === 0 ? (
        <p
          id={`${id}-hint`}
          className="rounded-xl border border-dashed bg-muted/30 p-3 text-sm text-muted-foreground"
        >
          {emptyText}
        </p>
      ) : (
        <Controller
          control={control}
          name={name}
          render={({ field }) => (
            <Select
              value={field.value || undefined}
              disabled={disabled}
              onValueChange={field.onChange}
            >
              <SelectTrigger
                id={id}
                className="w-full bg-background shadow-none data-[size=default]:h-11"
                aria-invalid={!!error}
                aria-describedby={error ? `${id}-error` : undefined}
                onBlur={field.onBlur}
              >
                <SelectValue placeholder={placeholder} />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                {all.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
      )}
      <FieldErrorText id={id} error={error} />
    </div>
  );
}

// Shows the saved seniority and requirements: they are what the question bank was built from.
function LockedRequirements({ posting }: { posting?: JobPosting }) {
  if (!posting) return null;
  return (
    <div className="space-y-3">
      <Notice tone="locked">
        <p>
          Seniority and requirements are locked because this posting&apos;s
          question bank was generated from them. Delete its questions to change
          them.{" "}
          <Link
            href={routes.recruiter.jobPostingQuestions(posting.id)}
            className="font-medium underline underline-offset-4"
          >
            Open the question bank
          </Link>
        </p>
      </Notice>
      <dl className="text-sm">
        <dt className="text-muted-foreground">Seniority</dt>
        <dd className="font-medium">{levelText[posting.seniority_level]}</dd>
      </dl>
      <ul className="divide-y rounded-xl border bg-muted/20 text-sm">
        {posting.requirements.map((row) => (
          <li
            key={`${row.name}-${row.target}`}
            className="flex flex-wrap justify-between gap-2 p-3"
          >
            <span className="font-medium [overflow-wrap:anywhere]">
              {row.name}
            </span>
            <span className="text-muted-foreground">
              {targetText[row.target]} ·{" "}
              {row.type === "required" ? "Required" : "Preferred"}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export { levelText, targetText };

// One numbered step of the form. Steps fade in one after another on first render.
function SectionCard({
  step,
  title,
  description,
  contentClassName,
  children,
}: {
  step: number;
  title: string;
  description: ReactNode;
  contentClassName?: string;
  children: ReactNode;
}) {
  return (
    <Card
      className="duration-500 animate-in fade-in slide-in-from-bottom-2 fill-mode-both motion-reduce:animate-none"
      style={{ animationDelay: `${(step - 1) * 60}ms` }}
    >
      <CardHeader>
        <CardTitle className="flex items-center gap-3">
          <span
            aria-hidden
            className="flex size-7 shrink-0 items-center justify-center rounded-full bg-workspace-accent/10 text-xs font-semibold text-workspace-accent ring-1 ring-workspace-accent/25"
          >
            {step}
          </span>
          <h2>{title}</h2>
        </CardTitle>
        <CardDescription className="sm:pl-10">{description}</CardDescription>
      </CardHeader>
      <CardContent className={cn(contentClassName)}>{children}</CardContent>
    </Card>
  );
}
