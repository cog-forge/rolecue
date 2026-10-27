"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import { downloadAvaturnExport } from "../api/avaturn-export";
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
const saveFailures: Record<string, { message: string; retry: boolean }> = {
  VALIDATION_ERROR: {
    message:
      "RoleCue couldn't read this avatar export. Export it again in Avaturn.",
    retry: false,
  },
  AVATAR_SOURCE_UNSUPPORTED: {
    message:
      "This avatar can't be used as an interviewer. Choose an avatar with facial animation (T2), then export it again.",
    retry: false,
  },
  AVATAR_SOURCE_TOO_LARGE: {
    message:
      "This avatar is larger than the 50 MB upload limit. Export a lighter version.",
    retry: false,
  },
  INVENTORY_FULL: {
    message:
      "Your inventory filled up before this avatar was saved. Free or buy a slot in your inventory, then try saving again.",
    retry: true,
  },
  AVATAR_CONVERTER_UNAVAILABLE: {
    message:
      "Avatar conversion is busy right now. Nothing was saved; try again in a minute.",
    retry: true,
  },
  AVATAR_CONVERSION_TIMEOUT: {
    message:
      "Converting this avatar took too long. Nothing was saved; try again.",
    retry: true,
  },
  AVATAR_CONVERSION_FAILED: {
    message:
      "We couldn't convert this avatar. Nothing was saved; try again or export it again.",
    retry: true,
  },
  AVATAR_PERSISTENCE_FAILED: {
    message:
      "We couldn't store this avatar. Nothing was saved; try again in a minute.",
    retry: true,
  },
};

