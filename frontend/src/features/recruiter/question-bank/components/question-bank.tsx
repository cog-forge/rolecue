"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleAlert, LoaderCircle, Lock, Pencil, Sparkles, Trash2 } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  addQuestion,
  deleteQuestion,
  generateBank,
  getBank,
  questionBankKeys,
  updateQuestion,
} from "../api/question-bank";
import type { Bank, CoreQuestion, PostingStatus } from "../schemas/question-bank-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { DeleteQuestionDialog } from "./delete-question-dialog";
import { QuestionEditor } from "./question-editor";

const statusText: Record<PostingStatus, string> = {
  pending: "Pending review",
  open: "Approved",
  intake_closed: "Intake closed",
  closed: "Closed",
  rejected: "Rejected",
};

// Codes after which the cached bank is stale: the posting left "pending", the bank was filled
// elsewhere, or the posting/question is gone. Refetching shows the server's truth.
const staleCodes = new Set([
  "QUESTION_BANK_LOCKED",
  "QUESTION_BANK_EXISTS",
  "JOB_POSTING_NOT_FOUND",
  "CORE_QUESTION_NOT_FOUND",
]);

export function QuestionBank({ postingId }: { postingId: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = questionBankKeys.detail(user.id, postingId);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<CoreQuestion | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getBank(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const patch = (update: (bank: Bank) => Bank) =>
    cache.setQueryData<Bank>(key, (bank) => (bank ? update(bank) : bank));
  const onStale = (error: unknown) => {
    const { code, message } = apiError(error);
    if (!code || !staleCodes.has(code)) return false;
    setNotice(message ?? "This question bank changed. We refreshed it for you.");
    setEditingId(null);
    setDeleting(null);
    void cache.invalidateQueries({ queryKey: key });
    return true;
  };

  const generate = useMutation({
    mutationFn: () => generateBank(postingId),
    retry: false,
    onSuccess: (questions) => {
      patch((bank) => ({ ...bank, questions }));
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
    onSuccess: (question) => patch((bank) => ({ ...bank, questions: [...bank.questions, question] })),
    onError: onStale,
  });
  const update = useMutation({
    mutationFn: (input: { id: string; content: string }) =>
      updateQuestion(postingId, input.id, input.content),
    retry: false,
    onSuccess: (question) => {
      patch((bank) => ({
        ...bank,
        questions: bank.questions.map((q) => (q.id === question.id ? question : q)),
      }));
      setEditingId(null);
    },
    onError: onStale,
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteQuestion(postingId, id),
    retry: false,
    onSuccess: (_, id) => {
      patch((bank) => ({ ...bank, questions: bank.questions.filter((q) => q.id !== id) }));
      setDeleting(null);
      gooeyToast.success("Question deleted");
    },
    onError: onStale,
  });

  const denied = [query, generate, add, update, remove].some((state) =>
    isAccessDenied(state.error),
  );
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  // Mutation errors that the stale handler didn't take over are shown next to the form they came from.
  const formError = (state: { isError: boolean; error: unknown }, fallback: string) => {
    if (!state.isError || isAccessDenied(state.error)) return null;
    const { code, message } = apiError(state.error);
    if (code && staleCodes.has(code)) return null;
    return message ?? fallback;
  };
  const submit = async (run: () => Promise<unknown>) => {
    try {
      await run();
      return true;
    } catch {
      return false;
    }
  };

  return (
    <section className="mx-auto w-full max-w-4xl space-y-6">
      <Link
        href={routes.recruiter.jobPostings}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to my job postings
      </Link>

      {query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-1/2" />
          <Skeleton className="h-64 rounded-xl" />
          <span className="sr-only">Loading the question bank…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">You don&apos;t have access to this question bank</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… Only the recruiter who owns this job
            posting can manage its questions.
          </p>
        </div>
      ) : query.isError ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">
            {apiError(query.error).status === 404
              ? "This job posting doesn't exist"
              : "We couldn't load the question bank"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {apiError(query.error).status === 404
              ? "It may have been removed, or it belongs to another recruiter."
              : "Please try again in a moment."}
          </p>
          {apiError(query.error).status !== 404 && (
            <Button
              className="mt-4 h-11"
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              Try again
            </Button>
          )}
        </div>
      ) : (
        <>
          <header className="space-y-2">
            <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
              Job posting
            </p>
            <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">Question bank</h1>
            <p className="text-sm text-muted-foreground">
              The core questions the AI interviewer asks every applicant to this
              posting. Candidates never see them.
            </p>
          </header>

          {!query.data.editable && (
            <p
              role="status"
              className="flex items-start gap-2 rounded-lg border p-3 text-sm"
            >
              <Lock className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
              <span>
                This posting is{" "}
                <span className="font-medium">
                  {statusText[query.data.posting_status].toLowerCase()}
                </span>
                , so the question bank is read-only. Questions can only change
                while the posting is pending review.
              </span>
            </p>
          )}

          {notice && (
            <p
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <CircleAlert className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
              {notice}
            </p>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="flex flex-wrap items-baseline justify-between gap-2">
                <h2>Core questions</h2>
                <span className="text-sm font-normal text-muted-foreground tabular-nums">
                  {query.data.questions.length}{" "}
                  {query.data.questions.length === 1 ? "question" : "questions"}
                </span>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
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
                <ol className="divide-y rounded-lg border">
                  {query.data.questions.map((question, index) => (
                    <li key={question.id} className="p-4">
                      {editingId === question.id ? (
                        <QuestionEditor
                          id={`edit-${question.id}`}
                          label={`Edit question ${index + 1}`}
                          initial={question.content}
                          maxLength={query.data.max_question_length}
                          pending={update.isPending}
                          error={formError(update, "We couldn't save the question. Your edit is still here.")}
                          submitLabel="Save question"
                          pendingLabel="Saving…"
                          onCancel={() => {
                            update.reset();
                            setEditingId(null);
                          }}
                          onSubmit={(content) =>
                            submit(() => update.mutateAsync({ id: question.id, content }))
                          }
                        />
                      ) : (
                        <div className="flex gap-3">
                          <span className="w-6 shrink-0 text-sm text-muted-foreground tabular-nums">
                            {index + 1}.
                          </span>
                          <p className="min-w-0 flex-1 text-sm whitespace-pre-line [overflow-wrap:anywhere]">
                            {question.content}
                          </p>
                          {query.data.editable && (
                            <div className="flex shrink-0 gap-1">
                              <Button
                                variant="ghost"
                                size="icon"
                                className="size-11"
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
                                className="size-11"
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
                <QuestionEditor
                  id="new-question"
                  label="Add a question"
                  maxLength={query.data.max_question_length}
                  pending={add.isPending}
                  error={formError(add, "We couldn't add the question. Your draft is still here.")}
                  submitLabel="Add question"
                  pendingLabel="Adding…"
                  onSubmit={(content) => submit(() => add.mutateAsync(content))}
                />
              )}
            </CardContent>
          </Card>

          <DeleteQuestionDialog
            question={deleting}
            pending={remove.isPending}
            error={formError(remove, "We couldn't delete the question. Please try again.")}
            onCancel={() => setDeleting(null)}
            onConfirm={() => deleting && remove.mutate(deleting.id)}
          />
        </>
      )}
    </section>
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
  if (!bank.editable)
    return <p className="text-sm text-muted-foreground">This posting has no core questions.</p>;
  if (bank.generation_size === 0)
    return (
      <p className="text-sm text-muted-foreground">
        No questions yet. Automatic generation isn&apos;t available right now,
        so add your questions below.
      </p>
    );
  return (
    <div className="space-y-3 rounded-lg border border-dashed p-6 text-center">
      <p className="text-sm text-muted-foreground">
        No questions yet. Generate {bank.generation_size} questions from the
        confirmed requirements of this posting, or add your own below.
      </p>
      <Button className="h-11" disabled={generating} aria-busy={generating} onClick={onGenerate}>
        {generating ? (
          <LoaderCircle className="size-4 animate-spin motion-reduce:animate-none" aria-hidden />
        ) : (
          <Sparkles className="size-4" aria-hidden />
        )}
        {generating ? "Generating questions…" : "Generate questions"}
      </Button>
      <p role="status" aria-live="polite" className="text-xs text-muted-foreground">
        {generating ? "This can take up to a minute. You can stay on this page." : ""}
      </p>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
