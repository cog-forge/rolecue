"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { Coins, LoaderCircle, Plus, RotateCcw } from "lucide-react";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
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
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import {
  avatarKeys,
  avatarPageSize,
  deleteAvatar,
  getInventory,
  inventoryFailure,
  isAccessDenied,
  listAvatars,
} from "../api/inventory";
import type { Avatar, Capacity } from "../schemas/avatar-schema";
import { AvatarPreviewDialog } from "./avatar-preview-dialog";
import {
  AddSlotsCard,
  AvatarSlotCard,
  EmptySlotCard,
} from "./avatar-slot-card";
import { BuySlotsDialog } from "./buy-slots-dialog";

// Page numbers and the open preview live in the URL so reload and Back keep them.
function pageFrom(value: string | null) {
  const page = Number(value);
  return Number.isSafeInteger(page) && page > 0 ? page : 1;
}

export function AvatarInventory() {
  const user = useSessionUser();
  if (user.role === "admin") return null; // RoleGate redirects Admin first.
  return (
    <PersonalInventory
      key={`${user.id}:${user.role}`}
      userId={user.id}
      role={user.role}
    />
  );
}

function PersonalInventory({
  userId,
  role,
}: {
  userId: string;
  role: "candidate" | "recruiter";
}) {
  const cache = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const page = pageFrom(params.get("page"));
  const previewId = params.get("avatar");
  const [buying, setBuying] = useState(false);
  const [deleting, setDeleting] = useState<Avatar | null>(null);
  const deleteTrigger = useRef<HTMLButtonElement | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);

  const summary = useQuery({
    queryKey: avatarKeys.summary(userId),
    queryFn: ({ signal }) => getInventory(signal),
    refetchOnMount: "always",
    retry: false,
  });
  // The grid pages through slots: avatars fill the first slots, empty slots follow.
  const list = useQuery({
    queryKey: avatarKeys.list(userId, page),
    queryFn: ({ signal }) => listAvatars(page, signal),
    refetchOnMount: "always",
    retry: false,
    placeholderData: keepPreviousData,
  });
  const denied = isAccessDenied(summary.error) || isAccessDenied(list.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  function setParams(changes: Record<string, string | null>) {
    const next = new URLSearchParams(params);
    for (const [key, value] of Object.entries(changes))
      if (value === null) next.delete(key);
      else next.set(key, value);
    const query = next.toString();
    router.push(query ? `${pathname}?${query}` : pathname, { scroll: false });
  }

  const remove = useMutation({
    mutationFn: (avatar: Avatar) => deleteAvatar(avatar.id),
    onSuccess: (_, avatar) => finishDelete(avatar, "Avatar deleted"),
    onError: (error, avatar) => {
      const failure = inventoryFailure(error, "delete");
      if (failure.denied)
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      if (failure.confirmed && failure.code === "AVATAR_NOT_FOUND")
        finishDelete(avatar, "This avatar was already deleted");
      // A dropped reply may follow a committed delete; refresh so the list shows the truth.
      if (!failure.confirmed)
        void cache.invalidateQueries({ queryKey: avatarKeys.all(userId) });
    },
  });

  function finishDelete(avatar: Avatar, title: string) {
    deleteTrigger.current = null;
    setDeleting(null);
    cache.removeQueries({ queryKey: avatarKeys.detail(userId, avatar.id) });
    // Free the deleted avatar's rendered portrait as well as its cache entry.
    const portraitKey = ["avatar-portrait", userId, avatar.id];
    for (const [, url] of cache.getQueriesData<string>({
      queryKey: portraitKey,
    }))
      if (url) URL.revokeObjectURL(url);
    cache.removeQueries({ queryKey: portraitKey });
    void cache.invalidateQueries({ queryKey: avatarKeys.all(userId) });
    if (previewId === avatar.id) setParams({ avatar: null });
    gooeyToast.success(title, {
      description: "One slot is free in your inventory.",
    });
  }

  const deleteFailure = remove.error
    ? inventoryFailure(remove.error, "delete")
    : null;
  const deleteMessage = !deleteFailure
    ? null
    : !deleteFailure.confirmed
      ? "We couldn't confirm the deletion. Your list was refreshed; if the avatar is still listed, try again."
      : deleteFailure.code === "AVATAR_IN_USE"
        ? "This avatar is used by a job posting or an interview, so it can't be deleted."
        : deleteFailure.denied
          ? "Your session can't delete avatars right now. Sign in again and retry."
          : "We couldn't delete this avatar. Try again.";

  const capacity = summary.data;
  const full = capacity?.is_full ?? false;
  const canCreate = !!capacity && !full && capacity.available > 0;
  const slotPages = capacity
    ? Math.max(1, Math.ceil(capacity.capacity / avatarPageSize))
    : 1;
  const firstSlot = (page - 1) * avatarPageSize;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-8">
      <header className="flex flex-wrap items-end justify-between gap-5">
        <div className="space-y-3">
          <p className="text-xs font-semibold tracking-widest text-workspace-accent uppercase">
            Your interviewers, your way
          </p>
          <h1
            ref={heading}
            tabIndex={-1}
            className="text-3xl font-semibold tracking-tight text-balance outline-none sm:text-4xl"
          >
            Personal Avatar Studio
          </h1>
          <p className="max-w-xl text-sm leading-relaxed text-muted-foreground">
            Create 3D avatars from your photos, give each one a personality, and
            keep them in your private inventory.
          </p>
        </div>
        {full ? (
          <Button className="h-11" onClick={() => setBuying(true)}>
            <Coins aria-hidden /> Buy slots
          </Button>
        ) : canCreate ? (
          <Button asChild className="h-11">
            <Link href={routes.avatarStudioNew}>
              <Plus aria-hidden /> Create avatar
            </Link>
          </Button>
        ) : (
          <Button className="h-11" disabled aria-describedby="create-reason">
            <Plus aria-hidden /> Create avatar
          </Button>
        )}
      </header>
      {!canCreate && !full && (
        <p id="create-reason" className="sr-only">
          {summary.isPending
            ? "Checking your avatar slots."
            : "Your avatar slots couldn't be checked."}
        </p>
      )}

      <CapacityPanel
        capacity={capacity}
        loading={summary.isPending}
        failed={summary.isError}
        fetching={summary.isFetching}
        onRefresh={() => void summary.refetch()}
        onBuy={() => setBuying(true)}
      />

      {list.isPending || summary.isPending ? (
        <div
          role="status"
          className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4"
        >
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="aspect-[4/5] rounded-2xl" />
          ))}
          <span className="sr-only">Loading your avatars…</span>
        </div>
      ) : list.isError || !capacity ? (
        <div
          role="alert"
          className="space-y-4 rounded-2xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <p className="text-sm">
            {denied
              ? "Checking your account access… If this stays, sign in again."
              : "We couldn't load your avatars."}
          </p>
          <Button
            variant="outline"
            className="h-11"
            onClick={() => {
              void list.refetch();
              void summary.refetch();
            }}
          >
            Try again
          </Button>
        </div>
      ) : page > slotPages ? (
        <div className="space-y-4 rounded-2xl border p-6">
          <p className="text-sm">
            This page is past the end of your inventory.
          </p>
          <Button asChild variant="outline" className="h-11">
            <Link href={routes.avatarStudio}>Go to the first page</Link>
          </Button>
        </div>
      ) : (
        <div className="space-y-6">
          <ul
            key={page}
            aria-label={`Avatar slots, ${capacity.used} of ${capacity.capacity} used`}
            className={cn(
              "grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4",
              list.isPlaceholderData && "opacity-60",
            )}
          >
            {list.data.items.map((avatar, index) => (
              <AvatarSlotCard
                key={avatar.id}
                avatar={avatar}
                userId={userId}
                index={index}
                onOpen={() => setParams({ avatar: avatar.id })}
                onDelete={(trigger) => {
                  deleteTrigger.current = trigger;
                  remove.reset();
                  setDeleting(avatar);
                }}
              />
            ))}
            {Array.from(
              {
                length: Math.max(
                  Math.min(avatarPageSize, capacity.capacity - firstSlot) -
                    list.data.items.length,
                  0,
                ),
              },
              (_, offset) => {
                const slot = firstSlot + list.data.items.length + offset + 1;
                return (
                  <EmptySlotCard
                    key={`empty-${slot}`}
                    slot={slot}
                    capacity={capacity.capacity}
                    index={list.data.items.length + offset}
                    first={slot === capacity.used + 1}
                  />
                );
              },
            )}
            {full && page === slotPages && (
              <AddSlotsCard
                index={list.data.items.length}
                onBuy={() => setBuying(true)}
              />
            )}
          </ul>
          {capacity.capacity > avatarPageSize && (
            <DataTablePagination
              offset={firstSlot}
              pageSize={avatarPageSize}
              total={capacity.capacity}
              disabled={list.isFetching}
              label="Avatar slot pages"
              onPageChange={(next) =>
                setParams({ page: next > 1 ? String(next) : null })
              }
            />
          )}
        </div>
      )}

      <BuySlotsDialog
        open={buying}
        onOpenChange={setBuying}
        capacity={capacity}
        userId={userId}
        role={role}
      />
      <AvatarPreviewDialog
        userId={userId}
        avatarId={previewId}
        onClose={() => setParams({ avatar: null })}
        onDelete={(avatar) => {
          deleteTrigger.current = null;
          remove.reset();
          setDeleting(avatar);
        }}
      />
      <Dialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setDeleting(null);
        }}
      >
        <DialogContent
          className="sm:max-w-md"
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            // Several cards open this dialog; return focus to the opener while it still exists.
            if (deleteTrigger.current?.isConnected)
              deleteTrigger.current.focus();
            else heading.current?.focus();
          }}
        >
          <DialogHeader>
            <DialogTitle className="[overflow-wrap:anywhere]">
              Delete {deleting?.name}?
            </DialogTitle>
            <DialogDescription>
              This frees one slot in your inventory. Your capacity stays the
              same. This can&apos;t be undone.
            </DialogDescription>
          </DialogHeader>
          {deleteMessage && (
            <p role="alert" className="text-sm text-destructive">
              {deleteMessage}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              className="h-11"
              disabled={remove.isPending}
              onClick={() => setDeleting(null)}
            >
              Keep avatar
            </Button>
            <Button
              variant="destructive"
              className="h-11"
              disabled={remove.isPending}
              aria-busy={remove.isPending}
              onClick={() => {
                if (deleting) remove.mutate(deleting);
              }}
            >
              {remove.isPending && (
                <LoaderCircle
                  aria-hidden
                  className="size-4 animate-spin motion-reduce:animate-none"
                />
              )}
              {remove.isPending ? "Deleting…" : "Delete avatar"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

// One segment per slot mirrors the grid below; past 24 slots a plain bar reads better.
const segmentLimit = 24;

function CapacityPanel({
  capacity,
  loading,
  failed,
  fetching,
  onRefresh,
  onBuy,
}: {
  capacity: Capacity | undefined;
  loading: boolean;
  failed: boolean;
  fetching: boolean;
  onRefresh: () => void;
  onBuy: () => void;
}) {
  return (
    <div className="flex flex-col gap-5 rounded-2xl border bg-card p-5 sm:flex-row sm:items-center sm:justify-between sm:p-6">
      <div className="min-w-0 flex-1 space-y-3">
        <p className="text-sm text-muted-foreground">Avatar slots</p>
        {loading ? (
          <div role="status" className="space-y-3">
            <Skeleton className="h-8 w-40" />
            <Skeleton className="h-2 w-full max-w-md" />
            <span className="sr-only">Checking your avatar slots…</span>
          </div>
        ) : failed || !capacity ? (
          <p role="alert" className="text-sm text-destructive">
            We couldn&apos;t check your avatar slots.
          </p>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <p className="text-2xl font-semibold tracking-tight tabular-nums sm:text-3xl">
                <PopNumber value={capacity.used} /> of{" "}
                <PopNumber value={capacity.capacity} /> used
              </p>
              {capacity.is_full && (
                <span className="rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-900 dark:bg-amber-400/15 dark:text-amber-200">
                  Full
                </span>
              )}
            </div>
            <SlotMeter capacity={capacity} />
            <p className="text-sm text-muted-foreground">
              {capacity.is_full
                ? "Delete an avatar you no longer need, or buy more slots to create another."
                : `${capacity.available} free ${capacity.available === 1 ? "slot" : "slots"} for new avatars.`}
            </p>
          </>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          className="size-11"
          aria-label="Refresh slots"
          disabled={fetching}
          onClick={onRefresh}
        >
          <RotateCcw
            aria-hidden
            className={cn(
              "size-4",
              fetching && "animate-spin motion-reduce:animate-none",
            )}
          />
        </Button>
        {/* When full, the title row and the grid's last card carry the purchase instead. */}
        {!capacity?.is_full && (
          <Button variant="outline" className="h-11" onClick={onBuy}>
            <Coins aria-hidden /> Buy slots
          </Button>
        )}
      </div>
    </div>
  );
}

function SlotMeter({ capacity }: { capacity: Capacity }) {
  const fill = capacity.is_full ? "bg-amber-500" : "bg-foreground/80";
  if (capacity.capacity > segmentLimit || capacity.capacity === 0) {
    const percent =
      capacity.capacity > 0
        ? Math.min(100, (capacity.used / capacity.capacity) * 100)
        : 0;
    return (
      <div
        aria-hidden
        className="h-2 w-full max-w-md overflow-hidden rounded-full bg-muted"
      >
        <div
          className={cn(
            "h-full rounded-full transition-[width] duration-(--duration-slow) ease-(--ease-smooth-out) motion-reduce:transition-none",
            fill,
          )}
          style={{ width: `${percent}%` }}
        />
      </div>
    );
  }
  return (
    <div aria-hidden className="flex w-full max-w-md gap-1">
      {Array.from({ length: capacity.capacity }, (_, index) => (
        <span
          key={index}
          className={cn(
            "h-2 flex-1 rounded-full transition-colors duration-(--duration-slow) motion-reduce:transition-none",
            index < capacity.used ? fill : "bg-muted",
          )}
        />
      ))}
    </div>
  );
}

// transitions.dev number pop-in: when the value changes, only the digits that changed re-enter.
function PopNumber({ value }: { value: number }) {
  const [shown, setShown] = useState<{
    value: number;
    previous: number | null;
  }>({ value, previous: null });
  if (shown.value !== value) setShown({ value, previous: shown.value });
  const text = String(value);
  const before = shown.previous === null ? null : String(shown.previous);
  let same = 0;
  if (before)
    while (
      same < before.length &&
      same < text.length &&
      before[same] === text[same]
    )
      same++;
  return (
    <span
      key={value}
      className={cn("t-digit-group", before !== null && "is-animating")}
    >
      {text.split("").map((digit, index) => (
        <span
          key={index}
          className="t-digit"
          data-same={index < same ? "" : undefined}
          data-stagger={
            index === text.length - 1 && text.length > 1 ? "1" : undefined
          }
        >
          {digit}
        </span>
      ))}
    </span>
  );
}
