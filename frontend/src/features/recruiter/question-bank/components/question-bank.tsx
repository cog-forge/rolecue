"use client";

import { useEffect, useState } from "react";
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
import {
  PageHeader,
  pillClass,
} from "@/features/recruiter/job-postings/components/page-header";
import { PostingProgress } from "@/features/recruiter/job-postings/components/posting-progress";
import { PostingStatus } from "@/features/recruiter/job-postings/components/posting-status";
import {
  addQuestion,
  deleteQuestion,
  generateBank,
  getBank,
  questionBankKeys,
  updateQuestion,
} from "../api/question-bank";
import type {
  Bank,
  CoreQuestion,
  PostingStatus as Status,
} from "../schemas/question-bank-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { DeleteQuestionDialog } from "./delete-question-dialog";
import { QuestionEditor } from "./question-editor";

const statusText: Record<Status, string> = {
  pending: "Pending review",
  open: "Approved",
  intake_closed: "Intake closed",
  closed: "Closed",
  rejected: "Rejected",
};

// Codes after which the cached bank is stale: the posting left "pending", the bank was filled
// elsewhere, or the posting/question is gone. Refetching shows the server's truth.
const staleCodes = new Set([
  "QUESTION_BANK_NOT_READY",
  "QUESTION_BANK_LOCKED",
  "QUESTION_BANK_EXISTS",
  "JOB_POSTING_NOT_FOUND",
  "CORE_QUESTION_NOT_FOUND",
]);

