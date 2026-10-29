"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, DownloadCloud, LoaderCircle, Search } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import {
  activateVoice,
  listVoices,
  removeVoice,
  syncVoices,
  voiceKeys,
} from "../api/voices";
import type { Voice } from "../schemas/voice-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { useDesktopLayout } from "../utils/use-desktop-layout";
import {
  MAX_VOICE_QUERY,
  VOICE_PAGE_SIZE,
  voiceSearch,
  type VoiceFilters,
  type VoiceStatus,
} from "../utils/voice-filters";
import { RemoveVoiceDialog } from "./remove-voice-dialog";
import { SyncResult, syncSummary } from "./sync-result";
import { VoiceCard, VoiceCardSkeleton } from "./voice-card";
import { VoiceDetails } from "./voice-details";

const tabs: { status: VoiceStatus; label: string }[] = [
  { status: "all", label: "All" },
  { status: "active", label: "Active" },
  { status: "inactive", label: "Inactive" },
];
const catalogHref = (filters: VoiceFilters) =>
  `${routes.admin.voices}${voiceSearch(filters)}`;
const gridClass = "grid gap-3 sm:grid-cols-2 xl:grid-cols-3";

export function VoiceCatalog({ filters }: { filters: VoiceFilters }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const desktop = useDesktopLayout();
  const [removing, setRemoving] = useState<Voice | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const query = useQuery({
    queryKey: voiceKeys.list(user.id, filters),
    queryFn: ({ signal }) => listVoices(filters, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const remove = useMutation({
    mutationFn: (id: string) => removeVoice(id),
    retry: false,
    onSuccess: (result) => {
      setRemoving(null);
      setNotice(null);
      void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
      gooeyToast.success(
        result.outcome === "deleted" ? "Voice removed" : "Voice deactivated",
        {
          description:
            result.outcome === "deleted"
              ? `${result.voice.name} is no longer in the catalog.`
              : `${result.voice.name} is now inactive; interviews and postings that use it keep it.`,
        },
      );
    },
    onError: (error) => {
      // The page body shows the access message; a dialog left open would hide it.
      if (isAccessDenied(error)) setRemoving(null);
      if (apiError(error).status === 404) {
        // Someone else removed it first: close the dialog and show the current catalog.
        setRemoving(null);
        setNotice(
          "That voice was already removed. We refreshed the catalog for you.",
        );
        void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
      }
    },
  });
  const activate = useMutation({
    mutationFn: (voice: Voice) => activateVoice(voice.id),
    retry: false,
    onMutate: () => setNotice(null),
    onSuccess: (voice) => {
      void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
      gooeyToast.success("Voice reactivated", {
        description: `${voice.name} is active again and can be chosen.`,
      });
    },
    onError: (error, voice) => {
      if (isAccessDenied(error)) return;
      if (apiError(error).status === 404) {
        setNotice(
          `${voice.name} was removed permanently, so it can't be reactivated. We refreshed the catalog for you.`,
        );
        void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
        return;
      }
      setNotice(
        apiError(error).message ??
          `We couldn't reactivate ${voice.name}. Please try again.`,
      );
    },
  });
  const sync = useMutation({
    mutationFn: syncVoices,
    retry: false,
    onSuccess: (result) => {
      void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
      gooeyToast.success("Voice catalog synced", {
        description: syncSummary(result),
      });
    },
  });
  const denied =
    isAccessDenied(query.error) ||
    isAccessDenied(remove.error) ||
    isAccessDenied(activate.error) ||
    isAccessDenied(sync.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const page = query.data;
  const total = page?.pagination.total ?? 0;
  const firstRow = total === 0 ? 0 : (filters.page - 1) * VOICE_PAGE_SIZE + 1;
  const lastRow = Math.min(filters.page * VOICE_PAGE_SIZE, total);
  // Beside the grid, the first voice is selected until another is picked; on small screens the
  // details only open once a card is tapped.
  const selected =
    page?.items.find((voice) => voice.id === selectedId) ??
    (desktop ? (page?.items[0] ?? null) : null);
  const dialogError =
    removing &&
    remove.isError &&
    !isAccessDenied(remove.error) &&
    apiError(remove.error).status !== 404
      ? (apiError(remove.error).message ??
        "We couldn't remove the voice. Please try again.")
      : null;
  const details = (
    <VoiceDetails
      voice={selected}
      activating={activate.isPending && activate.variables?.id === selected?.id}
      onActivate={(voice) => activate.mutate(voice)}
      onRemove={(voice) => {
        remove.reset();
        setRemoving(voice);
      }}
    />
  );

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <PageHeader
        title="Voice Profiles"
        description="The text-to-speech voices recruiters and candidates can choose for the AI interviewer. Sync adds new voices and deactivates ones the provider dropped."
        actions={
          <Button
            className={pillClass}
            disabled={sync.isPending}
            onClick={() => {
              setNotice(null);
              sync.mutate();
            }}
          >
            <span
              className="t-icon-swap"
              data-state={sync.isPending ? "b" : "a"}
              aria-hidden
            >
              <DownloadCloud className="t-icon size-4" data-icon="a" />
              <LoaderCircle
                className="t-icon size-4 animate-spin"
                data-icon="b"
              />
            </span>
            {sync.isPending ? "Syncing…" : "Sync from provider"}
          </Button>
        }
      />

      {sync.isPending && (
        <p role="status" className="sr-only">
          Syncing voices from the provider…
        </p>
      )}
      {sync.isSuccess && (
        <SyncResult result={sync.data} onDismiss={sync.reset} />
      )}
      {sync.isError && !isAccessDenied(sync.error) && (
        <p
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-destructive/30 p-3 text-sm"
        >
          <CircleAlert
            className="mt-0.5 size-4 shrink-0 text-destructive"
            aria-hidden
          />
          <span>
            {syncErrorMessage(sync.error)} The catalog was not changed.{" "}
            <button
              type="button"
              className="font-medium underline underline-offset-2 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              onClick={() => sync.mutate()}
            >
              Try again
            </button>
          </span>
        </p>
      )}

      <div className="space-y-4">
        <div className="border-b">
          <nav
            aria-label="Filter by status"
            className="-mb-px max-w-full overflow-x-auto"
          >
            <ul className="flex gap-5">
              {tabs.map((tab) => {
                const active = filters.status === tab.status;
                return (
                  <li key={tab.status}>
                    <Link
                      href={catalogHref({
                        ...filters,
                        status: tab.status,
                        page: 1,
                      })}
                      scroll={false}
                      aria-current={active ? "page" : undefined}
                      className={cn(
                        "inline-flex min-h-11 items-center border-b-2 border-transparent text-sm whitespace-nowrap text-muted-foreground transition-colors duration-[var(--duration-quick)] outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
                        active &&
                          "border-foreground font-medium text-foreground",
                      )}
                    >
                      {tab.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </nav>
        </div>
        <div className="flex justify-end">
          <SearchForm key={filters.q} filters={filters} />
        </div>
      </div>

      {notice && (
        <p
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-destructive/30 p-3 text-sm"
        >
          <CircleAlert
            className="mt-0.5 size-4 shrink-0 text-destructive"
            aria-hidden
          />
          {notice}
        </p>
      )}

      {query.isPending ? (
        <div role="status">
          <ul className={gridClass} aria-hidden>
            {Array.from({ length: 6 }, (_, i) => (
              <li key={i}>
                <VoiceCardSkeleton />
              </li>
            ))}
          </ul>
          <span className="sr-only">Loading voice profiles…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">
            You don&apos;t have access to voice profiles
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with an
            administrator account.
          </p>
        </div>
      ) : query.isError ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">
            We couldn&apos;t load the voice profiles
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please try again in a moment.
          </p>
          <Button
            className={cn(pillClass, "mt-4")}
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </div>
      ) : page && page.items.length === 0 && total > 0 ? (
        <div className="rounded-xl border p-8 text-center">
          <h2 className="font-semibold">There are no voices on this page.</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            The list got shorter, or this page number doesn&apos;t exist.
          </p>
          <Button asChild variant="outline" className={cn(pillClass, "mt-4")}>
            <Link href={catalogHref({ ...filters, page: 1 })} scroll={false}>
              Go to the first page
            </Link>
          </Button>
        </div>
      ) : page && page.items.length === 0 ? (
        <div className="rounded-xl border p-8 text-center">
          <h2 className="font-semibold">
            {filters.q
              ? `No voices match “${filters.q}”.`
              : filters.status === "all"
                ? "There are no voices in the catalog yet."
                : `There are no ${filters.status} voices.`}
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Use “Sync from provider” to add voices from the text-to-speech
            provider.
          </p>
        </div>
      ) : page ? (
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
          <div className="space-y-4">
            <VoiceGrid key={`${filters.status}|${filters.q}|${filters.page}`}>
              {page.items.map((voice) => (
                <VoiceCard
                  key={voice.id}
                  voice={voice}
                  selected={voice.id === selected?.id}
                  onSelect={(picked) => {
                    setSelectedId(picked.id);
                    if (!desktop) setSheetOpen(true);
                  }}
                />
              ))}
            </VoiceGrid>
            <nav
              aria-label="Pagination"
              className="flex flex-wrap items-center justify-between gap-3 text-sm"
            >
              <p className="text-muted-foreground tabular-nums">
                Showing {firstRow}–{lastRow} of {total}
              </p>
              <div className="flex gap-2">
                <PageLink
                  label="Previous"
                  href={catalogHref({ ...filters, page: filters.page - 1 })}
                  disabled={filters.page <= 1}
                />
                <PageLink
                  label="Next"
                  href={catalogHref({ ...filters, page: filters.page + 1 })}
                  disabled={lastRow >= total}
                />
              </div>
            </nav>
          </div>
          {desktop && (
            <aside
              aria-label="Voice details"
              className="rounded-xl border bg-card p-5 lg:sticky lg:top-6"
            >
              {details}
            </aside>
          )}
        </div>
      ) : null}

      {!desktop && (
        <Sheet
          open={sheetOpen && selected !== null}
          onOpenChange={setSheetOpen}
        >
          <SheetContent
            side="bottom"
            className="max-h-[85dvh] overflow-y-auto rounded-t-2xl"
          >
            <SheetHeader className="sr-only">
              <SheetTitle>Voice details</SheetTitle>
              <SheetDescription>
                Details and actions for the selected voice.
              </SheetDescription>
            </SheetHeader>
            <div className="p-5 pt-8">{details}</div>
          </SheetContent>
        </Sheet>
      )}

      <RemoveVoiceDialog
        voice={removing}
        pending={remove.isPending}
        error={dialogError}
        onCancel={() => setRemoving(null)}
        onConfirm={() => removing && remove.mutate(removing.id)}
      />
    </section>
  );
}

// The API words 502/503 for an Admin; anything else (network, 500) gets a generic message.
function syncErrorMessage(error: unknown) {
  const { status, message } = apiError(error);
  if ((status === 502 || status === 503) && message) return `${message}.`;
  return "We couldn't sync the voices from the provider.";
}

// A new key (filters or page changed) remounts the grid, so the cards rise in again. Cards that
// arrive later (after a sync) join the grid already shown and don't replay.
function VoiceGrid({ children }: { children: React.ReactNode }) {
  const [shown, setShown] = useState(false);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setShown(true));
    return () => cancelAnimationFrame(frame);
  }, []);
  const items = Array.isArray(children) ? children : [children];
  return (
    <ul
      aria-label="Voices"
      className={cn(gridClass, "t-stagger", shown && "is-shown")}
    >
      {items.map((child, index) => (
        <li
          key={index}
          className="t-stagger-line"
          style={{ transitionDelay: `${Math.min(index, 11) * 40}ms` }}
        >
          {child}
        </li>
      ))}
    </ul>
  );
}

function PageLink({
  label,
  href,
  disabled,
}: {
  label: string;
  href: string;
  disabled: boolean;
}) {
  if (disabled)
    return (
      <Button variant="outline" className="h-10 rounded-full" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-10 rounded-full">
      <Link href={href} scroll={false}>
        {label}
      </Link>
    </Button>
  );
}

function SearchForm({ filters }: { filters: VoiceFilters }) {
  const router = useRouter();
  const [q, setQ] = useState(filters.q);
  const tooLong = Array.from(q.trim()).length > MAX_VOICE_QUERY;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (tooLong) return;
    router.push(catalogHref({ ...filters, q: q.trim(), page: 1 }), {
      scroll: false,
    });
  };
  return (
    <form
      noValidate
      role="search"
      aria-label="Search voices"
      onSubmit={submit}
      className="flex flex-wrap items-center gap-2"
    >
      <div>
        <label htmlFor="voice-search" className="sr-only">
          Search by name
        </label>
        <Input
          id="voice-search"
          type="search"
          placeholder="Search by name"
          value={q}
          onChange={(event) => setQ(event.target.value)}
          className="h-10 w-64 rounded-full"
          aria-invalid={tooLong}
          aria-describedby={tooLong ? "voice-search-error" : undefined}
        />
      </div>
      <Button
        type="submit"
        variant="outline"
        className="h-10 rounded-full"
        disabled={tooLong}
      >
        <Search className="size-4" aria-hidden />
        Search
      </Button>
      {filters.q && (
        <Button asChild variant="ghost" className="h-10 rounded-full">
          <Link
            href={catalogHref({ ...filters, q: "", page: 1 })}
            scroll={false}
          >
            Clear search
          </Link>
        </Button>
      )}
      {tooLong && (
        <p
          id="voice-search-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          Keep the search to {MAX_VOICE_QUERY} characters or fewer.
        </p>
      )}
    </form>
  );
}
