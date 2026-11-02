"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CircleAlert,
  Eye,
  LoaderCircle,
  RotateCcw,
  UserRound,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
} from "@/components/ui/dialog";
import { getAvaturnProjectUrl } from "@/config/env";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import {
  avatarKeys,
  createAvatar,
  getInventory,
  inventoryFailure,
  isAccessDenied,
} from "../api/inventory";
import {
  AvaturnExportError,
  downloadAvaturnExport,
} from "../api/avaturn-export";
import type { Avatar, AvatarMetadata } from "../schemas/avatar-schema";
import { AvatarDetailsForm } from "./avatar-details-form";
import { AvaturnCreator } from "./avaturn-creator";
import { StudioJourney, StudioPhotoGuide } from "./studio-journey";

export function AvatarStudio() {
  const user = useSessionUser();
  if (user.role === "admin") return null; // RoleGate redirects Admin first.
  // Identity changes discard drafts and callbacks, even if the gate stays mounted.
  return (
    <PersonalAvatarStudio key={`${user.id}:${user.role}`} userId={user.id} />
  );
}

type Stage =
  | "ready"
  | "editing"
  | "downloading"
  | "processing"
  | "success"
  | "unknown"
  | "failed";

// Each confirmed failure returns before the avatar row is inserted, so resending is safe.
const saveFailures = {
  VALIDATION_ERROR: { key: "validation", retry: false },
  AVATAR_SOURCE_UNSUPPORTED: { key: "unsupported", retry: false },
  AVATAR_SOURCE_TOO_LARGE: { key: "tooLarge", retry: false },
  INVENTORY_FULL: { key: "inventoryFull", retry: true },
  AVATAR_CONVERTER_UNAVAILABLE: { key: "converterUnavailable", retry: true },
  AVATAR_CONVERSION_TIMEOUT: { key: "conversionTimeout", retry: true },
  AVATAR_CONVERSION_FAILED: { key: "conversionFailed", retry: true },
  AVATAR_PERSISTENCE_FAILED: { key: "persistenceFailed", retry: true },
} as const;

function knownSaveFailure(code: string) {
  return Object.hasOwn(saveFailures, code)
    ? saveFailures[code as keyof typeof saveFailures]
    : undefined;
}

