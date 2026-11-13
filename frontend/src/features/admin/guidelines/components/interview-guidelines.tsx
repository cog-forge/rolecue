"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { CircleAlert, LoaderCircle } from "lucide-react";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useInterviewGuidelines } from "../hooks/use-interview-guidelines";
import type { Guidelines, GuidelineValues } from "../schemas/guidelines-schema";
import { GuidelinesForm } from "./guidelines-form";
import { GuidelinesStatus } from "./guidelines-status";
import { Reveal } from "./reveal";

// Admin: the bounded interview behavior guidelines (KAN-276). Only Administrators reach it (the admin
// layout's role gate); the API refuses everyone else as well.
// 44px high on phones (the touch target in DESIGN.md), the workspace's 40px pill from sm up.
const actionClass = "h-11 rounded-full px-5 sm:h-10";

export function InterviewGuidelines() {
  const t = useTranslations("admin.interviewGuidelines");
  const tCommon = useTranslations("common");
  const errorMessage = useApiErrorMessage();
  const { query, save, denied, outcome, dismissOutcome } =
    useInterviewGuidelines();
  const [resetting, setResetting] = useState(false);
  const guidelines = query.data;

  const submit = (values: GuidelineValues, reset = false) => {
    if (!guidelines) return;
    save.mutate({ expectedVersion: guidelines.version, values, reset });
  };
  // A conflict is explained by the banner (and the reload); only other failures get the form's own alert.
  const saveError =
    save.isError && outcome?.kind !== "conflict" && !denied
      ? errorMessage(save.error, t("feedback.saveFailed"))
      : null;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-7">
      <PageHeader
        title={t("title")}
        description={t("description")}
        meta={guidelines && <GuidelinesStatus guidelines={guidelines} />}
      />

      {query.isPending ? (
        <div role="status" className="space-y-4">
          <div aria-hidden className="space-y-4">
            <Skeleton className="h-24 rounded-xl" />
            <Skeleton className="h-32 rounded-xl" />
            <Skeleton className="h-48 rounded-xl" />
          </div>
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <ErrorState
          title={t("denied.title")}
          description={t("denied.description")}
        ></ErrorState>
      ) : query.isError || !guidelines ? (
        <ErrorState
          title={t("loadFailed.title")}
          description={tCommon("messages.tryAgainLater")}
          onRetry={() => void query.refetch()}
          retryLabel={tCommon("actions.tryAgain")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : (
        <div className="space-y-5">
          <Outcome
            guidelines={guidelines}
            outcome={outcome}
            onDismiss={dismissOutcome}
          />
          <GuidelinesForm
            // A new version (saved here, or loaded after a conflict) starts a clean form.
            key={guidelines.version}
            guidelines={guidelines}
            saving={save.isPending && !resetting}
            error={saveError}
            onSave={(values) => submit(values)}
            onRequestReset={() => {
              save.reset();
              setResetting(true);
            }}
          />
        </div>
      )}

      <Dialog
        open={resetting}
        onOpenChange={(open) => !open && !save.isPending && setResetting(false)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("reset.title")}</DialogTitle>
            <DialogDescription>{t("reset.description")}</DialogDescription>
          </DialogHeader>
          {resetting && save.isError && !denied && (
            <p
              role="alert"
              className="flex items-start gap-2 rounded-xl border border-destructive/30 p-3 text-sm"
            >
              <CircleAlert
                className="mt-0.5 size-4 shrink-0 text-destructive"
                aria-hidden
              />
              {outcome?.kind === "conflict"
                ? errorMessage(save.error)
                : errorMessage(save.error, t("feedback.saveFailed"))}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              className={actionClass}
              disabled={save.isPending}
              onClick={() => setResetting(false)}
            >
              {t("reset.cancel")}
            </Button>
            <Button
              className={actionClass}
              disabled={save.isPending || !guidelines}
              onClick={() =>
                guidelines &&
                save.mutate(
                  {
                    expectedVersion: guidelines.version,
                    values: guidelines.defaults,
                    reset: true,
                  },
                  { onSuccess: () => setResetting(false) },
                )
              }
            >
              {save.isPending && (
                <LoaderCircle className="size-4 animate-spin" aria-hidden />
              )}
              {save.isPending ? t("reset.pending") : t("reset.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

// What the last save did, in words. A conflict names the version that won, so the Admin knows the
// screen now shows someone else's values and their own edits were not kept.
function Outcome({
  guidelines,
  outcome,
  onDismiss,
}: {
  guidelines: Guidelines;
  outcome: ReturnType<typeof useInterviewGuidelines>["outcome"];
  onDismiss: () => void;
}) {
  const t = useTranslations("admin.interviewGuidelines.feedback");
  if (!outcome) return null;
  const conflict = outcome.kind === "conflict";
  return (
    <Reveal key={`${outcome.kind}-${guidelines.version}`}>
      <div
        role={conflict ? "alert" : "status"}
        className="flex flex-wrap items-start justify-between gap-3 rounded-xl border p-4"
      >
        <p className="flex min-w-0 flex-1 items-start gap-2 text-sm">
          {conflict ? (
            <CircleAlert
              className="mt-0.5 size-4 shrink-0 text-destructive"
              aria-hidden
            />
          ) : (
            <span className="t-success-check mt-0.5 shrink-0" data-state="in">
              <svg
                viewBox="0 0 24 24"
                width="16"
                height="16"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden
              >
                <path d="M5 12l4 4L19 6" />
              </svg>
            </span>
          )}
          <span>
            {conflict
              ? t("conflict", { version: guidelines.version })
              : outcome.kind === "reset"
                ? t("resetDone", { version: outcome.version })
                : t("saved", { version: outcome.version })}
          </span>
        </p>
        <Button
          variant="ghost"
          className="h-11 rounded-full px-4 sm:h-10"
          onClick={onDismiss}
        >
          {t("dismiss")}
        </Button>
      </div>
    </Reveal>
  );
}
