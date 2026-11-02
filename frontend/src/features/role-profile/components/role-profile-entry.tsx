"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowRight, Check, FileText, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  createEntrySchema,
  normalizeRoleText,
  type EntryValues,
  type RoleProfile,
  type UploadTicket,
} from "../api/contract";
import { roleProfileKeys } from "../api/keys";
import {
  analyzeRoleProfile,
  completePDFUpload,
  createPDFUpload,
  createRoleProfile,
  getRoleProfile,
  uploadPDF,
} from "../api/requests";
import { DocumentUploadPanel } from "./document-upload-panel";
import { RoleProfileSteps } from "./role-profile-steps";
import { useRoleError } from "../lib/use-role-error";
import { useFormatters } from "@/lib/i18n/use-formatters";

export function RoleProfileEntry() {
  const t = useTranslations("candidate.roleProfiles");
  const roleError = useRoleError();
  const fmt = useFormatters();
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
  const entrySchema = useMemo(
    () => createEntrySchema((key, values) => t(`validation.${key}`, values)),
    [t],
  );
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
          if (current.id)
            draft = await getRoleProfile(current.id, controller.signal);
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
            current.ticket ??= await createPDFUpload(values, controller.signal);
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
            draft = await completePDFUpload(
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
              ? await getRoleProfile(
                  textSubmission.current.id,
                  controller.signal,
                )
              : await createRoleProfile(rawText);
          textSubmission.current = { text, id: draft.id };
        }
        setDraftId(draft.id);
        cache.setQueryData(roleProfileKeys.detail(user.id, draft.id), draft);
        setPhase("analysis");
        const reviewed = await analyzeRoleProfile(draft.id, controller.signal);
        cache.setQueryData(roleProfileKeys.detail(user.id, draft.id), reviewed);
        return reviewed;
      } finally {
        if (abort.current === controller) abort.current = null;
        setPhase(null);
        setProgress(null);
      }
    },
    onSuccess: (profile) => {
      void cache.invalidateQueries({
        queryKey: roleProfileKeys.lists(user.id),
      });
      router.push(routes.roleProfiles.detail(profile.id));
    },
    onError: () => {
      void cache.invalidateQueries({
        queryKey: roleProfileKeys.lists(user.id),
      });
    },
  });
  const busy = form.formState.isSubmitting || mutation.isPending;
  const currentPhase = phase ?? "analysis";
  return (
    <section className="mx-auto w-full max-w-6xl space-y-8 sm:space-y-10">
      <header className="space-y-4">
        <Link
          className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline-offset-4 hover:underline"
          href={routes.roleProfiles.root}
        >
          {t("entry.back")}
        </Link>
        <h1 className="max-w-3xl text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
          {t("entry.heading")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground sm:text-base">
          {t("entry.description")}
        </p>
        <RoleProfileSteps current="input" />
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
                aria-label={t("entry.sourceLabel")}
              >
                <TabsTrigger
                  value="text"
                  disabled={busy}
                  className="min-h-11 px-5"
                >
                  {t("entry.textTab")}
                </TabsTrigger>
                <TabsTrigger
                  value="pdf"
                  disabled={busy}
                  className="min-h-11 px-5"
                >
                  {t("entry.pdfTab")}
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
                  {t("entry.textLabel")}
                </label>
                <Textarea
                  id="role-text"
                  {...form.register("rawText")}
                  rows={12}
                  aria-invalid={Boolean(form.formState.errors.rawText)}
                  aria-describedby="role-text-help role-text-error"
                  placeholder={t("entry.placeholder")}
                  className="mt-4 min-h-64 resize-y bg-background leading-relaxed"
                />
                <div
                  id="role-text-help"
                  className="mt-3 flex flex-wrap justify-between gap-2 text-xs text-muted-foreground"
                >
                  <span>{t("entry.help")}</span>
                  <span>
                    {t("entry.counter", {
                      count: fmt.number(
                        [...normalizeRoleText(rawText ?? "")].length,
                      ),
                    })}
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
                  href={routes.roleProfiles.detail(draftId)}
                  className="mt-2 inline-block font-medium underline underline-offset-4"
                >
                  {t("entry.draftLink")}
                </Link>
              )}
            </div>
          )}
          <div className="flex flex-wrap items-center gap-4">
            <Button
              type="submit"
              disabled={busy || (mode === "pdf" && !file)}
              className="h-auto min-h-11 py-2 whitespace-normal"
            >
              {busy ? (
                <LoaderCircle
                  className="size-4 motion-safe:animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <ArrowRight className="size-4" aria-hidden="true" />
              )}
              {busy
                ? t(`entry.phase.${currentPhase}.action`)
                : t("entry.submit")}
            </Button>
            {busy && mode === "pdf" && (
              <Button
                type="button"
                variant="outline"
                className="h-auto min-h-11 py-2 whitespace-normal"
                onClick={() => abort.current?.abort()}
              >
                {t("entry.cancel")}
              </Button>
            )}
            <p
              role="status"
              aria-live="polite"
              className="text-xs text-muted-foreground"
            >
              {busy
                ? t(`entry.phase.${currentPhase}.status`)
                : t("entry.private")}
            </p>
          </div>
        </form>
        <aside className="border-l pl-5 lg:sticky lg:top-6">
          <h2 className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
            {t("entry.next.title")}
          </h2>
          <ul className="mt-6 space-y-5 text-sm">
            {(["save", "extract", "review"] as const).map((item) => (
              <li key={item} className="flex gap-3">
                <Check
                  className="mt-0.5 size-4 shrink-0 text-workspace-accent"
                  aria-hidden="true"
                />
                <span>{t(`entry.next.${item}`)}</span>
              </li>
            ))}
          </ul>
        </aside>
      </div>
    </section>
  );
}
