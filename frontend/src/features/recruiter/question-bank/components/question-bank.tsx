"use client";

import { GradingBank } from "./grading-bank";
import { useUnsavedChangesGuard } from "@/lib/hooks/use-unsaved-changes-guard";
import { ConfirmDialog } from "@/features/recruiter/evaluation/components/confirm-dialog";
import { ReviewSubmissionDialog } from "./review-submission-dialog";
import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ListPlus, LoaderCircle, Pencil, Trash2 } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  getPosting,
  jobPostingKeys,
  resubmitPosting,
  submitPosting,
} from "@/features/recruiter/job-postings/api/job-postings";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { PostingTabs } from "@/features/recruiter/job-postings/components/posting-tabs";
import { PostingStatus } from "@/features/recruiter/job-postings/components/posting-status";
import {
  addQuestion,
  deleteQuestion,
  generateBank,
  getBank,
  questionBankKeys,
  updateQuestion,
} from "../api/question-bank";
import type { Bank, CoreQuestion } from "../schemas/question-bank-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { DeleteQuestionDialog } from "./delete-question-dialog";
import { QuestionEditor } from "./question-editor";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";

// Codes after which the cached bank is stale: the posting left "pending", the bank was filled
// elsewhere, or the posting/question is gone. Refetching shows the server's truth.
const staleCodes = new Set([
  "QUESTION_BANK_NOT_READY",
  "QUESTION_BANK_LOCKED",
  "QUESTION_BANK_EXISTS",
  "GRADING_CONFIG_INCOMPLETE",
  "JOB_POSTING_NOT_FOUND",
  "CORE_QUESTION_NOT_FOUND",
  // The Admin decided the posting meanwhile (for example a reject while this page was open): reload
  // so Submit turns into Resubmit, or the bank becomes read-only.
  "JOB_POSTING_NOT_PENDING",
  "JOB_POSTING_NOT_REJECTED",
  "JOB_POSTING_NOT_EDITABLE",
]);

type BankTranslator = ReturnType<
  typeof useTranslations<"recruiter.questionBank">
>;

