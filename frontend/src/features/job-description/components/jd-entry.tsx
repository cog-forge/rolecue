"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Check, FileText, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  entrySchema,
  normalizeRoleText,
  type EntryValues,
  type RoleProfile,
  type UploadTicket,
} from "../api/contract";
import { roleError, uploadPDF } from "../api/transport";
import type { JDWorkflow } from "../api/workflow";
import { DocumentUploadPanel } from "./document-upload-panel";
import { RoleProfileSteps } from "./role-profile-steps";

const phaseLabels = {
  upload: {
    action: "Uploading PDF…",
    status: "Uploading your PDF securely.",
  },
  reading: {
    action: "Reading PDF…",
    status: "Reading your PDF and saving its original.",
  },
  analysis: {
    action: "Analyzing role…",
    status: "Saving your Role Profile and extracting technical requirements.",
  },
};

export function JDEntry({ workflow }: { workflow: JDWorkflow }) {
  const { api, keys, links, audience } = workflow;
  const recruiter = audience === "recruiter";
  const user = useSessionUser();
  const router = useRouter();
  const cache = useQueryClient();
  const [mode, setMode] = useState<"text" | "pdf">("text");
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState<number | null>(null);
  const [phase, setPhase] = useState<"upload" | "reading" | "analysis" | null>(
    null,
  );
  const abort = useRef<AbortController | null>(null);
  const pdfSubmission = useRef<{
    file: File;
    ticket?: UploadTicket;
    uploaded: boolean;
    id?: string;
  } | null>(null);
  useEffect(() => () => abort.current?.abort(), []);
  const textSubmission = useRef<{ text: string; id: string } | null>(null);
  const [draftId, setDraftId] = useState<string | null>(null);
  const form = useForm<EntryValues>({
    resolver: zodResolver(entrySchema),
    defaultValues: { rawText: "" },
  });
  const rawText = useWatch({ control: form.control, name: "rawText" });
  const mutation = useMutation({
    mutationFn: async (values: EntryValues | File) => {
      const controller = new AbortController();
      abort.current = controller;
      try {
        let draft: RoleProfile;
        if (values instanceof File) {
          if (pdfSubmission.current?.file !== values)
            pdfSubmission.current = {
              file: values,
              uploaded: false,
            };
          const current = pdfSubmission.current;
          if (!current) throw new Error("Submission unavailable");
          if (current.id) draft = await api.get(current.id, controller.signal);
          else {
            if (
              current.ticket &&
              Date.parse(current.ticket.request.ExpiresAt) <= Date.now()
            ) {
              current.ticket = undefined;
              current.uploaded = false;
            }
            setPhase("upload");
            setProgress(0);
            current.ticket ??= await api.createUpload(
              values,
              controller.signal,
            );
            if (!current.uploaded) {
              await uploadPDF(
                current.ticket,
                values,
                controller.signal,
                setProgress,
              );
              current.uploaded = true;
            }
            setPhase("reading");
            setProgress(null);
            draft = await api.completeUpload(
              current.ticket.uploadId,
              controller.signal,
            );
            current.id = draft.id;
          }
        } else {
          const { rawText } = values;
          const text = normalizeRoleText(rawText);
          draft =
            textSubmission.current?.text === text
              ? await api.get(textSubmission.current.id, controller.signal)
              : await api.create(rawText);
          textSubmission.current = { text, id: draft.id };
        }
        setDraftId(draft.id);
        cache.setQueryData(keys.detail(user.id, draft.id), draft);
        setPhase("analysis");
        const reviewed = await api.analyze(draft.id, controller.signal);
        cache.setQueryData(keys.detail(user.id, draft.id), reviewed);
        return reviewed;
      } finally {
        if (abort.current === controller) abort.current = null;
        setPhase(null);
        setProgress(null);
      }
    },
    onSuccess: (profile) => {
      void cache.invalidateQueries({
        queryKey: keys.lists(user.id),
      });
      router.push(links.detail(profile.id));
    },
    onError: () => {
      void cache.invalidateQueries({
        queryKey: keys.lists(user.id),
      });
    },
  });
  const busy = form.formState.isSubmitting || mutation.isPending;
  const phaseLabel = phaseLabels[phase ?? "analysis"];
  return (
    <section className="mx-auto w-full max-w-6xl space-y-8 sm:space-y-10">
      <header className="space-y-4">
        <Link
          className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline-offset-4 hover:underline"
          href={links.list}
        >
          {recruiter ? "My Job Postings" : "Role Profiles"}
        </Link>
        <h1 className="max-w-3xl text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
          {recruiter
            ? "Start with the role you’re hiring for."
            : "Bring the role into the room."}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground sm:text-base">
          Start with a job description. We&apos;ll extract the role context, and
          you stay in control of the review.
        </p>
        <RoleProfileSteps current="input" audience={audience} />
      </header>
      <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_16rem]">
        <form
          onSubmit={(event) => {
            if (mode === "pdf") {
              event.preventDefault();
              if (file) mutation.mutate(file);
            } else
              void form.handleSubmit((values) => mutation.mutate(values))(
                event,
              );
          }}
          className="min-w-0 space-y-6"
        >
          <fieldset
            disabled={busy}
            className="min-w-0 rounded-xl border bg-card p-5 sm:p-8"
          >
            <Tabs
              value={mode}
              onValueChange={(value) => {
                if (value === "text" || value === "pdf") {
                  setMode(value);
                  mutation.reset();
                  setDraftId(null);
                }
              }}
            >
              <TabsList
                className="mb-5 h-12! w-full sm:w-fit"
                aria-label="Job description source"
              >
                <TabsTrigger
                  value="text"
                  disabled={busy}
                  className="min-h-11 px-5"
                >
                  Paste text
                </TabsTrigger>
                <TabsTrigger
                  value="pdf"
                  disabled={busy}
                  className="min-h-11 px-5"
                >
                  Upload PDF
                </TabsTrigger>
              </TabsList>
              <TabsContent value="text">
                <label
                  htmlFor="role-text"
                  className="flex items-center gap-2 text-sm font-medium"
                >
                  <FileText
                    className="size-4 text-workspace-accent"
                    aria-hidden="true"
                  />{" "}
                  Job description text
                </label>
                <Textarea
                  id="role-text"
                  {...form.register("rawText")}
                  rows={12}
                  aria-invalid={Boolean(form.formState.errors.rawText)}
                  aria-describedby="role-text-help role-text-error"
                  placeholder="Paste the role, responsibilities, requirements, technologies, and domain context here."
                  className="mt-4 min-h-64 resize-y bg-background leading-relaxed"
                />
                <div
                  id="role-text-help"
                  className="mt-3 flex flex-wrap justify-between gap-2 text-xs text-muted-foreground"
                >
                  <span>100–20,000 characters after whitespace cleanup.</span>
                  <span>
                    {[
                      ...normalizeRoleText(rawText ?? ""),
                    ].length.toLocaleString()}{" "}
                    / 20,000
                  </span>
                </div>
                <p
                  id="role-text-error"
                  role={form.formState.errors.rawText ? "alert" : undefined}
                  className="mt-2 text-sm text-destructive"
                >
                  {form.formState.errors.rawText?.message}
                </p>
              </TabsContent>
              <TabsContent value="pdf">
                <DocumentUploadPanel
                  file={file}
                  disabled={busy}
                  progress={progress}
                  onChange={(next) => {
                    setFile(next);
                    pdfSubmission.current = null;
                    mutation.reset();
                    setDraftId(null);
                  }}
                />
              </TabsContent>
            </Tabs>
          </fieldset>
          {mutation.error && (
            <div
              role="alert"
              className="rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm"
            >
              <p>{roleError(mutation.error)}</p>
              {draftId && (
                <Link
                  href={links.detail(draftId)}
                  className="mt-2 inline-block font-medium underline underline-offset-4"
                >
                  Open your saved draft
                </Link>
              )}
            </div>
          )}
          <div className="flex flex-wrap items-center gap-4">
            <Button
              type="submit"
              disabled={busy || (mode === "pdf" && !file)}
              className="h-11"
            >
              {busy ? (
                <LoaderCircle
                  className="size-4 motion-safe:animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <ArrowRight className="size-4" aria-hidden="true" />
              )}
              {busy ? phaseLabel.action : "Analyze role"}
            </Button>
            {busy && mode === "pdf" && (
              <Button
                type="button"
                variant="outline"
                className="h-11"
                onClick={() => abort.current?.abort()}
              >
                Cancel
              </Button>
            )}
            <p
              role="status"
              aria-live="polite"
              className="text-xs text-muted-foreground"
            >
              {busy
                ? phaseLabel.status.replace(
                    "Role Profile",
                    recruiter ? "job posting" : "Role Profile",
                  )
                : recruiter
                  ? "Your posting stays unpublished while you prepare it."
                  : "Your Role Profile stays private."}
            </p>
          </div>
        </form>
        <aside className="border-l pl-5 lg:sticky lg:top-6">
          <h2 className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
            What happens next
          </h2>
          <ul className="mt-6 space-y-5 text-sm">
            {[
              "We save your JD text or original PDF privately.",
              "AI extracts requirements and expected proficiency.",
              recruiter
                ? "Confirm requirements, then review your saved core questions."
                : "You review and confirm the role context.",
            ].map((item) => (
              <li key={item} className="flex gap-3">
                <Check
                  className="mt-0.5 size-4 shrink-0 text-workspace-accent"
                  aria-hidden="true"
                />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </aside>
      </div>
    </section>
  );
}
