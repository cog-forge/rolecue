"use client";
import { EmptyState } from "@/components/ui/empty-state";

import { ErrorState } from "@/components/ui/error-state";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
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
import { readApiError } from "@/lib/i18n/api-errors";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { cn } from "@/lib/utils";
import {
  activateVoice,
  listVoices,
  removeVoice,
  syncVoices,
  voiceKeys,
} from "../api/voices";
import type { Voice } from "../schemas/voice-schema";
import { isAccessDenied } from "../utils/api-error";
import { useDesktopLayout } from "../utils/use-desktop-layout";
import {
  MAX_VOICE_QUERY,
  VOICE_PAGE_SIZE,
  voiceSearch,
  voiceStatuses,
  type VoiceFilters,
} from "../utils/voice-filters";
import { RemoveVoiceDialog } from "./remove-voice-dialog";
import { SyncResult } from "./sync-result";
import { VoiceCard, VoiceCardSkeleton } from "./voice-card";
import { VoiceDetails } from "./voice-details";

const catalogHref = (filters: VoiceFilters) =>
  `${routes.admin.voices}${voiceSearch(filters)}`;
const gridClass = "grid gap-3 sm:grid-cols-2 xl:grid-cols-3";

export function VoiceCatalog({ filters }: { filters: VoiceFilters }) {
  const t = useTranslations("admin.voices");
  const tCommon = useTranslations("common");
  const errorMessage = useApiErrorMessage();
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
      const name = result.voice.name;
      gooeyToast.success(
        result.outcome === "deleted"
          ? t("feedback.removedTitle")
          : t("feedback.deactivatedTitle"),
        {
          description:
            result.outcome === "deleted"
              ? t("feedback.removedDescription", { name })
              : t("feedback.deactivatedDescription", { name }),
        },
      );
    },
    onError: (error) => {
      // The page body shows the access message; a dialog left open would hide it.
      if (isAccessDenied(error)) setRemoving(null);
      if (readApiError(error).status === 404) {
        // Someone else removed it first: close the dialog and show the current catalog.
        setRemoving(null);
        setNotice(t("feedback.alreadyRemoved"));
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
      gooeyToast.success(t("feedback.reactivatedTitle"), {
        description: t("feedback.reactivatedDescription", {
          name: voice.name,
        }),
      });
    },
    onError: (error, voice) => {
      if (isAccessDenied(error)) return;
      if (readApiError(error).status === 404) {
        setNotice(t("feedback.reactivateGone", { name: voice.name }));
        void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
        return;
      }
      setNotice(
        errorMessage(
          error,
          t("feedback.reactivateFailed", { name: voice.name }),
        ),
      );
    },
  });
  const sync = useMutation({
    mutationFn: syncVoices,
    retry: false,
    onSuccess: (result) => {
      void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
      gooeyToast.success(t("sync.toastTitle"), {
        description: t("sync.summary", {
          added: result.added,
          updated: result.updated,
          deactivated: result.deactivated,
          unchanged: result.unchanged,
        }),
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
    readApiError(remove.error).status !== 404
      ? errorMessage(remove.error, t("feedback.removeFailed"))
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
        title={t("title")}
        description={t("description")}
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
            {sync.isPending ? t("sync.pending") : t("sync.action")}
          </Button>
        }
      />

      {sync.isPending && (
        <p role="status" className="sr-only">
          {t("sync.progress")}
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
            {t("sync.failedNote", {
              error: errorMessage(sync.error, t("sync.failed")),
            })}{" "}
            <button
              type="button"
              className="font-medium underline underline-offset-2 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              onClick={() => sync.mutate()}
            >
              {tCommon("actions.tryAgain")}
            </button>
          </span>
        </p>
      )}

      <div className="space-y-4">
        <div className="border-b">
          <nav
            aria-label={t("filters.label")}
            className="-mb-px max-w-full overflow-x-auto"
          >
            <ul className="flex gap-5">
              {voiceStatuses.map((status) => {
                const active = filters.status === status;
                return (
                  <li key={status}>
                    <Link
                      href={catalogHref({
                        ...filters,
                        status,
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
                      {status === "all"
                        ? tCommon("labels.all")
                        : t(`status.${status}`)}
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
          <span className="sr-only">{t("list.loading")}</span>
        </div>
      ) : denied ? (
        <ErrorState
          title={t("denied.title")}
          description={t("denied.description")}
        ></ErrorState>
      ) : query.isError ? (
        <ErrorState
          title={t("loadFailed.title")}
          description={tCommon("messages.tryAgainLater")}
          onRetry={() => void query.refetch()}
          retryLabel={tCommon("actions.tryAgain")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : page && page.items.length === 0 && total > 0 ? (
        <EmptyState
          title={t("pageGone.title")}
          description={t("pageGone.description")}
        >
          <Button asChild variant="outline" className={cn(pillClass, "mt-4")}>
            <Link href={catalogHref({ ...filters, page: 1 })} scroll={false}>
              {tCommon("actions.firstPage")}
            </Link>
          </Button>
        </EmptyState>
      ) : page && page.items.length === 0 ? (
        <EmptyState
          title={
            filters.q
              ? t("empty.noMatch", { query: filters.q })
              : filters.status === "all"
                ? t("empty.none")
                : t("empty.byStatus", { status: filters.status })
          }
          description={t("empty.hint", { action: t("sync.action") })}
        ></EmptyState>
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
              aria-label={tCommon("labels.pagination")}
              className="flex flex-wrap items-center justify-between gap-3 text-sm"
            >
              <p className="text-muted-foreground tabular-nums">
                {t("list.showing", {
                  first: firstRow,
                  last: lastRow,
                  total,
                })}
              </p>
              <div className="flex gap-2">
                <PageLink
                  label={tCommon("actions.previous")}
                  href={catalogHref({ ...filters, page: filters.page - 1 })}
                  disabled={filters.page <= 1}
                />
                <PageLink
                  label={tCommon("actions.next")}
                  href={catalogHref({ ...filters, page: filters.page + 1 })}
                  disabled={lastRow >= total}
                />
              </div>
            </nav>
          </div>
          {desktop && (
            <aside
              aria-label={t("details.title")}
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
              <SheetTitle>{t("details.title")}</SheetTitle>
              <SheetDescription>
                {t("details.sheetDescription")}
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

// A new key (filters or page changed) remounts the grid, so the cards rise in again. Cards that
// arrive later (after a sync) join the grid already shown and don't replay.
function VoiceGrid({ children }: { children: React.ReactNode }) {
  const t = useTranslations("admin.voices.list");
  const [shown, setShown] = useState(false);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setShown(true));
    return () => cancelAnimationFrame(frame);
  }, []);
  const items = Array.isArray(children) ? children : [children];
  return (
    <ul
      aria-label={t("label")}
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
  const t = useTranslations("admin.voices.search");
  const tCommon = useTranslations("common");
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
      aria-label={t("form")}
      onSubmit={submit}
      className="flex flex-wrap items-center gap-2"
    >
      <div>
        <label htmlFor="voice-search" className="sr-only">
          {t("label")}
        </label>
        <Input
          id="voice-search"
          type="search"
          placeholder={t("placeholder")}
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
        {tCommon("actions.search")}
      </Button>
      {filters.q && (
        <Button asChild variant="ghost" className="h-10 rounded-full">
          <Link
            href={catalogHref({ ...filters, q: "", page: 1 })}
            scroll={false}
          >
            {tCommon("actions.clearSearch")}
          </Link>
        </Button>
      )}
      {tooLong && (
        <p
          id="voice-search-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          {t("tooLong", { max: MAX_VOICE_QUERY })}
        </p>
      )}
    </form>
  );
}