function PersonalAvatarStudio({ userId }: { userId: string }) {
  const t = useTranslations("candidate.avatarStudio");
  const url = getAvaturnProjectUrl();
  const cache = useQueryClient();
  const [stage, setStage] = useState<Stage>("ready");
  const [metadata, setMetadata] = useState<AvatarMetadata | null>(null);
  const [saved, setSaved] = useState<Avatar | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [canRetry, setCanRetry] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const [focusMode, setFocusMode] = useState(false);
  const stageHeading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (["editing", "processing", "success", "unknown"].includes(stage))
      stageHeading.current?.focus({ preventScroll: true });
  }, [stage]);
  const exportLocked = useRef(false);
  // The exported GLB of this attempt, kept in memory only for a user-initiated retry.
  const exportedFile = useRef<Blob | null>(null);
  const controller = useRef<AbortController | null>(null);
  const mounted = useRef(false);
  const capacity = useQuery({
    queryKey: avatarKeys.summary(userId),
    queryFn: ({ signal }) => getInventory(signal),
    retry: false,
    refetchOnMount: "always",
  });
  const save = useMutation({
    mutationFn: ({
      file,
      metadata,
      signal,
    }: {
      file: Blob;
      metadata: AvatarMetadata;
      signal: AbortSignal;
    }) => createAvatar(file, metadata, signal),
    retry: false,
    gcTime: 0,
  });
  const { mutateAsync, reset } = save;
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      controller.current?.abort();
      exportedFile.current = null;
      // Aborting the browser request does not cancel a backend insert.
      reset();
    };
  }, [reset]);
  useEffect(() => {
    if (isAccessDenied(capacity.error))
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [capacity.error, cache]);

  // Leaving now loses the Avaturn draft, or the result of a save that may still finish.
  const guarded = ["editing", "downloading", "processing"].includes(stage);
  useEffect(() => {
    if (!guarded) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [guarded]);

  const submit = useCallback(
    async (file: Blob, details: AvatarMetadata) => {
      const abort = new AbortController();
      controller.current = abort;
      setConfirmClose(false);
      setError(null);
      setCanRetry(false);
      setStage("processing");
      try {
        const avatar = await mutateAsync({
          file,
          metadata: details,
          signal: abort.signal,
        });
        if (!mounted.current) return;
        exportedFile.current = null;
        setSaved(avatar);
        setMetadata(null);
        setStage("success");
      } catch (cause) {
        if (!mounted.current) return;
        const failure = inventoryFailure(cause, "create");
        if (failure.denied)
          void cache.invalidateQueries({ queryKey: ["auth", "me"] });
        const known = failure.confirmed
          ? knownSaveFailure(failure.code)
          : undefined;
        if (!failure.confirmed) {
          // Never resend after an unconfirmed outcome: the first request may have saved it.
          exportedFile.current = null;
          setMetadata(null);
          setStage("unknown");
        } else {
          if (!known?.retry) exportedFile.current = null;
          setCanRetry(!!known?.retry);
          setError(
            failure.denied
              ? t("saveErrors.denied")
              : t(`saveErrors.${known?.key ?? "generic"}`),
          );
          setStage("failed");
        }
      } finally {
        reset(); // Never retain GLB bytes in the mutation cache.
        controller.current = null;
        void cache.invalidateQueries({ queryKey: avatarKeys.all(userId) });
      }
    },
    [mutateAsync, reset, cache, userId, t],
  );

  const receiveExport = useCallback(
    (payload: unknown) => {
      if (exportLocked.current || !metadata || !mounted.current) return;
      exportLocked.current = true;
      const abort = new AbortController();
      controller.current = abort;
      setError(null);
      setStage("downloading");
      void (async () => {
        let file: Blob;
        try {
          file = await downloadAvaturnExport(payload, abort.signal);
        } catch (cause) {
          if (!mounted.current || abort.signal.aborted) return;
          setError(
            cause instanceof AvaturnExportError
              ? t(`exportErrors.${cause.code}`)
              : t("exportErrors.generic"),
          );
          exportLocked.current = false;
          setStage("editing");
          return;
        }
        if (!mounted.current || abort.signal.aborted) return;
        exportedFile.current = file;
        await submit(file, metadata);
      })();
    },
    [metadata, submit, t],
  );

  const canCreate =
    !!url &&
    !!capacity.data &&
    !capacity.error &&
    !capacity.data.is_full &&
    capacity.data.available > 0;
  const creatorOpen = stage === "editing" || stage === "downloading";
  const step =
    stage === "ready" || stage === "failed" ? 0 : creatorOpen ? 1 : 2;
  return (
    <section
      className={cn(
        "mx-auto w-full space-y-6",
        focusMode && creatorOpen ? "max-w-none" : "max-w-6xl",
      )}
    >
      <Button asChild variant="ghost" className="-ml-3 h-11 px-3">
        <Link href={routes.avatarStudio}>
          <ArrowLeft aria-hidden /> {t("header.back")}
        </Link>
      </Button>
      <header className="flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
        <div className="space-y-3">
          <p className="text-xs font-semibold tracking-widest text-workspace-accent uppercase">
            {t("header.eyebrow")}
          </p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            {t("header.title")}
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            {t("header.intro")}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-3 rounded-xl border bg-card p-3 sm:min-w-64">
          <span
            aria-hidden
            className="flex size-10 items-center justify-center rounded-lg bg-muted"
          >
            <UserRound className="size-5 text-muted-foreground" />
          </span>
          <div className="flex-1">
            <p role="status" className="text-xs font-medium">
              {capacity.isPending
                ? t("header.slotsChecking")
                : capacity.error
                  ? t("header.slotsFailed")
                  : capacity.data
                    ? t("header.slotsUsed", {
                        used: capacity.data.used,
                        capacity: capacity.data.capacity,
                      })
                    : t("header.slotsUnavailable")}
            </p>
            {capacity.data && (
              <p className="mt-1 text-xs text-muted-foreground">
                {t("header.slotsAvailable", {
                  available: capacity.data.available,
                })}
              </p>
            )}
          </div>
          <Button
            className="size-11"
            size="icon"
            variant="ghost"
            aria-label={t("header.refresh")}
            title={t("header.refresh")}
            disabled={capacity.isFetching}
            onClick={() => void capacity.refetch()}
          >
            <RotateCcw
              aria-hidden
              className={cn(
                "size-4",
                capacity.isFetching &&
                  "animate-spin motion-reduce:animate-none",
              )}
            />
          </Button>
        </div>
      </header>
      <StudioJourney current={step} complete={stage === "success"} />
      {!url && (
        <p role="alert" className="rounded-xl border bg-muted p-4 text-sm">
          {t("notConfigured")}
        </p>
      )}
      {capacity.data?.is_full &&
        !["processing", "success", "unknown"].includes(stage) && (
          <div
            role="alert"
            className="space-y-1 rounded-xl border bg-muted p-4 text-sm"
          >
            <p>{t("inventoryFull.message")}</p>
            <Link
              href={routes.avatarStudio}
              className="font-medium underline underline-offset-4"
            >
              {t("inventoryFull.manage")}
            </Link>
          </div>
        )}
      {error && (
        <div
          role="alert"
          className="flex flex-col gap-3 rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm leading-relaxed sm:flex-row sm:items-start"
        >
          <CircleAlert
            aria-hidden
            className="mt-0.5 size-5 shrink-0 text-destructive"
          />
          <p className="flex-1">{error}</p>
          {stage === "failed" && canRetry && metadata && (
            <Button
              className="min-h-11 shrink-0 px-4"
              variant="outline"
              onClick={() => {
                const file = exportedFile.current;
                if (file && metadata) void submit(file, metadata);
              }}
            >
              <RotateCcw aria-hidden /> {t("retrySave")}
            </Button>
          )}
        </div>
      )}
      {creatorOpen && url ? (
        <div className="space-y-5 animate-in fade-in slide-in-from-bottom-2 duration-(--duration-slow) motion-reduce:animate-none">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <h2
                ref={stageHeading}
                tabIndex={-1}
                className="text-xl font-semibold break-words tracking-tight outline-none"
              >
                {t("creating.title", { name: metadata?.name ?? "" })}
              </h2>
              <p className="text-sm text-muted-foreground">
                {t("creating.intro")}
              </p>
            </div>
            <Button
              className="min-h-11 px-3"
              variant="outline"
              onClick={() => setConfirmClose(true)}
            >
              <X aria-hidden /> {t("creating.close")}
            </Button>
          </div>
          <div
            className={cn(
              "grid items-start gap-6",
              !focusMode && "xl:grid-cols-[minmax(0,1fr)_17rem]",
            )}
          >
            <div className="min-w-0">
              <AvaturnCreator
                url={url}
                onExport={receiveExport}
                expanded={focusMode}
                onToggleExpanded={() => setFocusMode((value) => !value)}
              />
              {stage === "downloading" && (
                <p
                  role="status"
                  className="mt-4 flex items-center gap-2 text-sm"
                >
                  <LoaderCircle
                    aria-hidden
                    className="size-4 animate-spin motion-reduce:animate-none"
                  />
                  <span
                    className="t-shimmer"
                    data-text={t("creating.reading")}
                    aria-hidden
                  >
                    {t("creating.reading")}
                  </span>
                  <span className="sr-only">{t("creating.reading")}</span>
                </p>
              )}
            </div>
            <aside
              aria-label={t("creating.asideLabel")}
              className={cn(
                "space-y-5 rounded-2xl border bg-card p-5",
                focusMode && "hidden",
              )}
            >
              <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
                {t("creating.asideTitle")}
              </p>
              <dl className="space-y-4">
                <div>
                  <dt className="text-xs text-muted-foreground">
                    {t("creating.name")}
                  </dt>
                  <dd className="mt-1 text-sm font-medium break-words">
                    {metadata?.name}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">
                    {t("creating.personality")}
                  </dt>
                  <dd className="mt-1 text-sm leading-relaxed break-words">
                    {metadata?.personality}
                  </dd>
                </div>
              </dl>
              <div className="space-y-2 border-t pt-5">
                <h3 className="text-sm font-medium">
                  {t("creating.beforeTitle")}
                </h3>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t("creating.beforeBody")}
                </p>
              </div>
              <div className="space-y-2 border-t pt-5">
                <h3 className="text-sm font-medium">
                  {t("creating.finalTitle")}
                </h3>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t("creating.finalBody")}
                </p>
              </div>
            </aside>
          </div>
        </div>
      ) : stage === "processing" ? (
        <div
          role="status"
          className="flex min-h-96 flex-col items-center justify-center rounded-2xl border bg-card px-6 py-12 text-center animate-in fade-in duration-(--duration-slow) motion-reduce:animate-none"
        >
          <span className="mb-6 flex size-20 items-center justify-center rounded-3xl border bg-muted">
            <LoaderCircle
              className="size-8 animate-spin text-rolecue-brand motion-reduce:animate-none"
              aria-hidden
            />
          </span>
          <p className="mb-2 text-xs font-medium tracking-wider text-muted-foreground uppercase">
            {t("processing.step")}
          </p>
          <h2
            ref={stageHeading}
            tabIndex={-1}
            className="t-shimmer text-2xl font-semibold tracking-tight outline-none"
            data-text={t("processing.title")}
            aria-label={t("processing.title")}
          >
            {t("processing.title")}
          </h2>
          <p className="mt-3 max-w-sm text-sm leading-relaxed text-muted-foreground">
            {t("processing.body", {
              name: metadata?.name || t("processing.bodyFallback"),
            })}
          </p>
          <p className="mt-8 max-w-sm text-xs leading-relaxed text-muted-foreground">
            {t("processing.keepOpen")}
          </p>
        </div>
      ) : stage === "success" && saved ? (
        <div className="rounded-2xl border bg-card p-6 sm:p-10 animate-in fade-in slide-in-from-bottom-2 duration-(--duration-slow) motion-reduce:animate-none">
          <div role="status" className="space-y-4">
            <span
              aria-hidden
              className="flex size-14 items-center justify-center rounded-2xl border bg-rolecue-brand/10"
            >
              <Check className="size-6 text-rolecue-brand animate-in zoom-in duration-(--duration-slow) motion-reduce:animate-none" />
            </span>
            <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
              {t("success.eyebrow")}
            </p>
            <h2
              ref={stageHeading}
              tabIndex={-1}
              className="text-2xl font-semibold break-words tracking-tight outline-none"
            >
              {t("success.title", { name: saved.name })}
            </h2>
            <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
              {t("success.body")}
            </p>
            <div className="max-w-xl border-t pt-4">
              <p className="text-xs font-medium text-muted-foreground">
                {t("success.personality")}
              </p>
              <p className="mt-2 text-sm leading-relaxed break-words">
                {saved.personality}
              </p>
            </div>
          </div>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <Button className="min-h-11 px-4" asChild>
              <Link href={routes.avatarStudioPreview(saved.id)}>
                <Eye aria-hidden /> {t("success.preview")}
              </Link>
            </Button>
            <Button className="min-h-11 px-4" variant="outline" asChild>
              <Link href={routes.avatarStudio}>{t("success.back")}</Link>
            </Button>
            <Button
              className="min-h-11 px-4"
              variant="ghost"
              onClick={() => {
                setSaved(null);
                setStage("ready");
              }}
            >
              {t("success.another")}
            </Button>
          </div>
        </div>
      ) : stage === "unknown" ? (
        <div className="space-y-3 rounded-2xl border bg-card p-6 sm:p-8">
          <h2
            ref={stageHeading}
            tabIndex={-1}
            className="text-xl font-semibold outline-none"
          >
            {t("unknown.title")}
          </h2>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            {t("unknown.body")}
          </p>
          <Button className="mt-2 min-h-11 px-4" variant="outline" asChild>
            <Link href={routes.avatarStudio}>
              {t("unknown.check")} <ArrowRight aria-hidden />
            </Link>
          </Button>
        </div>
      ) : (
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
          <div className="rounded-2xl border bg-card p-6 sm:p-8">
            <AvatarDetailsForm
              initialValues={metadata ?? undefined}
              disabled={!canCreate}
              onStart={async (values) => {
                setError(null);
                setCanRetry(false);
                exportedFile.current = null;
                const fresh = await capacity.refetch();
                if (!mounted.current) return;
                if (
                  !fresh.isSuccess ||
                  !fresh.data ||
                  fresh.data.is_full ||
                  fresh.data.available <= 0
                ) {
                  setError(t("slotNotConfirmed"));
                  return;
                }
                exportLocked.current = false;
                setMetadata(values);
                setFocusMode(false);
                setStage("editing");
              }}
            />
          </div>
          <StudioPhotoGuide />
        </div>
      )}
      <Dialog open={confirmClose} onOpenChange={setConfirmClose}>
        <DialogContent>
          <DialogTitle>{t("discard.title")}</DialogTitle>
          <DialogDescription>{t("discard.description")}</DialogDescription>
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setConfirmClose(false)}
            >
              {t("discard.keep")}
            </Button>
            <Button
              className="min-h-11"
              onClick={() => {
                controller.current?.abort();
                controller.current = null;
                exportLocked.current = true;
                exportedFile.current = null;
                setMetadata(null);
                setError(null);
                setStage("ready");
                setConfirmClose(false);
              }}
            >
              {t("discard.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
