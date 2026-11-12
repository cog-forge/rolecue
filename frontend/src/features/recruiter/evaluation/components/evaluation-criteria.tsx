"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { LoaderCircle } from "lucide-react";
import { useForm, useWatch } from "react-hook-form";
import { gooeyToast } from "goey-toast";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { MetricCoverage } from "@/components/feedback/metric-coverage";
import { routes } from "@/config/routes";
import { questionBankKeys } from "@/features/recruiter/question-bank/api/question-bank";
import { jobPostingKeys } from "@/features/recruiter/job-postings/api/job-postings";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import { PostingTabs } from "@/features/recruiter/job-postings/components/posting-tabs";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import { apiErrorMessage } from "@/lib/i18n/api-errors";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import {
  evaluationKeys,
  getMetrics,
  getWeights,
  resetWeights,
  saveWeights,
} from "../api/evaluation";
import { useUnsavedChangesGuard } from "@/lib/hooks/use-unsaved-changes-guard";
import type { Metric, PostingWeights } from "../schemas/evaluation-schema";
import { ConfirmDialog } from "./confirm-dialog";
import {
  evaluateDrafts,
  formatPercent,
  formatSum,
  weightDrafts,
  type WeightDrafts,
} from "../utils/weight-drafts";

// Codes after which the cached weights are stale: the posting was approved, or the metrics catalog
// changed. Both lead to reloading the saved weights and the catalog.
const staleCodes = new Set([
  "EVALUATION_WEIGHTS_LOCKED",
  "EVALUATION_METRIC_UNKNOWN",
]);

