"use client";

import { useEffect, useMemo, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Check, ChevronDown, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { RubricCriteria } from "@/features/rubrics/components/rubric-criteria";
import { cn } from "@/lib/utils";
import { RubricPicker } from "@/features/rubrics/components/rubric-picker";
import type { Rubric } from "@/features/rubrics/schemas/rubric-schema";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { getBank, saveGrading } from "../api/question-bank";
import type { Bank, BankQuestion } from "../schemas/question-bank-schema";
import {
  answerWordCount,
  createGradingSchema,
  type GradingValues,
} from "../schemas/grading-schema";
import { apiError } from "../utils/api-error";

const valuesOf = (question: BankQuestion): GradingValues => ({
  rubric_id: question.rubric_id ?? "",
  expected_answer: question.expected_answer ?? "",
  reviewed: false,
});
export function GradingEditor({
  question,
  expanded = false,
  onDone,
  disabled = false,
  number,
  postingId,
  bank,
  rubrics,
  onSaved,
  onDirtyChange,
  onPendingChange,
}: {
  question: BankQuestion;
  expanded?: boolean;
  onDone?: () => void;
  disabled?: boolean;
  number: number;
  postingId: string;
  bank: Bank;
  rubrics: Rubric[];
  onSaved: (question: BankQuestion) => void;
  onDirtyChange: (id: string, dirty: boolean) => void;
  onPendingChange: (id: string, pending: boolean) => void;
}) {
  const t = useTranslations("recruiter.questionBank.grading");
  const common = useTranslations("common");
  const errorMessage = useApiErrorMessage();
  const [open, setOpen] = useState(false);
  const [choosingRubric, setChoosingRubric] = useState(!question.rubric_id);
  const [baseline, setBaseline] = useState(question);
  const [current, setCurrent] = useState<BankQuestion | null>(null);
  const [conflicted, setConflicted] = useState(false);
  const schema = useMemo(
    () =>
      createGradingSchema(
        bank.max_expected_answer_length,
        bank.max_expected_answer_words,
        t("tooLong"),
      ),
    [bank.max_expected_answer_length, bank.max_expected_answer_words, t],
  );
  const form = useForm<GradingValues>({
    defaultValues: valuesOf(question),
    resolver: zodResolver(schema),
    mode: "onChange",
  });
  const values = useWatch({ control: form.control });
  const text = values.expected_answer ?? "";
  const rubricChanged = (values.rubric_id || null) !== baseline.rubric_id;
  const mustReview =
    !!values.rubric_id &&
    !!text.trim() &&
    (rubricChanged || baseline.expected_answer === null);
  const { isDirty: dirty, isValid } = form.formState;
  const save = useMutation({
    mutationFn: (draft: GradingValues) =>
      saveGrading(postingId, question.id, {
        rubric_id: draft.rubric_id || null,
        expected_answer: draft.expected_answer.trim() || null,
        previous_revision: baseline.revision,
      }),
    onSuccess: (saved) => {
      setBaseline(saved);
      form.reset(valuesOf(saved));
      setConflicted(false);
      setCurrent(null);
      onSaved(saved);
    },
    onError: (error) => {
      if (apiError(error).code === "QUESTION_STALE") setConflicted(true);
    },
  });
  const reload = useMutation({
    mutationFn: async () => {
      const fresh = await getBank(postingId);
      const found = fresh.questions.find((q) => q.id === question.id);
      if (!found) throw new Error("question missing");
      return found;
    },
    onSuccess: setCurrent,
  });
  useEffect(() => {
    onDirtyChange(question.id, dirty);
  }, [dirty, question.id, onDirtyChange]);
  useEffect(() => {
    onPendingChange(question.id, save.isPending);
  }, [save.isPending, question.id, onPendingChange]);
  useEffect(
    () => () => {
      onDirtyChange(question.id, false);
      onPendingChange(question.id, false);
    },
    [question.id, onDirtyChange, onPendingChange],
  );
  const id = `grading-${question.id}`;
  return (
    <div className={cn("space-y-3", !expanded && "mt-3 pl-0 sm:pl-11")}>
      {!expanded && (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span
            className={
              question.grading_incomplete
                ? "text-xs font-medium text-amber-800 dark:text-amber-300"
                : "text-xs text-muted-foreground"
            }
          >
            {question.grading_incomplete ? (
              t("needsConfiguration")
            ) : (
              <span className="inline-flex items-center gap-1">
                <Check className="size-3.5" aria-hidden />
                {t("configured")}
              </span>
            )}
          </span>
          <Button
            variant="ghost"
            className="min-h-11 text-xs"
            aria-expanded={open}
            aria-controls={id}
            onClick={() => setOpen(!open)}
          >
            {t(bank.editable ? "review" : "view", { number })}
            <ChevronDown className="size-3.5" aria-hidden />
          </Button>
        </div>
      )}
      <div
        id={id}
        hidden={!expanded && !open}
        className={expanded ? "" : "rounded-xl border bg-card p-4"}
      >
        {!bank.editable ? (
          <div className="space-y-2">
            <p className="text-xs font-medium text-muted-foreground">
              {t("answer")}
            </p>
            <p className="text-sm whitespace-pre-line wrap-anywhere">
              {question.expected_answer ?? t("noAnswer")}
            </p>
          </div>
        ) : (
          <form
            className="space-y-5"
            aria-label={t("review", { number })}
            onSubmit={form.handleSubmit((draft) => {
              if (!conflicted && (!mustReview || draft.reviewed))
                save.mutate(draft);
            })}
          >
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm font-semibold">{t("rubricStep")}</p>
                {values.rubric_id && !choosingRubric && (
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11 rounded-full text-rolecue-brand"
                    disabled={save.isPending || disabled}
                    onClick={() => setChoosingRubric(true)}
                  >
                    {t("changeRubric")}
                  </Button>
                )}
              </div>
              {choosingRubric ? (
                <RubricPicker
                  id={`${id}-rubric`}
                  rubrics={rubrics}
                  value={values.rubric_id ?? ""}
                  onChange={(value) => {
                    form.setValue("rubric_id", value, {
                      shouldDirty: true,
                      shouldValidate: true,
                    });
                    form.setValue("reviewed", false);
                    setChoosingRubric(false);
                  }}
                  disabled={save.isPending || disabled}
                  allowClear
                />
              ) : (
                <div className="rounded-xl border bg-muted/40 px-4 py-3 text-sm font-medium">
                  {rubrics.find((r) => r.id === values.rubric_id)?.name ??
                    t(values.rubric_id ? "unavailableRubric" : "noRubric")}
                </div>
              )}
              <p className="text-xs leading-relaxed text-muted-foreground">
                {t("rubricHint")}
              </p>
              {values.rubric_id && (
                <RubricCriteria rubricId={values.rubric_id} />
              )}
            </div>
            <div className="space-y-2">
              <label htmlFor={`${id}-answer`} className="text-sm font-medium">
                {t("answer")}
              </label>
              <Textarea
                id={`${id}-answer`}
                rows={5}
                className="min-h-36 rounded-xl bg-card text-[15px] leading-relaxed"
                disabled={save.isPending || disabled}
                {...form.register("expected_answer")}
                aria-invalid={!!form.formState.errors.expected_answer}
                aria-describedby={`${id}-guidance ${id}-counter${form.formState.errors.expected_answer ? ` ${id}-error` : ""}`}
              />
              <p
                id={`${id}-guidance`}
                className="text-xs leading-relaxed text-muted-foreground"
              >
                {t("guidance", {
                  words: bank.expected_answer_guidance_words,
                  maxWords: bank.max_expected_answer_words,
                  maxCharacters: bank.max_expected_answer_length,
                })}
              </p>
              <p
                id={`${id}-counter`}
                className="text-right text-xs text-muted-foreground tabular-nums"
              >
                {t("counter", {
                  words: answerWordCount(text),
                  maxWords: bank.max_expected_answer_words,
                  characters: Array.from(text.trim()).length,
                  maxCharacters: bank.max_expected_answer_length,
                })}
              </p>
              {form.formState.errors.expected_answer && (
                <p
                  id={`${id}-error`}
                  role="alert"
                  className="text-sm text-destructive"
                >
                  {form.formState.errors.expected_answer.message}
                </p>
              )}
            </div>
            {(rubricChanged || baseline.expected_answer === null) && (
              <div className="space-y-2 rounded-xl border border-amber-500/25 bg-amber-50 p-4 dark:bg-amber-950/50">
                <p className="text-sm font-medium">{t("mustReview")}</p>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t("reviewHint")}
                </p>
                {mustReview && (
                  <label className="flex min-h-11 items-center gap-3 text-sm">
                    <Checkbox
                      checked={values.reviewed ?? false}
                      onCheckedChange={(checked) =>
                        form.setValue("reviewed", checked === true, {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                      }
                      disabled={save.isPending || disabled}
                    />
                    {t("confirmed")}
                  </label>
                )}
              </div>
            )}
            {conflicted && (
              <div
                role="alert"
                className="space-y-3 rounded-lg border border-destructive/40 p-3"
              >
                <p className="text-sm font-medium">{t("stale")}</p>
                <p className="text-xs text-muted-foreground">
                  {t("staleHint")}
                </p>
                {!current ? (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => reload.mutate()}
                    disabled={reload.isPending}
                  >
                    {t("reload")}
                  </Button>
                ) : (
                  <>
                    <div className="space-y-2 text-sm">
                      <p className="font-medium">{t("current")}</p>
                      <p className="wrap-anywhere">{current.content}</p>
                      <p className="wrap-anywhere">
                        {rubrics.find((r) => r.id === current.rubric_id)
                          ?.name ?? t("noRubric")}
                      </p>
                      <p className="whitespace-pre-line wrap-anywhere">
                        {current.expected_answer ?? t("noAnswer")}
                      </p>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => {
                        setBaseline(current);
                        form.setValue("reviewed", false);
                        setCurrent(null);
                        setConflicted(false);
                        save.reset();
                      }}
                    >
                      {t("reapply")}
                    </Button>
                  </>
                )}
                {reload.isError && (
                  <p className="text-sm text-destructive">
                    {errorMessage(reload.error, t("reloadFailed"))}
                  </p>
                )}
              </div>
            )}
            {save.isError && !conflicted && (
              <p role="alert" className="text-sm text-destructive">
                {errorMessage(save.error, t("failed"))}
              </p>
            )}
            {save.isSuccess && !dirty && (
              <p
                role="status"
                className="rounded-xl bg-(--rolecue-status-on-bg) px-4 py-3 text-sm font-medium text-(--rolecue-status-on-ink)"
              >
                {t("saved")}
              </p>
            )}
            <div className="flex flex-wrap gap-2 border-t pt-4">
              <Button
                type="submit"
                className="min-h-11 rounded-full"
                disabled={
                  !dirty ||
                  disabled ||
                  !isValid ||
                  save.isPending ||
                  conflicted ||
                  (mustReview && !values.reviewed)
                }
                aria-busy={save.isPending}
              >
                {save.isPending && (
                  <LoaderCircle
                    className="size-4 animate-spin motion-reduce:animate-none"
                    aria-hidden
                  />
                )}
                {save.isPending ? common("actions.saving") : t("save")}
              </Button>
              {expanded ? (
                <Button
                  type="button"
                  variant="outline"
                  className="min-h-11 rounded-full"
                  disabled={dirty || save.isPending || conflicted}
                  onClick={onDone}
                >
                  {t("viewInGroup")}
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="ghost"
                  className="min-h-11"
                  disabled={save.isPending || disabled}
                  onClick={() => {
                    form.reset(valuesOf(baseline));
                    setOpen(false);
                    save.reset();
                  }}
                >
                  {common("actions.cancel")}
                </Button>
              )}
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