export function QuestionBank({ postingId }: { postingId: string }) {
  const t = useTranslations("recruiter.questionBank");
  const tp = useTranslations("recruiter.postings");
  const tCommon = useTranslations("common");
  const errorMessage = useApiErrorMessage();
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = questionBankKeys.detail(user.id, postingId);
  const [gradingDirty, setGradingDirty] = useState(false);
  const [gradingPending, setGradingPending] = useState(false);
  const [editDirty, setEditDirty] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [addDirty, setAddDirty] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<CoreQuestion | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const guard = useUnsavedChangesGuard(addDirty || editDirty || gradingDirty);

  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getBank(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  // Only names the posting in the way back; the bank works without it.
  const posting = useQuery({
    queryKey: jobPostingKeys.detail(user.id, postingId),
    queryFn: ({ signal }) => getPosting(postingId, signal),
    retry: false,
  });
  const patch = (update: (bank: Bank) => Bank) =>
    cache.setQueryData<Bank>(key, (bank) => {
      if (!bank) return bank;
      const next = update(bank);
      const count = next.questions.filter((q) => q.grading_incomplete).length;
      return {
        ...next,
        grading_incomplete_count: count,
        grading_ready: next.questions.length > 0 && count === 0,
      };
    });
  const savedGrading = useCallback(
    (questions: Bank["questions"]) => {
      cache.setQueryData<Bank>(
        questionBankKeys.detail(user.id, postingId),
        (bank) => {
          if (!bank) return bank;
          const next = bank.questions.map(
            (q) => questions.find((saved) => saved.id === q.id) ?? q,
          );
          const count = next.filter((q) => q.grading_incomplete).length;
          const changed = questions.some(
            (saved) =>
              bank.questions.find((q) => q.id === saved.id)?.revision !==
              saved.revision,
          );
          return {
            ...bank,
            questions: next,
            grading_incomplete_count: count,
            grading_ready: count === 0 && next.length > 0,
            review_submitted: changed ? false : bank.review_submitted,
          };
        },
      );
      void cache.invalidateQueries({
        queryKey: jobPostingKeys.detail(user.id, postingId),
      });
    },
    [cache, user.id, postingId],
  );
  const onStale = (error: unknown) => {
    const { code } = apiError(error);
    if (!code || !staleCodes.has(code)) return false;
    setNotice(errorMessage(error, t("changed")));
    setEditingId(null);
    setEditDirty(false);
    setDeleting(null);
    void cache.invalidateQueries({ queryKey: key });
    return true;
  };

  const refreshPosting = () => {
    void cache.invalidateQueries({
      queryKey: jobPostingKeys.detail(user.id, postingId),
    });
    void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
  };
  const sendForReview = useMutation({
    mutationFn: (bank: Bank) =>
      bank.posting_status === "rejected"
        ? resubmitPosting(postingId)
        : submitPosting(postingId),
    retry: false,
    onSuccess: (posting) => {
      patch((bank) => ({
        ...bank,
        posting_status: posting.status,
        review_submitted: posting.review_submitted,
      }));
      refreshPosting();
      setReviewOpen(false);
      gooeyToast.success(t("submit.toast"));
    },
    onError: onStale,
  });

  const generate = useMutation({
    mutationFn: () => generateBank(postingId),
    retry: false,
    onSuccess: (questions) => {
      patch((bank) => ({ ...bank, questions, editable: true }));
      refreshPosting();
      setNotice(null);
      gooeyToast.success(tp("toast.bankGenerated"), {
        description: t("generate.toast", { count: questions.length }),
      });
    },
    onError: onStale,
  });
  const add = useMutation({
    mutationFn: (content: string) => addQuestion(postingId, content),
    retry: false,
    onSuccess: (question) => {
      patch((bank) => ({
        ...bank,
        questions: [...bank.questions, question],
        review_submitted: false,
      }));
      setAddDirty(false);
      refreshPosting();
    },
    onError: onStale,
  });
  const update = useMutation({
    mutationFn: (input: { id: string; content: string }) =>
      updateQuestion(
        postingId,
        input.id,
        input.content,
        query.data?.questions.find((q) => q.id === input.id)?.revision,
      ),
    retry: false,
    onSuccess: (question) => {
      patch((bank) => ({
        ...bank,
        review_submitted:
          bank.questions.find((q) => q.id === question.id)?.content ===
          question.content
            ? bank.review_submitted
            : false,
        questions: bank.questions.map((q) =>
          q.id === question.id ? question : q,
        ),
      }));
      setEditingId(null);
      setEditDirty(false);
    },
    onError: onStale,
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteQuestion(postingId, id),
    retry: false,
    onSuccess: (_, id) => {
      const withdrawn =
        query.data?.review_submitted && query.data.questions.length === 1;
      // The server withdraws the submission with the last question; an empty bank is generated
      // again rather than edited.
      patch((bank) => {
        const questions = bank.questions.filter((q) => q.id !== id);
        return questions.length > 0
          ? { ...bank, questions, review_submitted: false }
          : { ...bank, questions, editable: false, review_submitted: false };
      });
      setDeleting(null);
      refreshPosting();
      gooeyToast.success(
        t("delete.toast"),
        withdrawn ? { description: t("delete.withdrawn") } : undefined,
      );
    },
    onError: onStale,
  });

  const denied = [query, generate, add, update, remove, sendForReview].some(
    (state) => isAccessDenied(state.error),
  );
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  // Mutation errors that the stale handler didn't take over are shown next to the form they came from.
  const formError = (
    state: { isError: boolean; error: unknown },
    fallback: string,
  ) => {
    if (!state.isError || isAccessDenied(state.error)) return null;
    const { code } = apiError(state.error);
    if (code && staleCodes.has(code)) return null;
    return errorMessage(state.error, fallback);
  };
  const submitError = formError(sendForReview, t("submit.failed"));
  const submit = async (run: () => Promise<unknown>) => {
    try {
      await run();
      return true;
    } catch {
      return false;
    }
  };

  const back = {
    href: routes.recruiter.jobPostings,
    label: tp("backToList"),
  };

  // Why Submit is off, in the order the Recruiter can fix things: nothing to send, then work in
  // progress that would be lost, then saves still running.
  const submitBlock = !query.data
    ? null
    : gradingPending
      ? t("blocked.saving")
      : gradingDirty
        ? t("grading.saveFirst")
        : query.data.questions.length === 0
          ? t("blocked.empty")
          : editingId !== null
            ? t("blocked.editing")
            : addDirty
              ? t("blocked.adding")
              : add.isPending || update.isPending || remove.isPending
                ? t("blocked.saving")
                : generate.isPending
                  ? t("blocked.generating")
                  : !query.data.grading_ready
                    ? t("grading.incomplete", {
                        count:
                          query.data.grading_incomplete_count ??
                          query.data.questions.filter(
                            (q) => q.grading_incomplete,
                          ).length,
                      })
                    : null;

  return (
    <section data-question-bank className="mx-auto w-full max-w-5xl space-y-6">
      {query.isPending ? (
        <>
          <PageHeader back={back} title={t("title")} />
          <div role="status" className="space-y-3">
            <Skeleton className="h-12 rounded-md" />
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
            <span className="sr-only">{t("loading")}</span>
          </div>
        </>
      ) : denied ? (
        <>
          <PageHeader back={back} title={t("title")} />
          <div role="alert" className="rounded-xl border p-6">
            <h2 className="text-base font-semibold">{t("denied.title")}</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {t("denied.body")}
            </p>
          </div>
        </>
      ) : query.isError ? (
        <>
          <PageHeader back={back} title={t("title")} />
          <div role="alert" className="rounded-xl border p-6">
            <h2 className="text-base font-semibold">
              {apiError(query.error).status === 404
                ? t("error.notFoundTitle")
                : t("error.failedTitle")}
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {apiError(query.error).status === 404
                ? t("error.notFoundBody")
                : tCommon("messages.tryAgainLater")}
            </p>
            {apiError(query.error).status !== 404 && (
              <Button
                className={cn("mt-4", pillClass)}
                variant="outline"
                disabled={query.isFetching}
                onClick={() => void query.refetch()}
              >
                {tCommon("actions.tryAgain")}
              </Button>
            )}
          </div>
        </>
      ) : (
        <>
          <PageHeader
            back={back}
            title={t("title")}
            meta={
              <>
                <PostingStatus
                  status={query.data.posting_status}
                  questionBankReady={query.data.grading_ready}
                  reviewSubmitted={query.data.review_submitted}
                  interviewSlot={posting.data?.interview_slot}
                />
                <span>{t("hiddenFromCandidates")}</span>
              </>
            }
            description={bankState(query.data, t)}
            actions={
              preparing(query.data) && !query.data.review_submitted ? (
                <div className="flex flex-col gap-1.5 sm:items-end">
                  <Button
                    className={pillClass}
                    disabled={submitBlock !== null || sendForReview.isPending}
                    aria-busy={sendForReview.isPending}
                    aria-describedby={
                      submitBlock ? "submit-block-reason" : undefined
                    }
                    onClick={() => setReviewOpen(true)}
                  >
                    {sendForReview.isPending && (
                      <LoaderCircle
                        className="size-4 animate-spin motion-reduce:animate-none"
                        aria-hidden
                      />
                    )}
                    {sendForReview.isPending
                      ? t("submit.pending")
                      : query.data.posting_status === "rejected"
                        ? t("submit.resubmit")
                        : t("submit.submit")}
                  </Button>
                  {submitBlock && (
                    <p
                      id="submit-block-reason"
                      className="max-w-64 text-xs text-muted-foreground sm:text-right"
                    >
                      {submitBlock}
                    </p>
                  )}
                </div>
              ) : query.data.review_submitted &&
                query.data.posting_status === "pending" ? (
                <span className="inline-flex h-10 items-center gap-2 text-sm font-medium">
                  <span
                    className="t-success-check text-emerald-600 dark:text-emerald-400"
                    data-state={sendForReview.isSuccess ? "in" : undefined}
                    style={sendForReview.isSuccess ? undefined : { opacity: 1 }}
                    aria-hidden
                  >
                    <svg
                      viewBox="0 0 24 24"
                      className="size-5"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth={2.5}
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path
                        d="M5 12l4 4L19 6"
                        style={
                          sendForReview.isSuccess
                            ? undefined
                            : { strokeDashoffset: 0 }
                        }
                      />
                    </svg>
                  </span>
                  {t("submit.inQueue")}
                </span>
              ) : null
            }
          />

          {submitError && (
            <p role="alert" className="text-sm text-destructive">
              {submitError}
            </p>
          )}

          <PostingTabs
            postingId={postingId}
            active="questions"
            status={query.data.posting_status}
          />
          <p className="text-sm text-muted-foreground">
            {preparing(query.data)
              ? t("evaluation.preparing")
              : t("evaluation.locked")}
          </p>
          {reviewOpen && (
            <ReviewSubmissionDialog
              postingId={postingId}
              posting={posting.data ?? null}
              questionCount={query.data.questions.length}
              rejected={query.data.posting_status === "rejected"}
              pending={sendForReview.isPending}
              error={submitError}
              onCancel={() => setReviewOpen(false)}
              onConfirm={() => sendForReview.mutate(query.data)}
            />
          )}

          {notice && (
            <Notice role="alert" tone="danger">
              <p>{notice}</p>
            </Notice>
          )}

          <section aria-labelledby="questions-heading" className="space-y-4">
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h2 id="questions-heading" className="text-base font-semibold">
                {t("list.title")}
              </h2>
              <span className="text-sm text-muted-foreground tabular-nums">
                {t("list.count", { count: query.data.questions.length })}
              </span>
            </div>
            {query.data.questions.length === 0 ? (
              <EmptyBank
                bank={query.data}
                generating={generate.isPending}
                error={formError(generate, t("generate.failed"))}
                onGenerate={() => generate.mutate()}
              />
            ) : (
              <GradingBank
                bank={query.data}
                adding={add.isPending}
                addError={
                  add.isError ? errorMessage(add.error, t("add.failed")) : null
                }
                newQuestionDirty={addDirty}
                onNewQuestionDirtyChange={setAddDirty}
                onAddQuestion={async (content) => {
                  try {
                    return await add.mutateAsync(content);
                  } catch {
                    return null;
                  }
                }}
                postingId={postingId}
                onSaved={savedGrading}
                onDirtyChange={setGradingDirty}
                onPendingChange={setGradingPending}
                disabled={
                  add.isPending ||
                  update.isPending ||
                  remove.isPending ||
                  editDirty ||
                  addDirty
                }
                renderQuestion={(question, index) => (
                  <div
                    className={cn(
                      "min-w-0",
                      editingId === question.id && "bg-muted/40",
                    )}
                  >
                    {editingId === question.id ? (
                      <QuestionEditor
                        onDirtyChange={setEditDirty}
                        id={`edit-${question.id}`}
                        label={t("list.editLabel", { number: index + 1 })}
                        initial={question.content}
                        maxLength={query.data.max_question_length}
                        pending={update.isPending}
                        error={formError(update, t("edit.failed"))}
                        submitLabel={t("edit.save")}
                        pendingLabel={tCommon("actions.saving")}
                        onCancel={() => {
                          update.reset();
                          setEditingId(null);
                          setEditDirty(false);
                        }}
                        onSubmit={(content) =>
                          submit(() =>
                            update.mutateAsync({ id: question.id, content }),
                          )
                        }
                      />
                    ) : (
                      <div className="grid grid-cols-[1.75rem_minmax(0,1fr)] items-start gap-x-3 gap-y-1 sm:grid-cols-[1.75rem_minmax(0,1fr)_auto] sm:gap-x-4">
                        <span className="pt-px text-right text-sm text-muted-foreground tabular-nums">
                          {index + 1}.
                        </span>
                        <p className="text-[15px] leading-relaxed whitespace-pre-line [overflow-wrap:anywhere]">
                          {question.content}
                        </p>
                        {query.data.editable && (
                          <div className="col-start-2 -ml-2.5 flex gap-0.5 sm:col-start-3 sm:-my-2 sm:ml-0">
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-11 rounded-full text-rolecue-brand hover:bg-rolecue-brand/10 hover:text-rolecue-brand"
                              aria-label={t("list.editLabel", {
                                number: index + 1,
                              })}
                              disabled={
                                editingId !== null ||
                                gradingDirty ||
                                gradingPending
                              }
                              onClick={() => {
                                update.reset();
                                setEditingId(question.id);
                              }}
                            >
                              <Pencil className="size-4" aria-hidden />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-11 rounded-full text-destructive/80 hover:bg-destructive/8 hover:text-destructive"
                              disabled={
                                gradingDirty ||
                                gradingPending ||
                                editingId !== null ||
                                addDirty
                              }
                              aria-label={t("list.deleteLabel", {
                                number: index + 1,
                              })}
                              onClick={() => {
                                remove.reset();
                                setDeleting(question);
                              }}
                            >
                              <Trash2 className="size-4" aria-hidden />
                            </Button>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              />
            )}
          </section>

          <DeleteQuestionDialog
            question={deleting}
            pending={remove.isPending}
            error={formError(remove, t("delete.failed"))}
            onCancel={() => setDeleting(null)}
            onConfirm={() => deleting && remove.mutate(deleting.id)}
          />
        </>
      )}
      <ConfirmDialog
        open={guard.leaving}
        title={t("grading.leaveTitle")}
        description={t("grading.leaveHint")}
        cancelLabel={t("grading.stay")}
        confirmLabel={t("grading.leave")}
        onCancel={guard.stay}
        onConfirm={guard.leave}
      />
    </section>
  );
}

// The sentence under the title: what the bank's state lets the Recruiter do next.
function bankState(bank: Bank, t: BankTranslator) {
  switch (bank.posting_status) {
    case "open":
      return t("state.readOnlyApproved");
    case "intake_closed":
      return t("state.readOnlyIntakeClosed");
    case "closed":
      return t("state.readOnlyClosed");
    case "pending":
    case "rejected":
      break;
  }
  if (bank.review_submitted) return t("state.submitted");
  return bank.questions.length > 0
    ? t("state.review")
    : t("state.needsQuestion");
}

// Questions change, and the posting is submitted, while it is prepared: pending or rejected.
function preparing(bank: Bank) {
  return (
    bank.posting_status === "pending" || bank.posting_status === "rejected"
  );
}

function EmptyBank({
  bank,
  generating,
  error,
  onGenerate,
}: {
  bank: Bank;
  generating: boolean;
  error: string | null;
  onGenerate: () => void;
}) {
  const t = useTranslations("recruiter.questionBank.empty");
  if (!preparing(bank))
    return (
      <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
        {t("none")}
      </p>
    );
  if (bank.generation_size === 0)
    return (
      <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
        {t("unavailable")}
      </p>
    );
  return (
    <div className="space-y-4 rounded-xl border border-dashed px-6 py-10 text-center">
      <div className="space-y-1">
        <h3 className="font-semibold">{t("title")}</h3>
        <p className="mx-auto max-w-[52ch] text-sm text-muted-foreground">
          {t("description", { count: bank.generation_size })}
        </p>
      </div>
      <Button
        className={pillClass}
        disabled={generating}
        aria-busy={generating}
        onClick={onGenerate}
      >
        {generating ? (
          <LoaderCircle
            className="size-4 animate-spin motion-reduce:animate-none"
            aria-hidden
          />
        ) : (
          <ListPlus className="size-4" aria-hidden />
        )}
        {generating ? t("generating") : error ? t("retry") : t("generate")}
      </Button>
      <p role="status" aria-live="polite" className="text-[13px]">
        {generating ? (
          <span className="t-shimmer" data-text={t("wait")}>
            {t("wait")}
          </span>
        ) : (
          ""
        )}
      </p>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