export function EvaluationCriteria({ postingId }: { postingId: string }) {
  const t = useTranslations("recruiter.evaluation");
  const tCommon = useTranslations("common");
  const navT = useTranslations("common.shell.nav.items");
  const user = useSessionUser();
  const cache = useQueryClient();
  // Remounts the form on the server's values after a save or a refresh.
  const [version, setVersion] = useState(0);
  // Both live here, outside the keyed form, so a remount neither hides the notice nor forgets
  // that leaving the page would lose unsaved weights.
  const [notice, setNotice] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const weightsKey = evaluationKeys.weights(user.id, postingId);
  const metricsKey = evaluationKeys.metrics(user.id);
  const guard = useUnsavedChangesGuard(dirty);

  const weights = useQuery({
    queryKey: weightsKey,
    queryFn: ({ signal }) => getWeights(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const metrics = useQuery({
    queryKey: metricsKey,
    queryFn: ({ signal }) => getMetrics(signal),
    retry: false,
  });
  const error = weights.error ?? metrics.error;
  const denied = isAccessDenied(error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const back = {
    href: routes.recruiter.jobPostings,
    label: navT("myJobPostings"),
  };

  if (weights.isPending || metrics.isPending)
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <PageHeader back={back} title={t("title")} />
        <div role="status" className="space-y-3">
          <Skeleton className="h-64 rounded-xl" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      </section>
    );
  if (weights.isError || metrics.isError || !weights.data || !metrics.data) {
    const notFound = [400, 404].includes(apiError(weights.error).status);
    const failure = denied ? "denied" : notFound ? "notFound" : "failed";
    return (
      <section className="mx-auto w-full max-w-3xl space-y-6">
        <PageHeader back={back} title={t("title")} />
        <div
          role="alert"
          className={cn(
            "rounded-xl border p-6",
            !denied && !notFound && "border-destructive/30",
          )}
        >
          <h2 className="text-base font-semibold">
            {t(`loadError.${failure}.title`)}
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {failure === "failed"
              ? tCommon("messages.tryAgainLater")
              : t(`loadError.${failure}.description`)}
          </p>
          {!denied && !notFound && (
            <Button
              className={cn("mt-4", pillClass)}
              variant="outline"
              disabled={weights.isFetching || metrics.isFetching}
              onClick={() => {
                void weights.refetch();
                void metrics.refetch();
              }}
            >
              {tCommon("actions.tryAgain")}
            </Button>
          )}
        </div>
      </section>
    );
  }

  const saved = weights.data;
  // A change that left the bank unable to cover every active metric took the posting out of the Admin queue
  // (KAN-377): say so, and reread the screens that show whether it is submitted.
  const afterChange = (next: typeof saved) => {
    setNotice(next.submission_withdrawn ? t("withdrawnNotice") : null);
    if (!next.submission_withdrawn) return;
    void cache.invalidateQueries({ queryKey: questionBankKeys.all(user.id) });
    void cache.invalidateQueries({ queryKey: jobPostingKeys.all(user.id) });
  };
  // Reloads the saved weights and the metrics catalog, then rebuilds the form on them.
  const refresh = () =>
    void Promise.all([
      cache.refetchQueries({ queryKey: weightsKey }),
      cache.refetchQueries({ queryKey: metricsKey }),
    ]).then(() => setVersion((value) => value + 1));
  return (
    <section className="mx-auto w-full max-w-5xl space-y-6">
      <PageHeader
        back={back}
        title={t("title")}
        description={t("description")}
      />
      <PostingTabs
        postingId={postingId}
        active="evaluation"
        status={saved.posting_status}
      />
      <div className="max-w-3xl space-y-6">
        {notice && (
          <Notice role="status" tone="info">
            <p>{notice}</p>
          </Notice>
        )}
        {saved.editable ? (
          <WeightsForm
            key={version}
            postingId={postingId}
            metrics={metrics.data}
            saved={saved}
            onDirtyChange={setDirty}
            onSaved={(next) => {
              cache.setQueryData(weightsKey, next);
              afterChange(next);
              setVersion((value) => value + 1);
            }}
            onReset={(next) => {
              cache.setQueryData(weightsKey, next);
              afterChange(next);
              refresh();
            }}
            onStale={() => {
              setNotice(t("staleNotice"));
              refresh();
            }}
          />
        ) : (
          <ReadOnlyWeights saved={saved} />
        )}
        <ConfirmDialog
          open={guard.leaving}
          title={t("leaveDialog.title")}
          description={t("leaveDialog.description")}
          cancelLabel={t("leaveDialog.stay")}
          confirmLabel={t("leaveDialog.leave")}
          onCancel={guard.stay}
          onConfirm={guard.leave}
        />
      </div>
    </section>
  );
}

function WeightsForm({
  postingId,
  metrics,
  saved,
  onSaved,
  onReset,
  onStale,
  onDirtyChange,
}: {
  postingId: string;
  metrics: readonly Metric[];
  saved: PostingWeights;
  onSaved: (saved: PostingWeights) => void;
  onReset: (defaults: PostingWeights) => void;
  onStale: () => void;
  onDirtyChange: (dirty: boolean) => void;
}) {
  const t = useTranslations("recruiter.evaluation");
  const tCommon = useTranslations("common");
  const draftsT = useTranslations("recruiter.evaluation.drafts");
  const errorsT = useTranslations("errors");
  const fmt = useFormatters();
  const cache = useQueryClient();
  const form = useForm<{ drafts: WeightDrafts }>({
    defaultValues: { drafts: weightDrafts(metrics, saved) },
  });
  const drafts = useWatch({ control: form.control, name: "drafts" });
  const evaluation = evaluateDrafts(metrics, drafts, saved.total, draftsT, fmt);
  const dirty = form.formState.isDirty;

  const save = useMutation({
    mutationFn: (body: NonNullable<typeof evaluation.body>) =>
      saveWeights(postingId, body),
    retry: false,
    onSuccess: (next) => {
      onSaved(next);
      gooeyToast.success(t("toast.saved"));
    },
    onError: (failure) => {
      if (isAccessDenied(failure))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      const { code } = apiError(failure);
      if (code && staleCodes.has(code)) onStale();
    },
  });

  // Back to the equal-split defaults (Decision 13a): the saved set is deleted, not overwritten.
  const [confirmReset, setConfirmReset] = useState(false);
  const reset = useMutation({
    mutationFn: () => resetWeights(postingId),
    retry: false,
    onSuccess: (next) => {
      setConfirmReset(false);
      onReset(next);
      gooeyToast.success(t("toast.restored"));
    },
    onError: (failure) => {
      if (isAccessDenied(failure))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      const { code } = apiError(failure);
      if (code && staleCodes.has(code)) {
        setConfirmReset(false);
        onStale();
      }
    },
  });
  const busy = save.isPending || reset.isPending;

  // Tell the page whether leaving it would lose weights; a remount of the form resets it. The page
  // owns the guard, so it survives that remount.
  useEffect(() => {
    onDirtyChange(dirty);
    return () => onDirtyChange(false);
  }, [dirty, onDirtyChange]);

  // The server's own text is never shown: the message is chosen by the error code.
  const serverError =
    save.isError && !staleCodes.has(apiError(save.error).code ?? "")
      ? isAccessDenied(save.error)
        ? t("editDenied")
        : t("saveError", {
            reason: apiErrorMessage(errorsT, save.error, t("saveFailed")),
          })
      : null;
  const resetError =
    reset.isError && !staleCodes.has(apiError(reset.error).code ?? "")
      ? isAccessDenied(reset.error)
        ? t("editDenied")
        : apiErrorMessage(errorsT, reset.error, t("resetFailed"))
      : null;
  const sumLabel =
    saved.total > 0
      ? t("sumOf", {
          sum: formatSum(evaluation.sum, saved.total, fmt),
          total: formatPercent(saved.total, fmt),
        })
      : formatPercent(evaluation.sum, fmt);
  // How the equal-split defaults are described: split a total, or every metric counting the same.
  const splitMode = saved.total > 0 ? "split" : "same";
  const splitTotal = formatPercent(saved.total, fmt);

  return (
    <form
      noValidate
      aria-label={t("title")}
      className="space-y-6"
      onSubmit={form.handleSubmit(() => {
        if (save.isPending || !evaluation.body || !dirty) return;
        save.mutate(evaluation.body);
      })}
    >
      <MetricCoverage coverage={saved.coverage} audience="recruiter" />
      {saved.using_defaults && (
        <Notice tone="info" title={t("defaults.title")}>
          <p>{t("defaults.body", { mode: splitMode, total: splitTotal })}</p>
        </Notice>
      )}
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("weights.title")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <ul className="divide-y rounded-xl border bg-muted/20">
            {metrics.map((metric) => {
              const id = `weight-${metric.id}`;
              const error = evaluation.fieldErrors[metric.id];
              return (
                <li
                  key={metric.id}
                  className="flex flex-wrap items-center justify-between gap-3 p-3"
                >
                  <label
                    htmlFor={id}
                    className="min-w-0 text-sm font-medium [overflow-wrap:anywhere]"
                  >
                    {metric.name}
                  </label>
                  <div className="flex items-center gap-2">
                    <Input
                      id={id}
                      type="number"
                      inputMode="decimal"
                      step="any"
                      min={0}
                      placeholder="0"
                      className="h-11 w-28 text-right tabular-nums"
                      disabled={busy}
                      aria-invalid={error !== undefined}
                      aria-describedby={error ? `${id}-error` : undefined}
                      {...form.register(`drafts.${metric.id}`)}
                    />
                    <span aria-hidden className="text-sm text-muted-foreground">
                      %
                    </span>
                  </div>
                  {error && (
                    <p
                      id={`${id}-error`}
                      role="alert"
                      className="w-full text-sm text-destructive"
                    >
                      {error}
                    </p>
                  )}
                </li>
              );
            })}
          </ul>
          <p
            role="status"
            aria-live="polite"
            className={cn(
              "flex flex-wrap items-center justify-between gap-2 text-sm",
              evaluation.valid ? "text-muted-foreground" : "text-foreground",
            )}
          >
            <span className="font-medium">{t("weights.total")}</span>
            <span className="tabular-nums">{sumLabel}</span>
          </p>
          {evaluation.formError && dirty && (
            <p role="alert" className="text-sm text-destructive">
              {evaluation.formError}
            </p>
          )}
          {serverError && (
            <p role="alert" className="text-sm text-destructive">
              {serverError}
            </p>
          )}
        </CardContent>
      </Card>
      <div className="flex flex-wrap items-center gap-3">
        <Button
          type="submit"
          className="h-11"
          disabled={
            !dirty || !evaluation.valid || busy || form.formState.isSubmitting
          }
          aria-busy={save.isPending}
        >
          {save.isPending && (
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
          )}
          {save.isPending ? tCommon("actions.saving") : t("save")}
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="h-11"
          disabled={!dirty || busy}
          onClick={() => form.reset()}
        >
          {t("discard")}
        </Button>
        <p
          role="status"
          aria-live="polite"
          className="text-xs text-muted-foreground"
        >
          {save.isPending
            ? tCommon("actions.saving")
            : dirty
              ? t("unsaved")
              : ""}
        </p>
        {saved.configured && (
          <Button
            type="button"
            variant="outline"
            className="h-11 sm:ml-auto"
            disabled={busy}
            onClick={() => {
              reset.reset();
              setConfirmReset(true);
            }}
          >
            {t("resetDefault")}
          </Button>
        )}
      </div>
      <ConfirmDialog
        open={confirmReset}
        title={t("resetDialog.title")}
        description={t("resetDialog.description", {
          mode: splitMode,
          total: splitTotal,
          dirty: dirty ? "yes" : "no",
        })}
        cancelLabel={t("resetDialog.cancel")}
        confirmLabel={t("resetDialog.confirm")}
        pendingLabel={t("resetDialog.pending")}
        pending={reset.isPending}
        error={resetError}
        onCancel={() => setConfirmReset(false)}
        onConfirm={() => reset.mutate()}
      />
    </form>
  );
}

function ReadOnlyWeights({ saved }: { saved: PostingWeights }) {
  const t = useTranslations("recruiter.evaluation");
  const fmt = useFormatters();
  const sum = saved.weights.reduce((acc, weight) => acc + weight.percentage, 0);
  return (
    <div className="space-y-6">
      <Notice tone="locked">
        <p>{t("readOnly.locked")}</p>
      </Notice>
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("weights.title")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {saved.using_defaults && (
            <p className="text-sm text-muted-foreground">
              {t("readOnly.noCustom")}
            </p>
          )}
          <ul className="divide-y rounded-xl border bg-muted/20 text-sm">
            {saved.weights.map((weight) => (
              <li
                key={weight.metric_id}
                className="flex flex-wrap justify-between gap-2 p-3"
              >
                <span className="font-medium [overflow-wrap:anywhere]">
                  {weight.name}
                </span>
                <span className="tabular-nums">
                  {formatPercent(weight.percentage, fmt)}%
                </span>
              </li>
            ))}
          </ul>
          <p className="flex justify-between text-sm">
            <span className="font-medium">{t("weights.total")}</span>
            <span className="tabular-nums">{formatPercent(sum, fmt)}%</span>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
