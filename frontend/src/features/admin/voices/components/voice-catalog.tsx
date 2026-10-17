"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, DownloadCloud, RotateCcw, Search, Trash2 } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import { activateVoice, listVoices, removeVoice, voiceKeys } from "../api/voices";
import type { Voice } from "../schemas/voice-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import {
  MAX_VOICE_QUERY,
  VOICE_PAGE_SIZE,
  voiceSearch,
  type VoiceFilters,
  type VoiceStatus,
} from "../utils/voice-filters";
import { RemoveVoiceDialog, usage } from "./remove-voice-dialog";

const tabs: { status: VoiceStatus; label: string }[] = [
  { status: "all", label: "All" },
  { status: "active", label: "Active" },
  { status: "inactive", label: "Inactive" },
];
const catalogHref = (filters: VoiceFilters) => `${routes.admin.voices}${voiceSearch(filters)}`;

export function VoiceCatalog({ filters }: { filters: VoiceFilters }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const [removing, setRemoving] = useState<Voice | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
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
      gooeyToast.success(result.outcome === "deleted" ? "Voice removed" : "Voice deactivated", {
        description:
          result.outcome === "deleted"
            ? `${result.voice.name} is no longer in the catalog.`
            : `${result.voice.name} is now inactive; interviews and postings that use it keep it.`,
      });
    },
    onError: (error) => {
      // The page body shows the access message; a dialog left open would hide it.
      if (isAccessDenied(error)) setRemoving(null);
      if (apiError(error).status === 404) {
        // Someone else removed it first: close the dialog and show the current catalog.
        setRemoving(null);
        setNotice("That voice was already removed. We refreshed the catalog for you.");
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
        setNotice(`${voice.name} was removed permanently, so it can't be reactivated. We refreshed the catalog for you.`);
        void cache.invalidateQueries({ queryKey: voiceKeys.lists(user.id) });
        return;
      }
      setNotice(apiError(error).message ?? `We couldn't reactivate ${voice.name}. Please try again.`);
    },
  });
  const denied =
    isAccessDenied(query.error) || isAccessDenied(remove.error) || isAccessDenied(activate.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const page = query.data;
  const total = page?.pagination.total ?? 0;
  const firstRow = total === 0 ? 0 : (filters.page - 1) * VOICE_PAGE_SIZE + 1;
  const lastRow = Math.min(filters.page * VOICE_PAGE_SIZE, total);
  const dialogError =
    removing && remove.isError && !isAccessDenied(remove.error) && apiError(remove.error).status !== 404
      ? (apiError(remove.error).message ?? "We couldn't remove the voice. Please try again.")
      : null;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
            Governance
          </p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">Voice Profiles</h1>
          <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
            The text-to-speech voices recruiters and candidates can choose for the
            AI interviewer.
          </p>
        </div>
        <div className="space-y-1 text-right">
          <Button variant="outline" className="h-11" disabled aria-describedby="fetch-voices-hint">
            <DownloadCloud className="size-4" aria-hidden />
            Fetch from provider
          </Button>
          <p id="fetch-voices-hint" className="text-xs text-muted-foreground">
            Available once provider sync is connected.
          </p>
        </div>
      </header>

      <div className="flex flex-wrap items-end justify-between gap-4">
        <nav aria-label="Filter by status">
          <ul className="inline-flex flex-wrap gap-1 rounded-lg bg-muted p-1">
            {tabs.map((tab) => {
              const active = filters.status === tab.status;
              return (
                <li key={tab.status}>
                  <Link
                    href={catalogHref({ ...filters, status: tab.status, page: 1 })}
                    scroll={false}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "inline-flex min-h-11 items-center rounded-md px-4 text-sm font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
                      active && "bg-background text-foreground shadow-sm",
                    )}
                  >
                    {tab.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>
        <SearchForm key={filters.q} filters={filters} />
      </div>

      {notice && (
        <p role="alert" className="flex items-start gap-2 rounded-lg border border-destructive/30 p-3 text-sm">
          <CircleAlert className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
          {notice}
        </p>
      )}

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-14 rounded-md" />
          ))}
          <span className="sr-only">Loading voice profiles…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">You don&apos;t have access to voice profiles</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with an
            administrator account.
          </p>
        </div>
      ) : query.isError ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">We couldn&apos;t load the voice profiles</h2>
          <p className="mt-2 text-sm text-muted-foreground">Please try again in a moment.</p>
          <Button
            className="mt-4 h-11"
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
          <Button asChild variant="outline" className="mt-4 h-11">
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
            Voices are added from the text-to-speech provider.
          </p>
        </div>
      ) : page ? (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[640px] text-left text-sm">
              <caption className="sr-only">
                Voice profiles, {firstRow}–{lastRow} of {total}
              </caption>
              <thead>
                <tr className="border-b text-xs tracking-[0.08em] text-muted-foreground uppercase">
                  <th scope="col" className="py-3 pr-4 font-medium">Voice</th>
                  <th scope="col" className="py-3 pr-4 font-medium">Status</th>
                  <th scope="col" className="py-3 pr-4 font-medium">In use</th>
                  <th scope="col" className="py-3 text-right font-medium">
                    <span className="sr-only">Action</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {page.items.map((voice) => (
                  <tr key={voice.id} className="border-b align-top last:border-b-0">
                    <td className="py-3 pr-4 font-medium [overflow-wrap:anywhere]">{voice.name}</td>
                    <td className="py-3 pr-4">
                      <VoiceStatusLabel active={voice.is_active} />
                    </td>
                    <td className="py-3 pr-4 text-muted-foreground">
                      {voice.in_use ? usage(voice) : "Not used"}
                    </td>
                    <td className="py-3 text-right">
                      <div className="inline-flex flex-wrap justify-end gap-2">
                      {!voice.is_active && (
                        <Button
                          variant="outline"
                          className="h-11"
                          aria-label={`Reactivate ${voice.name}`}
                          disabled={activate.isPending && activate.variables?.id === voice.id}
                          onClick={() => activate.mutate(voice)}
                        >
                          <RotateCcw aria-hidden />
                          {activate.isPending && activate.variables?.id === voice.id ? "Reactivating…" : "Reactivate"}
                        </Button>
                      )}
                      {/* An inactive voice in use must stay for its references, so only reactivating is left. */}
                      {(voice.is_active || !voice.in_use) && (
                        <Button
                          variant="outline"
                          className="h-11"
                          aria-label={`${voice.in_use ? "Deactivate" : "Remove"} ${voice.name}`}
                          onClick={() => {
                            remove.reset();
                            setRemoving(voice);
                          }}
                        >
                          <Trash2 aria-hidden />
                          {voice.in_use ? "Deactivate" : "Remove"}
                        </Button>
                      )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <nav aria-label="Pagination" className="flex flex-wrap items-center justify-between gap-3 text-sm">
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
        </>
      ) : null}

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

// Status is always text plus a dot, never colour alone (team design rule).
function VoiceStatusLabel({ active }: { active: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
      <span
        aria-hidden
        className={cn("size-2 shrink-0 rounded-full", active ? "bg-foreground" : "bg-muted-foreground")}
      />
      {active ? "Active" : "Inactive"}
    </span>
  );
}

function PageLink({ label, href, disabled }: { label: string; href: string; disabled: boolean }) {
  if (disabled)
    return (
      <Button variant="outline" className="h-11" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-11">
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
    router.push(catalogHref({ ...filters, q: q.trim(), page: 1 }), { scroll: false });
  };
  return (
    <form noValidate role="search" aria-label="Search voices" onSubmit={submit} className="flex flex-wrap items-end gap-2">
      <div className="space-y-1">
        <label htmlFor="voice-search" className="text-xs font-medium">
          Search by name
        </label>
        <Input
          id="voice-search"
          type="search"
          value={q}
          onChange={(event) => setQ(event.target.value)}
          className="h-11 w-64"
          aria-invalid={tooLong}
          aria-describedby={tooLong ? "voice-search-error" : undefined}
        />
      </div>
      <Button type="submit" variant="outline" className="h-11" disabled={tooLong}>
        <Search className="size-4" aria-hidden />
        Search
      </Button>
      {filters.q && (
        <Button asChild variant="ghost" className="h-11">
          <Link href={catalogHref({ ...filters, q: "", page: 1 })} scroll={false}>
            Clear search
          </Link>
        </Button>
      )}
      {tooLong && (
        <p id="voice-search-error" role="alert" className="w-full text-xs text-destructive">
          Keep the search to {MAX_VOICE_QUERY} characters or fewer.
        </p>
      )}
    </form>
  );
}