export function QuestionBank({ postingId }: { postingId: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = questionBankKeys.detail(user.id, postingId);
  const [addDirty, setAddDirty] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<CoreQuestion | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

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
    cache.setQueryData<Bank>(key, (bank) => (bank ? update(bank) : bank));
  const onStale = (error: unknown) => {
    const { code, message } = apiError(error);
    if (!code || !staleCodes.has(code)) return false;
    setNotice(
      message ?? "This question bank changed. We refreshed it for you.",
    );
    setEditingId(null);
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
      gooeyToast.success("Submitted for approval");
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
      gooeyToast.success("Question bank generated", {
        description: `${questions.length} questions are ready to review.`,
      });
    },
    onError: onStale,
  });
  const add = useMutation({
    mutationFn: (content: string) => addQuestion(postingId, content),
    retry: false,
    onSuccess: (question) => {
      patch((bank) => ({ ...bank, questions: [...bank.questions, question] }));
      refreshPosting();
    },
    onError: onStale,
  });
  const update = useMutation({
    mutationFn: (input: { id: string; content: string }) =>
      updateQuestion(postingId, input.id, input.content),
    retry: false,
    onSuccess: (question) => {
      patch((bank) => ({
        ...bank,
        questions: bank.questions.map((q) =>
          q.id === question.id ? question : q,
        ),
      }));
      setEditingId(null);
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
          ? { ...bank, questions }
          : { ...bank, questions, editable: false, review_submitted: false };
      });
      setDeleting(null);
      refreshPosting();
      gooeyToast.success(
        "Question deleted",
        withdrawn
          ? {
              description:
                "The posting left the review queue. Add questions and submit it again.",
            }
          : undefined,
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
    const { code, message } = apiError(state.error);
    if (code && staleCodes.has(code)) return null;
    return message ?? fallback;
  };
  const submitError = formError(
    sendForReview,
    "We couldn't submit the posting. Please try again.",
  );
  const submit = async (run: () => Promise<unknown>) => {
    try {
      await run();
      return true;
    } catch {
      return false;
    }
  };

  const back = {
    href: routes.recruiter.jobPosting(postingId),
    label: posting.data?.title ?? "Job posting",
  };

  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      {query.isPending ? (
        <>
          <PageHeader back={back} title="Question bank" />
          <div role="status" className="space-y-3">
            <Skeleton className="h-12 rounded-md" />
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
            <span className="sr-only">Loading the question bank…</span>
          </div>
        </>
      ) : denied ? (
        <>
          <PageHeader back={back} title="Question bank" />
          <div role="alert" className="rounded-xl border p-6">
            <h2 className="text-base font-semibold">
              You don&apos;t have access to this question bank
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Checking your account access… Only the recruiter who owns this job
              posting can manage its questions.
            </p>
          </div>
        </>
      ) : query.isError ? (
        <>
          <PageHeader back={back} title="Question bank" />
          <div role="alert" className="rounded-xl border p-6">
            <h2 className="text-base font-semibold">
              {apiError(query.error).status === 404
                ? "This job posting doesn't exist"
                : "We couldn't load the question bank"}
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {apiError(query.error).status === 404
                ? "It may have been removed, or it belongs to another recruiter."
                : "Please try again in a moment."}
            </p>
            {apiError(query.error).status !== 404 && (
              <Button
                className={cn("mt-4", pillClass)}
                variant="outline"
                disabled={query.isFetching}
                onClick={() => void query.refetch()}
              >
                Try again
              </Button>
            )}
          </div>
        </>
      ) : (
        <>
          <PageHeader
            back={back}
            title="Question bank"
            meta={
              <>
                <PostingStatus
                  status={query.data.posting_status}
                  questionBankReady={query.data.questions.length > 0}
                  reviewSubmitted={query.data.review_submitted}
                />
                <span>Candidates never see these questions.</span>
              </>
            }
            description={bankState(query.data)}
            actions={
              preparing(query.data) && !query.data.review_submitted ? (
                <Button
                  className={pillClass}
                  disabled={
                    query.data.questions.length === 0 ||
                    editingId !== null ||
                    addDirty ||
                    add.isPending ||
                    update.isPending ||
                    remove.isPending ||
                    generate.isPending ||
                    sendForReview.isPending
                  }
                  aria-busy={sendForReview.isPending}
                  onClick={() => sendForReview.mutate(query.data)}
                >
                  {sendForReview.isPending && (
                    <LoaderCircle
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden
                    />
                  )}
                  {sendForReview.isPending
                    ? "Submitting…"
                    : query.data.posting_status === "rejected"
                      ? "Resubmit for approval"
                      : "Submit for approval"}
                </Button>
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
                  In the review queue
                </span>
              ) : null
            }
          />

          {submitError && (
            <p role="alert" className="text-sm text-destructive">
              {submitError}
            </p>
          )}

          <PostingProgress
            posting={{
              status: query.data.posting_status,
              review_submitted: query.data.review_submitted,
              core_question_count: query.data.questions.length,
            }}
            generating={generate.isPending}
          />

          {notice && (
            <Notice role="alert" tone="danger">
              <p>{notice}</p>
            </Notice>
          )}

          <section aria-labelledby="questions-heading" className="space-y-4">
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h2 id="questions-heading" className="text-base font-semibold">
                Questions
              </h2>
              <span className="text-sm text-muted-foreground tabular-nums">
                {query.data.questions.length}{" "}
                {query.data.questions.length === 1 ? "question" : "questions"}
              </span>
            </div>
            {query.data.questions.length === 0 ? (
              <EmptyBank
                bank={query.data}
                generating={generate.isPending}
                error={formError(
                  generate,
                  "We couldn't generate questions. Please try again.",
                )}
                onGenerate={() => generate.mutate()}
              />
            ) : (
              <ol className="divide-y rounded-xl border">
                {query.data.questions.map((question, index) => (
                  <li
                    key={question.id}
                    className={cn(
                      "px-4 py-4 sm:px-5",
                      editingId === question.id && "bg-muted/40",
                    )}
                  >
                    {editingId === question.id ? (
                      <QuestionEditor
                        id={`edit-${question.id}`}
                        label={`Edit question ${index + 1}`}
                        initial={question.content}
                        maxLength={query.data.max_question_length}
                        pending={update.isPending}
                        error={formError(
                          update,
                          "We couldn't save the question. Your edit is still here.",
                        )}
                        submitLabel="Save question"
                        pendingLabel="Saving…"
                        onCancel={() => {
                          update.reset();
                          setEditingId(null);
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
                              className="size-10 rounded-full text-muted-foreground hover:text-foreground"
                              aria-label={`Edit question ${index + 1}`}
                              disabled={editingId !== null}
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
                              className="size-10 rounded-full text-muted-foreground hover:text-destructive"
                              aria-label={`Delete question ${index + 1}`}
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
                  </li>
                ))}
              </ol>
            )}

            {query.data.editable && (
              <div className="rounded-xl bg-muted/60 p-4 sm:p-5">
                <QuestionEditor
                  onDirtyChange={setAddDirty}
                  id="new-question"
                  label="Add a question"
                  maxLength={query.data.max_question_length}
                  pending={add.isPending}
                  error={formError(
                    add,
                    "We couldn't add the question. Your draft is still here.",
                  )}
                  submitLabel="Add question"
                  pendingLabel="Adding…"
                  onSubmit={(content) => submit(() => add.mutateAsync(content))}
                />
              </div>
            )}
          </section>

          <DeleteQuestionDialog
            question={deleting}
            pending={remove.isPending}
            error={formError(
              remove,
              "We couldn't delete the question. Please try again.",
            )}
            onCancel={() => setDeleting(null)}
            onConfirm={() => deleting && remove.mutate(deleting.id)}
          />
        </>
      )}
    </section>
  );
}

// The sentence under the title: what the bank's state lets the Recruiter do next.
function bankState(bank: Bank) {
  if (!preparing(bank))
    return `This posting is ${statusText[bank.posting_status].toLowerCase()}, so the question bank is read-only.`;
  if (bank.review_submitted)
    return "Submitted for approval. You can still edit the questions until an administrator reviews the posting.";
  return bank.questions.length > 0
    ? "Review and save your questions, then submit the posting for approval."
    : "The posting needs at least one question before you can submit it for approval.";
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
  if (!preparing(bank))
    return (
      <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
        This posting has no core questions.
      </p>
    );
  if (bank.generation_size === 0)
    return (
      <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
        No questions yet. Automatic generation isn&apos;t available right now;
        please try again later.
      </p>
    );
  return (
    <div className="space-y-4 rounded-xl border border-dashed px-6 py-10 text-center">
      <div className="space-y-1">
        <h3 className="font-semibold">No questions yet</h3>
        <p className="mx-auto max-w-[52ch] text-sm text-muted-foreground">
          Generate {bank.generation_size} questions from the requirements you
          confirmed. You can edit, add and remove questions before submitting.
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
        {generating
          ? "Generating questions…"
          : error
            ? "Retry generating questions"
            : "Generate questions"}
      </Button>
      <p role="status" aria-live="polite" className="text-[13px]">
        {generating ? (
          <span
            className="t-shimmer"
            data-text="This can take up to a minute. You can stay on this page."
          >
            This can take up to a minute. You can stay on this page.
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
