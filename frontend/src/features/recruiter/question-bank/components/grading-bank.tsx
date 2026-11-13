"use client";
import { ErrorState } from "@/components/ui/error-state";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import {
  ArrowRight,
  BookOpen,
  Check,
  ChevronRight,
  CircleAlert,
  ListChecks,
  LoaderCircle,
  Plus,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { ConfirmDialog } from "@/features/recruiter/evaluation/components/confirm-dialog";
import {
  getRubric,
  listActiveRubrics,
  rubricKeys,
} from "@/features/rubrics/api/rubrics";
import { RubricCriteria } from "@/features/rubrics/components/rubric-criteria";
import { RubricPicker } from "@/features/rubrics/components/rubric-picker";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { cn } from "@/lib/utils";
import type { Bank, BankQuestion } from "../schemas/question-bank-schema";
import { applyRubric } from "../api/question-bank";
import { GradingEditor } from "./grading-editor";
import { QuestionEditor } from "./question-editor";

type Workspace =
  | { kind: "new" }
  | { kind: "grading"; question: BankQuestion }
  | { kind: "bulk" };
const ignoreDirty = () => {};

export function GradingBank({
  bank,
  postingId,
  renderQuestion,
  onSaved,
  onDirtyChange,
  onPendingChange,
  disabled,
  onAddQuestion,
  adding = false,
  addError = null,
  newQuestionDirty = false,
  onNewQuestionDirtyChange = ignoreDirty,
}: {
  bank: Bank;
  postingId: string;
  renderQuestion: (question: BankQuestion, index: number) => ReactNode;
  onSaved: (questions: BankQuestion[]) => void;
  onDirtyChange: (dirty: boolean) => void;
  onPendingChange: (pending: boolean) => void;
  disabled: boolean;
  onAddQuestion?: (content: string) => Promise<BankQuestion | null>;
  adding?: boolean;
  addError?: string | null;
  newQuestionDirty?: boolean;
  onNewQuestionDirtyChange?: (dirty: boolean) => void;
}) {
  const t = useTranslations("recruiter.questionBank.grading");
  const bankT = useTranslations("recruiter.questionBank");
  const common = useTranslations("common");
  const user = useSessionUser();
  const errorMessage = useApiErrorMessage();
  const catalog = useQuery({
    queryKey: [...rubricKeys.all(user.id), "bank-options"],
    queryFn: ({ signal }) => listActiveRubrics(signal),
    retry: false,
  });
  const [selected, setSelected] = useState<string[]>([]);
  const [rubricId, setRubricId] = useState("");
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [applied, setApplied] = useState<BankQuestion[] | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [highlighted, setHighlighted] = useState<string | null>(null);
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const revealAfterClose = useRef<string | null>(null);
  const hasDirty = Object.values(dirty).some(Boolean);
  const hasPending = Object.values(pending).some(Boolean);
  const markDirty = useCallback(
    (id: string, value: boolean) =>
      setDirty((previous) =>
        previous[id] === value ? previous : { ...previous, [id]: value },
      ),
    [],
  );
  const markPending = useCallback(
    (id: string, value: boolean) =>
      setPending((previous) =>
        previous[id] === value ? previous : { ...previous, [id]: value },
      ),
    [],
  );
  const bulk = useMutation({
    mutationFn: () =>
      applyRubric(postingId, {
        rubric_id: rubricId,
        question_ids: selected,
        previous_revisions: Object.fromEntries(
          bank.questions
            .filter((q) => selected.includes(q.id))
            .map((q) => [q.id, q.revision]),
        ),
      }),
    onSuccess: (questions) => {
      setSelected([]);
      setApplied(questions);
      onSaved(questions);
    },
  });
  useEffect(() => onDirtyChange(hasDirty), [hasDirty, onDirtyChange]);
  useEffect(
    () => onPendingChange(hasPending || bulk.isPending),
    [hasPending, bulk.isPending, onPendingChange],
  );
  const closeWorkspace = () => {
    const id =
      workspace?.kind === "grading" ? workspace.question.id : applied?.[0]?.id;
    revealAfterClose.current = id ?? null;
    if (id) setHighlighted(id);
    setWorkspace(null);
    setDiscardOpen(false);
    onNewQuestionDirtyChange(false);
  };
  const requestClose = () => {
    if (adding || hasPending || bulk.isPending) return;
    if (hasDirty || newQuestionDirty) setDiscardOpen(true);
    else closeWorkspace();
  };
  const incomplete = bank.questions.filter((q) => q.grading_incomplete);
  const groups = new Map<string, BankQuestion[]>();
  if (bank.questions.some((q) => !q.rubric_id)) groups.set("", []);
  for (const question of bank.questions) {
    const id = question.rubric_id ?? "";
    groups.set(id, [...(groups.get(id) ?? []), question]);
  }
  const toggle = (id: string, checked: boolean) =>
    setSelected((previous) =>
      checked
        ? [...new Set([...previous, id])]
        : previous.filter((value) => value !== id),
    );
  const selectedQuestion =
    workspace?.kind === "grading" ? workspace.question : null;
  const number = selectedQuestion
    ? Math.max(
        1,
        bank.questions.findIndex((q) => q.id === selectedQuestion.id) + 1,
      )
    : 0;

  return (
    <div className="space-y-5">
      <div
        className={cn(
          "overflow-hidden rounded-xl border bg-card",
          incomplete.length
            ? "border-amber-500/25"
            : "border-(--rolecue-cue-green)/25",
        )}
        aria-labelledby="grading-readiness"
      >
        <div className="flex items-start gap-3 p-5 sm:p-6">
          <span
            className={cn(
              "flex size-10 shrink-0 items-center justify-center rounded-full",
              incomplete.length
                ? "bg-amber-50 text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
                : "bg-(--rolecue-cue-green) text-white",
            )}
          >
            {incomplete.length ? (
              <ListChecks className="size-5" aria-hidden />
            ) : (
              <Check className="size-6" strokeWidth={2.5} aria-hidden />
            )}
          </span>
          <div className="min-w-0 space-y-1.5">
            <h3
              id="grading-readiness"
              className="text-base font-semibold tracking-tight"
            >
              {incomplete.length
                ? t("incomplete", { count: incomplete.length })
                : t("ready")}
            </h3>
            <p className="max-w-[78ch] text-sm leading-relaxed text-muted-foreground">
              {incomplete.length ? t("incompleteHint") : t("readyHint")}
            </p>
          </div>
        </div>
        <ol
          aria-label={t("reviewSteps")}
          className="grid gap-3 border-t px-5 py-4 text-sm sm:grid-cols-3 sm:px-6"
        >
          {["rubricStep", "answerStep", "submitStep"].map((step, index) => (
            <li key={step} className="flex items-center gap-2.5">
              <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-rolecue-brand/10 text-xs font-semibold text-rolecue-brand">
                {index + 1}
              </span>
              <span className="font-medium">
                {t(step as "rubricStep" | "answerStep" | "submitStep")}
              </span>
            </li>
          ))}
        </ol>
        {incomplete.length > 0 && (
          <ul className="grid gap-1 border-t px-4 py-3 sm:grid-cols-2">
            {incomplete.map((q) => (
              <li key={q.id}>
                <a
                  className="inline-flex min-h-11 items-center gap-2 rounded-lg px-2 text-sm text-amber-800 underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring dark:text-amber-300"
                  href={`#question-${q.id}`}
                >
                  <CircleAlert className="size-4 shrink-0" aria-hidden />
                  <span>
                    {t("questionIssue", {
                      number: bank.questions.indexOf(q) + 1,
                    })}
                    :{" "}
                    {q.grading_issues
                      .map((issue) => t(`issues.${issue}`))
                      .join(" · ")}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 px-1">
        <p className="max-w-[65ch] text-sm leading-relaxed text-muted-foreground">
          {t("bankHint")}
        </p>
        {bank.editable && onAddQuestion && (
          <Button
            variant="outline"
            className="min-h-11 rounded-full px-5"
            disabled={disabled || hasPending}
            onClick={() => {
              setApplied(null);
              setWorkspace({ kind: "new" });
            }}
          >
            <Plus className="size-4" aria-hidden />
            {bankT("add.label")}
          </Button>
        )}
      </div>
      {bank.editable && selected.length > 0 && (
        <div
          className="sticky top-3 z-10 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-rolecue-brand/25 bg-card p-4 shadow-md"
          aria-label={t("selectionActions")}
        >
          <div>
            <p
              role="status"
              className="text-sm font-semibold text-rolecue-brand"
            >
              {t("selected", {
                count: selected.length,
                max: bank.max_bulk_questions,
              })}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              {t("selectionHint")}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              className="min-h-11 rounded-full"
              aria-label={t("clearSelection")}
              disabled={bulk.isPending}
              onClick={() => setSelected([])}
            >
              <X className="size-4" aria-hidden />
              {t("clearSelection")}
            </Button>
            <Button
              className="min-h-11 rounded-full"
              disabled={
                selected.length > bank.max_bulk_questions ||
                disabled ||
                hasPending ||
                hasDirty
              }
              onClick={() => {
                setApplied(null);
                bulk.reset();
                setWorkspace({ kind: "bulk" });
              }}
            >
              {t("changeSelectedRubric")}
              <ArrowRight className="size-4" aria-hidden />
            </Button>
          </div>
        </div>
      )}
      {catalog.isError && (
        <ErrorState
          description={errorMessage(catalog.error, t("rubricsFailed"))}
          onRetry={() => void catalog.refetch()}
          retryLabel={common("actions.tryAgain")}
        ></ErrorState>
      )}

      {[...groups].map(([id, questions]) => {
        const rubric = catalog.data?.find((r) => r.id === id);
        return (
          <section
            key={id}
            className="overflow-hidden rounded-2xl border bg-card"
            aria-labelledby={`rubric-group-${id || "none"}`}
          >
            <div className="space-y-2 border-b px-5 py-4 sm:px-6">
              <div className="flex items-start gap-3">
                <span
                  className={cn(
                    "mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl",
                    id
                      ? "bg-rolecue-brand/10 text-rolecue-brand"
                      : "bg-amber-50 text-amber-800 dark:bg-amber-950/50 dark:text-amber-300",
                  )}
                >
                  <BookOpen className="size-4.5" aria-hidden />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                    <h3
                      id={`rubric-group-${id || "none"}`}
                      className="text-base font-semibold tracking-tight wrap-anywhere"
                    >
                      {id ? (
                        <RubricGroupName
                          id={id}
                          name={rubric?.name}
                          load={catalog.isSuccess && !rubric}
                          fallback={t(
                            catalog.isPending
                              ? "loadingRubrics"
                              : "unavailableRubric",
                          )}
                        />
                      ) : (
                        t("noRubric")
                      )}
                    </h3>
                    <span className="rounded-full bg-muted px-3 py-1 text-xs font-medium text-muted-foreground tabular-nums">
                      {t("groupCount", { count: questions.length })}
                    </span>
                  </div>
                  <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                    {id ? t("groupHint") : t("unassignedHint")}
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 sm:pl-12">
                {id ? <RubricCriteria rubricId={id} /> : <span />}
                {bank.editable && (
                  <label className="flex min-h-11 items-center gap-2.5 text-xs font-medium text-muted-foreground">
                    <Checkbox
                      checked={
                        questions.every((q) => selected.includes(q.id))
                          ? true
                          : questions.some((q) => selected.includes(q.id))
                            ? "indeterminate"
                            : false
                      }
                      disabled={disabled || hasPending || bulk.isPending}
                      onCheckedChange={(checked) =>
                        setSelected((previous) =>
                          checked === true
                            ? [
                                ...new Set([
                                  ...previous,
                                  ...questions.map((q) => q.id),
                                ]),
                              ]
                            : previous.filter(
                                (value) =>
                                  !questions.some((q) => q.id === value),
                              ),
                        )
                      }
                    />
                    {t("selectGroupCount", { count: questions.length })}
                  </label>
                )}
              </div>
            </div>
            <ol className="divide-y">
              {questions.map((question) => {
                const index = bank.questions.indexOf(question);
                return (
                  <li
                    key={question.id}
                    id={`question-${question.id}`}
                    tabIndex={-1}
                    data-highlighted={highlighted === question.id || undefined}
                    className="scroll-mt-28 px-4 py-5 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-rolecue-brand data-highlighted:bg-rolecue-brand/5 data-highlighted:ring-2 data-highlighted:ring-inset data-highlighted:ring-rolecue-brand/40 motion-reduce:transition-none sm:px-6"
                  >
                    {highlighted === question.id && (
                      <p className="mb-3 inline-flex items-center gap-1.5 rounded-full bg-rolecue-brand/10 px-3 py-1 text-xs font-medium text-rolecue-brand">
                        <Check className="size-3.5" aria-hidden />
                        {t("recentlyEdited")}
                      </p>
                    )}
                    <div
                      className={
                        bank.editable
                          ? "grid grid-cols-[1rem_minmax(0,1fr)] items-start gap-3"
                          : ""
                      }
                    >
                      {bank.editable && (
                        <Checkbox
                          className="mt-1"
                          aria-label={t("selectQuestion", {
                            number: index + 1,
                          })}
                          checked={selected.includes(question.id)}
                          disabled={disabled || hasPending || bulk.isPending}
                          onCheckedChange={(checked) =>
                            toggle(question.id, checked === true)
                          }
                        />
                      )}
                      {renderQuestion(question, index)}
                    </div>
                    <div className="mt-3 flex flex-wrap items-center justify-between gap-2 pl-7 sm:pl-16">
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 text-xs font-medium",
                          question.grading_incomplete
                            ? "text-amber-800 dark:text-amber-300"
                            : "text-(--rolecue-status-on-ink)",
                        )}
                      >
                        {question.grading_incomplete ? (
                          <CircleAlert className="size-3.5" aria-hidden />
                        ) : (
                          <Check
                            className="size-3.5"
                            strokeWidth={2.5}
                            aria-hidden
                          />
                        )}
                        {t(
                          question.grading_incomplete
                            ? "needsConfiguration"
                            : "configured",
                        )}
                      </span>
                      <Button
                        variant="ghost"
                        className="min-h-11 rounded-full text-xs font-medium text-rolecue-brand hover:bg-rolecue-brand/8 hover:text-rolecue-brand"
                        disabled={hasPending || bulk.isPending || disabled}
                        onClick={() => {
                          setApplied(null);
                          setWorkspace({ kind: "grading", question });
                        }}
                      >
                        {t(bank.editable ? "review" : "view", {
                          number: index + 1,
                        })}
                        <ChevronRight className="size-4" aria-hidden />
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ol>
          </section>
        );
      })}

      <Dialog
        open={workspace !== null}
        onOpenChange={(next) => {
          if (!next) requestClose();
        }}
      >
        <DialogContent
          className="motion-reduce:animate-none! max-h-[90dvh] overflow-y-auto p-5 sm:max-w-2xl sm:p-6"
          overlayClassName="motion-reduce:animate-none"
          onCloseAutoFocus={(event) => {
            const id = revealAfterClose.current;
            if (!id) return;
            event.preventDefault();
            revealAfterClose.current = null;
            requestAnimationFrame(() => {
              const element = document.getElementById(`question-${id}`);
              element?.scrollIntoView?.({
                block: "center",
                behavior: window.matchMedia("(prefers-reduced-motion: reduce)")
                  .matches
                  ? "auto"
                  : "smooth",
              });
              element?.focus({ preventScroll: true });
            });
          }}
        >
          <DialogHeader>
            <DialogTitle className="pr-8 text-xl tracking-tight">
              {workspace?.kind === "new"
                ? bankT("add.label")
                : workspace?.kind === "bulk"
                  ? t("bulkTitle")
                  : t(bank.editable ? "review" : "view", { number })}
            </DialogTitle>
            <DialogDescription className="leading-relaxed">
              {workspace?.kind === "new"
                ? t("newQuestionHint")
                : workspace?.kind === "bulk"
                  ? t("bulkPurpose")
                  : t("editorHint")}
            </DialogDescription>
          </DialogHeader>
          {workspace?.kind === "new" && onAddQuestion && (
            <QuestionEditor
              id="new-question"
              label={bankT("add.label")}
              maxLength={bank.max_question_length}
              pending={adding}
              error={addError}
              submitLabel={bankT("add.submit")}
              pendingLabel={bankT("add.pending")}
              onDirtyChange={onNewQuestionDirtyChange}
              onCancel={requestClose}
              onSubmit={async (content) => {
                const question = await onAddQuestion(content);
                if (!question) return false;
                onNewQuestionDirtyChange(false);
                setWorkspace({ kind: "grading", question });
                return true;
              }}
            />
          )}
          {selectedQuestion && (
            <>
              <div className="rounded-2xl border border-rolecue-brand/15 bg-rolecue-brand/5 p-4 text-sm leading-relaxed whitespace-pre-line wrap-anywhere">
                {selectedQuestion.content}
              </div>
              <GradingEditor
                key={selectedQuestion.id}
                expanded
                question={selectedQuestion}
                number={number}
                bank={bank}
                postingId={postingId}
                rubrics={catalog.data ?? []}
                onSaved={(saved) => {
                  onSaved([saved]);
                  setWorkspace({ kind: "grading", question: saved });
                }}
                onDirtyChange={markDirty}
                onPendingChange={markPending}
                onDone={closeWorkspace}
              />
              {catalog.isPending && (
                <p role="status" className="text-sm text-muted-foreground">
                  {t("loadingRubrics")}
                </p>
              )}
            </>
          )}
          {workspace?.kind === "bulk" &&
            (applied ? (
              <div className="space-y-4">
                <div className="flex items-start gap-3 rounded-xl bg-(--rolecue-status-on-bg) p-4 text-(--rolecue-status-on-ink)">
                  <Check className="mt-0.5 size-5 shrink-0" aria-hidden />
                  <p className="text-sm font-medium">{t("bulkSaved")}</p>
                </div>
                <Button
                  className="min-h-11 rounded-full"
                  onClick={() =>
                    setWorkspace({
                      kind: "grading",
                      question:
                        applied.find((q) => q.grading_incomplete) ?? applied[0],
                    })
                  }
                >
                  {t("reviewUpdated")}
                  <ArrowRight className="size-4" aria-hidden />
                </Button>
              </div>
            ) : (
              <div className="space-y-4">
                <p className="text-sm font-medium text-rolecue-brand">
                  {t("selected", {
                    count: selected.length,
                    max: bank.max_bulk_questions,
                  })}
                </p>
                <RubricPicker
                  id="bulk-rubric"
                  rubrics={catalog.data ?? []}
                  value={rubricId}
                  onChange={setRubricId}
                  disabled={catalog.isPending || bulk.isPending}
                />
                <p className="rounded-xl bg-amber-50 p-3 text-sm leading-relaxed text-amber-800 dark:bg-amber-950/50 dark:text-amber-300">
                  {t("bulkHint")}
                </p>
                {bulk.isError && (
                  <ErrorState
                    description={errorMessage(bulk.error, t("bulkFailed"))}
                  >
                    <p>{t("bulkRetryHint")}</p>
                    <Button
                      variant="outline"
                      onClick={() => window.location.reload()}
                    >
                      {t("reload")}
                    </Button>
                  </ErrorState>
                )}
                <Button
                  className="min-h-11 rounded-full"
                  disabled={
                    !rubricId ||
                    catalog.isPending ||
                    catalog.isError ||
                    bulk.isPending ||
                    selected.length > bank.max_bulk_questions
                  }
                  onClick={() => bulk.mutate()}
                  aria-busy={bulk.isPending}
                >
                  {bulk.isPending && (
                    <LoaderCircle
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden
                    />
                  )}
                  {t("apply")}
                </Button>
              </div>
            ))}
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={discardOpen}
        title={t("leaveTitle")}
        description={t("closeDraftHint")}
        cancelLabel={t("stay")}
        confirmLabel={t("discardDraft")}
        onCancel={() => setDiscardOpen(false)}
        onConfirm={closeWorkspace}
      />
    </div>
  );
}

function RubricGroupName({
  id,
  name,
  load,
  fallback,
}: {
  id: string;
  name?: string;
  load: boolean;
  fallback: string;
}) {
  const user = useSessionUser();
  const rubric = useQuery({
    queryKey: [...rubricKeys.all(user.id), "detail", id],
    queryFn: ({ signal }) => getRubric(id, signal),
    enabled: load,
    retry: false,
  });
  return name ?? rubric.data?.name ?? fallback;
}
