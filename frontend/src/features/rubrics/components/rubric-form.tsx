"use client";

import { useRef, useState } from "react";
import { Controller, useFieldArray, useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslations } from "next-intl";
import {
  Trash2,
  LockKeyhole,
  CircleAlert,
  ArrowLeft,
  Save,
  LoaderCircle,
} from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import {
  Field,
  FieldSet,
  FieldLabel,
  FieldDescription,
  FieldError,
} from "@/components/ui/field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useUnsavedChangesGuard } from "@/lib/hooks/use-unsaved-changes-guard";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  createRubricFormSchema,
  emptyCriterion,
  type Rubric,
  type RubricValues,
} from "../schemas/rubric-schema";

export function RubricForm({
  rubric,
  metrics,
  pending,
  error,
  conflict,
  onSave,
  onClose,
  onReload,
  onReapply,
}: {
  rubric: Rubric | null;
  metrics: { id: string; name: string }[];
  pending: boolean;
  error: string | null;
  conflict: boolean;
  onSave: (values: RubricValues, updatedAt?: string) => void;
  onClose: () => void;
  onReload: () => void;
  onReapply: () => void;
}) {
  const t = useTranslations("admin.rubrics");
  const form = useForm<RubricValues>({
    resolver: zodResolver(
      createRubricFormSchema(
        t("required"),
        t("invalid"),
        metrics.map((m) => m.id),
      ),
    ),
    defaultValues: rubric
      ? {
          name: rubric.name,
          description: rubric.description,
          question_type: rubric.question_type,
          criteria:
            rubric.criteria?.map((c) => ({
              id: c.id,
              name: c.name,
              metric_id: c.metric_id,
              max_points: c.max_points,
              scoring_guide: c.scoring_guide,
            })) ?? [],
        }
      : {
          name: "",
          description: "",
          question_type: "concept_explanation",
          criteria: [emptyCriterion()],
        },
  });
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "criteria",
    keyName: "fieldKey",
  });
  const criteria = useWatch({ control: form.control, name: "criteria" });
  // The draft must keep the revision it was loaded from until an explicit reload.
  const [loadedUpdatedAt] = useState(rubric?.updated_at);
  const dirty = form.formState.isDirty;
  const summary = useRef<HTMLDivElement>(null);
  const readOnly = Boolean(rubric && (!rubric.can_manage || rubric.locked));
  const guard = useUnsavedChangesGuard(dirty);
  const [discardAction, setDiscardAction] = useState<"close" | "reload" | null>(
    null,
  );
  function close() {
    if (dirty) setDiscardAction("close");
    else onClose();
  }
  const errors = form.formState.errors;
  const fieldClass = "h-11 bg-background";
  return (
    <form
      onSubmit={(event) => {
        void form.handleSubmit(
          (values) => onSave(values, loadedUpdatedAt),
          () => requestAnimationFrame(() => summary.current?.focus()),
        )(event);
      }}
      className="space-y-6 [&_input]:scroll-my-28 [&_textarea]:scroll-my-28 [&_button]:scroll-my-28"
      noValidate
    >
      <div className="flex items-center justify-between gap-3 py-2">
        <Button
          type="button"
          variant="ghost"
          className="h-11 px-2"
          onClick={close}
          disabled={pending}
        >
          <ArrowLeft
            aria-hidden
            className="size-4 transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover/button:-translate-x-1 group-focus-visible/button:-translate-x-1 motion-reduce:translate-x-0! motion-reduce:transition-none"
          />
          {t("back")}
        </Button>
        <div className="flex items-center gap-4">
          <p
            className="sr-only text-sm text-muted-foreground sm:not-sr-only"
            role="status"
          >
            {readOnly ? t("readOnly") : dirty ? t("unsaved") : ""}
          </p>
        </div>
      </div>
      {rubric?.locked && (
        <Alert className="border-amber-500/30 bg-amber-500/10 p-4 text-amber-800 dark:text-amber-200">
          <LockKeyhole aria-hidden />
          <AlertDescription className="text-inherit">
            {t("lockedReason", { count: rubric.reference_count ?? 0 })}
          </AlertDescription>
        </Alert>
      )}
      {error && (
        <Alert
          variant="destructive"
          className="border-destructive/30 bg-destructive/5 p-4"
        >
          <CircleAlert aria-hidden />
          <AlertDescription className="space-y-3">
            <p>{error}</p>
            {conflict && (
              <>
                <p>{t("conflictHelp")}</p>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    disabled={pending}
                    onClick={() => {
                      if (dirty) setDiscardAction("reload");
                      else onReload();
                    }}
                  >
                    {t("reload")}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={onReapply}
                    disabled={pending}
                  >
                    {t("reapply")}
                  </Button>
                </div>
              </>
            )}
          </AlertDescription>
        </Alert>
      )}
      {Object.keys(errors).length > 0 && (
        <Alert
          variant="destructive"
          ref={summary}
          tabIndex={-1}
          role="alert"
          className="scroll-mt-24 rounded-xl border border-destructive p-4 text-sm focus-visible:ring-2"
        >
          <AlertDescription>
            <p>{t("validationSummary")}</p>
            <a
              href={
                errors.name
                  ? "#rubric-name"
                  : errors.question_type
                    ? "#rubric-type"
                    : "#criteria-heading"
              }
              className="underline"
            >
              {t("reviewFields")}
            </a>
          </AlertDescription>
        </Alert>
      )}
      <FieldSet
        disabled={readOnly || pending}
        className="grid min-w-0 gap-6 lg:grid-cols-[minmax(0,1fr)_280px]"
      >
        <div className="space-y-6">
          <Card className="gap-5 rounded-2xl border-t-2 border-t-rolecue-brand/50 py-6 [--card-spacing:--spacing(6)]">
            <CardHeader>
              <CardTitle>
                <h2 className="text-lg font-semibold">{t("details")}</h2>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              <Field>
                <FieldLabel
                  className="text-sm font-medium"
                  htmlFor="rubric-name"
                >
                  {t("name")}
                </FieldLabel>
                <Input
                  id="rubric-name"
                  className={fieldClass}
                  maxLength={120}
                  {...form.register("name")}
                  aria-invalid={!!errors.name}
                  aria-describedby="name-error"
                />
                <FieldError id="name-error">{errors.name?.message}</FieldError>
              </Field>
              <Field>
                <FieldLabel
                  className="text-sm font-medium"
                  htmlFor="rubric-description"
                >
                  {t("description")}
                </FieldLabel>
                <Textarea
                  id="rubric-description"
                  maxLength={1000}
                  {...form.register("description")}
                  aria-invalid={!!errors.description}
                  aria-describedby="description-error"
                />
                <FieldError id="description-error">
                  {errors.description?.message}
                </FieldError>
              </Field>
              <Field>
                <FieldLabel
                  className="text-sm font-medium"
                  htmlFor="rubric-type"
                >
                  {t("questionType")}
                </FieldLabel>
                <Input
                  id="rubric-type"
                  className={fieldClass}
                  maxLength={64}
                  {...form.register("question_type")}
                  aria-invalid={!!errors.question_type}
                  aria-describedby="type-help type-error"
                />
                <FieldDescription id="type-help">
                  {t("typeHelp")}
                </FieldDescription>
                <FieldError id="type-error">
                  {errors.question_type?.message}
                </FieldError>
              </Field>
            </CardContent>
          </Card>
          <section className="space-y-4" aria-labelledby="criteria-heading">
            <div className="flex items-center justify-between gap-3">
              <h2
                id="criteria-heading"
                tabIndex={-1}
                className="text-lg font-semibold"
              >
                {t("criteria")}
              </h2>
              <span className="text-sm text-muted-foreground">
                {fields.length}/10
              </span>
            </div>
            {fields.map((field, i) => {
              const error = errors.criteria?.[i];
              return (
                <Card
                  key={field.fieldKey}
                  className="gap-0 rounded-2xl py-0 [--card-spacing:--spacing(6)]"
                >
                  <CardHeader className="flex flex-row items-center justify-between border-b bg-rolecue-brand/5 py-4">
                    <h3 className="flex items-center gap-3 font-medium">
                      <span className="flex size-8 items-center justify-center rounded-lg bg-rolecue-brand text-sm text-white dark:text-background">
                        {i + 1}
                      </span>
                      {t("criterion")}
                    </h3>
                    {!readOnly && (
                      <Button
                        type="button"
                        variant="ghost"
                        className="size-11 text-muted-foreground hover:bg-destructive/10 hover:text-destructive focus-visible:ring-destructive/30 dark:hover:bg-destructive/20 dark:hover:text-destructive"
                        disabled={fields.length <= 1}
                        aria-label={t("removeCriterion", { number: i + 1 })}
                        onClick={() => remove(i)}
                      >
                        <Trash2 aria-hidden className="size-4" />
                      </Button>
                    )}
                  </CardHeader>
                  <CardContent className="space-y-4 py-5">
                    <Field>
                      <FieldLabel
                        className="text-sm font-medium"
                        htmlFor={`criterion-${i}`}
                      >
                        {t("criterionName")}
                      </FieldLabel>
                      <Input
                        id={`criterion-${i}`}
                        className={fieldClass}
                        maxLength={160}
                        {...form.register(`criteria.${i}.name`)}
                        aria-invalid={!!error?.name}
                        aria-describedby={`criterion-error-${i}`}
                      />
                    </Field>
                    <div className="grid gap-4 sm:grid-cols-[1fr_140px]">
                      <Field>
                        <FieldLabel
                          className="text-sm font-medium"
                          htmlFor={`metric-${i}`}
                        >
                          {t("metric")}
                        </FieldLabel>
                        <Controller
                          control={form.control}
                          name={`criteria.${i}.metric_id`}
                          render={({ field: metricField }) => (
                            <Select
                              value={metricField.value}
                              onValueChange={metricField.onChange}
                              disabled={readOnly || pending}
                            >
                              <SelectTrigger
                                id={`metric-${i}`}
                                ref={metricField.ref}
                                onBlur={metricField.onBlur}
                                className="h-11 w-full bg-background data-[size=default]:h-11"
                                aria-invalid={!!error?.metric_id}
                                aria-describedby={`criterion-error-${i}`}
                              >
                                <SelectValue placeholder={t("chooseMetric")} />
                              </SelectTrigger>
                              <SelectContent
                                position="popper"
                                className="motion-reduce:animate-none! motion-reduce:transition-none!"
                              >
                                {metrics.map((m) => (
                                  <SelectItem key={m.id} value={m.id}>
                                    {m.name}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          )}
                        />
                      </Field>
                      <Field>
                        <FieldLabel
                          className="text-sm font-medium"
                          htmlFor={`points-${i}`}
                        >
                          {t("maxPoints")}
                        </FieldLabel>
                        <Input
                          id={`points-${i}`}
                          className={fieldClass}
                          type="number"
                          min={1}
                          max={10}
                          {...form.register(`criteria.${i}.max_points`, {
                            valueAsNumber: true,
                          })}
                          aria-invalid={!!error?.max_points}
                          aria-describedby={`criterion-error-${i}`}
                        />
                      </Field>
                    </div>
                    <Field>
                      <FieldLabel
                        className="text-sm font-medium"
                        htmlFor={`guide-${i}`}
                      >
                        {t("scoringGuide")}
                      </FieldLabel>
                      <Textarea
                        id={`guide-${i}`}
                        className="min-h-28"
                        maxLength={2000}
                        {...form.register(`criteria.${i}.scoring_guide`)}
                        placeholder={t("guidePlaceholder")}
                        aria-invalid={!!error?.scoring_guide}
                        aria-describedby={`criterion-error-${i}`}
                      />
                    </Field>
                    <FieldError id={`criterion-error-${i}`}>
                      {[
                        error?.name?.message,
                        error?.metric_id?.message,
                        error?.max_points?.message,
                        error?.scoring_guide?.message,
                      ]
                        .filter(Boolean)
                        .join(" · ")}
                    </FieldError>
                  </CardContent>
                </Card>
              );
            })}
            {!readOnly && (
              <Button
                type="button"
                variant="outline"
                className="h-11 w-full border-dashed border-rolecue-brand/40 bg-rolecue-brand/5 text-rolecue-brand hover:bg-rolecue-brand/10 hover:text-rolecue-brand"
                disabled={fields.length >= 10}
                onClick={() => {
                  append(emptyCriterion());
                  requestAnimationFrame(() =>
                    document
                      .getElementById(`criterion-${fields.length}`)
                      ?.focus(),
                  );
                }}
              >
                {t("addCriterion")}
              </Button>
            )}
          </section>
        </div>
        <aside className="h-fit lg:sticky lg:top-20" aria-label={t("summary")}>
          <Card className="gap-3 rounded-2xl border border-rolecue-brand/15 py-5 [--card-spacing:--spacing(5)]">
            <CardHeader>
              <CardTitle>
                <h2 className="font-semibold">{t("summary")}</h2>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="divide-y">
                <div className="flex items-center justify-between gap-4 pb-3">
                  <dt className="text-sm text-muted-foreground">
                    {t("criteria")}
                  </dt>
                  <dd className="font-semibold tabular-nums">
                    {fields.length}
                    <span className="font-normal text-muted-foreground">
                      {" "}
                      / 10
                    </span>
                  </dd>
                </div>
                <div className="flex items-center justify-between gap-4 pt-3">
                  <dt className="text-sm text-muted-foreground">
                    {t("totalPoints")}
                  </dt>
                  <dd className="text-2xl font-semibold text-rolecue-brand tabular-nums">
                    {criteria.reduce(
                      (n, c) =>
                        n + (Number.isFinite(c.max_points) ? c.max_points : 0),
                      0,
                    )}
                  </dd>
                </div>
              </dl>
              {!readOnly && (
                <Button
                  type="submit"
                  className="mt-5 h-11 w-full bg-rolecue-brand text-white hover:bg-rolecue-brand/90 dark:text-background"
                  disabled={pending || conflict}
                  aria-busy={pending}
                >
                  <span
                    aria-hidden
                    className="t-icon-swap size-4"
                    data-state={pending ? "b" : "a"}
                  >
                    <span className="t-icon" data-icon="a">
                      <Save className="size-4 transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover/button:-translate-y-0.5 group-hover/button:scale-110 group-focus-visible/button:-translate-y-0.5 group-focus-visible/button:scale-110 motion-reduce:translate-y-0! motion-reduce:scale-100! motion-reduce:transition-none" />
                    </span>
                    <span className="t-icon" data-icon="b">
                      <LoaderCircle
                        className={
                          pending ? "size-4 motion-safe:animate-spin" : "size-4"
                        }
                      />
                    </span>
                  </span>
                  {pending ? t("saving") : t("save")}
                </Button>
              )}
            </CardContent>
          </Card>
        </aside>
      </FieldSet>
      <Dialog
        open={guard.leaving || discardAction !== null}
        onOpenChange={(open) => {
          if (!open) {
            guard.stay();
            setDiscardAction(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("unsaved")}</DialogTitle>
            <DialogDescription>{t("discard")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                guard.stay();
                setDiscardAction(null);
              }}
            >
              {t("cancel")}
            </Button>
            <Button
              type="button"
              onClick={() => {
                if (discardAction) {
                  const action = discardAction;
                  setDiscardAction(null);
                  if (action === "close") onClose();
                  else onReload();
                } else guard.leave();
              }}
            >
              {t("confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </form>
  );
}