function PersonalAvatarStudio({ userId }: { userId: string }) {
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
          ? saveFailures[failure.code]
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
              ? "Your session can't save avatars right now. Sign in again, then create the avatar again."
              : (known?.message ?? "We couldn't save this avatar. Try again."),
          );
          setStage("failed");
        }
      } finally {
        reset(); // Never retain GLB bytes in the mutation cache.
        controller.current = null;
        void cache.invalidateQueries({ queryKey: avatarKeys.all(userId) });
      }
    },
    [mutateAsync, reset, cache, userId],
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
            cause instanceof Error
              ? cause.message
              : "We couldn't read the avatar export. Try exporting again.",
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
    [metadata, submit],
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
          <ArrowLeft aria-hidden /> Personal Avatar Studio
        </Link>
      </Button>
      <header className="flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
        <div className="space-y-3">
          <p className="text-xs font-semibold tracking-widest text-workspace-accent uppercase">
            Your interviewer, your way
          </p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            Create an avatar
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            A familiar face for your next interview. Add a personality, create
            the look in Avaturn, and make it part of your practice.
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
                ? "Checking avatar slots…"
                : capacity.error
                  ? "Couldn't check your avatar slots."
                  : capacity.data
                    ? `${capacity.data.used} of ${capacity.data.capacity} avatar slots used`
                    : "Avatar slots unavailable."}
            </p>
            {capacity.data && (
              <p className="mt-1 text-xs text-muted-foreground">
                {capacity.data.available} available for your next creation
              </p>
            )}
          </div>
          <Button
            className="size-11"
            size="icon"
            variant="ghost"
            aria-label="Refresh slots"
            title="Refresh slots"
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
          The Avaturn project is not configured. Please contact support.
        </p>
      )}
      {capacity.data?.is_full &&
        !["processing", "success", "unknown"].includes(stage) && (
          <div
            role="alert"
            className="space-y-1 rounded-xl border bg-muted p-4 text-sm"
          >
            <p>
              Your avatar inventory is full. Delete an avatar or buy more slots
              before creating another one.
            </p>
            <Link
              href={routes.avatarStudio}
              className="font-medium underline underline-offset-4"
            >
              Manage your inventory
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
              <RotateCcw aria-hidden /> Try saving again
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
                Creating {metadata?.name}
              </h2>
              <p className="text-sm text-muted-foreground">
                Take your photos, customize the look, then choose Next to save.
              </p>
            </div>
            <Button
              className="min-h-11 px-3"
              variant="outline"
              onClick={() => setConfirmClose(true)}
            >
              <X aria-hidden /> Close creator
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
                    data-text="Reading your exported avatar…"
                    aria-hidden
                  >
                    Reading your exported avatar…
                  </span>
                  <span className="sr-only">Reading your exported avatar…</span>
                </p>
              )}
            </div>
            <aside
              aria-label="Your avatar details"
              className={cn(
                "space-y-5 rounded-2xl border bg-card p-5",
                focusMode && "hidden",
              )}
            >
              <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
                Your avatar
              </p>
              <dl className="space-y-4">
                <div>
                  <dt className="text-xs text-muted-foreground">Name</dt>
                  <dd className="mt-1 text-sm font-medium break-words">
                    {metadata?.name}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Personality</dt>
                  <dd className="mt-1 text-sm leading-relaxed break-words">
                    {metadata?.personality}
                  </dd>
                </div>
              </dl>
              <div className="space-y-2 border-t pt-5">
                <h3 className="text-sm font-medium">Before you finish</h3>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  Use an avatar with facial animation support (T2). RoleCue
                  needs it for an expressive interviewer.
                </p>
              </div>
              <div className="space-y-2 border-t pt-5">
                <h3 className="text-sm font-medium">The final step</h3>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  Choose Next inside Avaturn. We’ll handle the save and confirm
                  when your avatar is in your inventory.
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
            Step 03 / Save
          </p>
          <h2
            ref={stageHeading}
            tabIndex={-1}
            className="t-shimmer text-2xl font-semibold tracking-tight outline-none"
            data-text="Processing avatar"
            aria-label="Processing avatar"
          >
            Processing avatar
          </h2>
          <p className="mt-3 max-w-sm text-sm leading-relaxed text-muted-foreground">
            Converting and saving {metadata?.name || "your avatar"}. This can
            take a few minutes.
          </p>
          <p className="mt-8 max-w-sm text-xs leading-relaxed text-muted-foreground">
            Keep this page open to see the result. Saving may continue if you
            leave.
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
              Saved to your personal inventory
            </p>
            <h2
              ref={stageHeading}
              tabIndex={-1}
              className="text-2xl font-semibold break-words tracking-tight outline-none"
            >
              {saved.name} has been saved
            </h2>
            <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
              Your personal avatar is in your inventory. Preview it in 3D or
              create another character.
            </p>
            <div className="max-w-xl border-t pt-4">
              <p className="text-xs font-medium text-muted-foreground">
                Personality
              </p>
              <p className="mt-2 text-sm leading-relaxed break-words">
                {saved.personality}
              </p>
            </div>
          </div>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <Button className="min-h-11 px-4" asChild>
              <Link href={routes.avatarStudioPreview(saved.id)}>
                <Eye aria-hidden /> Preview in 3D
              </Link>
            </Button>
            <Button className="min-h-11 px-4" variant="outline" asChild>
              <Link href={routes.avatarStudio}>Back to inventory</Link>
            </Button>
            <Button
              className="min-h-11 px-4"
              variant="ghost"
              onClick={() => {
                setSaved(null);
                setStage("ready");
              }}
            >
              Create another avatar
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
            Check your inventory before trying again
          </h2>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            We couldn&apos;t confirm whether your avatar was saved. Saving was
            sent to RoleCue and may still finish; closing or reloading this page
            does not cancel it. Your export will not be resent, because sending
            it again could create a duplicate.
          </p>
          <Button className="mt-2 min-h-11 px-4" variant="outline" asChild>
            <Link href={routes.avatarStudio}>
              Check inventory <ArrowRight aria-hidden />
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
                  setError(
                    "An available avatar slot couldn't be confirmed. Refresh slots and try again.",
                  );
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
          <DialogTitle>Discard this avatar draft?</DialogTitle>
          <DialogDescription>
            Your unsaved export will be discarded. No avatar has been sent to
            RoleCue yet.
          </DialogDescription>
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setConfirmClose(false)}
            >
              Keep editing
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
              Discard draft
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
