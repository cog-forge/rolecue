"use client";

import {
  useEffect,
  useState,
  type ComponentProps,
  type FormEvent,
  type MouseEvent,
} from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  BriefcaseBusiness,
  Building2,
  ChevronRight,
  MapPin,
  Search,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import { getJobBoard, jobBoardKeys } from "../api/job-board";
import type { BoardPosting } from "../schemas/job-board-schema";
import { isAccessDenied } from "../utils/api-error";
import {
  BOARD_PAGE_SIZE,
  FILTER_MAX,
  boardSearch,
  defaultBoardFilters,
  hasTextFilters,
  isFilterTooLong,
  parseBoardFilters,
  type BoardFilters,
} from "../utils/board-filters";
import { fullDate, levelText, postedText, salaryText } from "../utils/format";

const boardHref = (filters: BoardFilters) =>
  `${routes.jobs}${boardSearch(filters)}`;

// The detail page gets the board filters in "back" so its back link returns to the same results.
function postingHref(id: string, filters: BoardFilters) {
  const back = boardSearch(filters).slice(1);
  return `${routes.job(id)}${back ? `?${new URLSearchParams({ back })}` : ""}`;
}

// Filter changes update the URL with the History API instead of a route navigation, so the page
// is not re-rendered by the server and the current results stay visible while the next ones load.
function pushFilters(filters: BoardFilters) {
  window.history.pushState(null, "", boardHref(filters));
}

// A real link (open in new tab and copy link still work) whose plain click stays on the page.
function FilterLink({
  filters,
  onClick,
  ...props
}: Omit<ComponentProps<typeof Link>, "href"> & { filters: BoardFilters }) {
  return (
    <Link
      {...props}
      href={boardHref(filters)}
      scroll={false}
      onClick={(event: MouseEvent<HTMLAnchorElement>) => {
        onClick?.(event);
        if (
          event.defaultPrevented ||
          event.button !== 0 ||
          event.metaKey ||
          event.ctrlKey ||
          event.shiftKey ||
          event.altKey
        )
          return;
        event.preventDefault();
        pushFilters(filters);
      }}
    />
  );
}

function useBoardFilters() {
  const params = useSearchParams();
  const record: Record<string, string | string[]> = {};
  for (const key of new Set(params.keys())) {
    const values = params.getAll(key);
    record[key] = values.length === 1 ? values[0] : values;
  }
  return parseBoardFilters(record);
}

export function JobBoard() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const filters = useBoardFilters();
  const query = useQuery({
    queryKey: jobBoardKeys.list(user.id, filters),
    queryFn: ({ signal }) => getJobBoard(filters, signal),
    refetchOnMount: "always",
    retry: false,
    // Keep showing the previous results while another search or page loads.
    placeholderData: keepPreviousData,
  });
  const switching = query.isPlaceholderData;
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const page = query.data;
  // While the next results load, the counts describe the results still on screen, not the URL.
  const offset = page?.pagination.offset ?? 0;
  const total = page?.pagination.total ?? 0;
  const shownPage = Math.floor(offset / BOARD_PAGE_SIZE) + 1;
  const firstRow = total === 0 ? 0 : offset + 1;
  const lastRow = Math.min(offset + (page?.items.length ?? 0), total);
  const filtered = hasTextFilters(filters);
  // An empty previous result says nothing about the new search, so show loading instead.
  const loading = query.isPending || (switching && page?.items.length === 0);

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <header className="space-y-2 rounded-2xl border border-workspace-accent/15 bg-gradient-to-br from-workspace-accent/10 via-workspace-accent/[0.04] to-transparent p-6 duration-500 animate-in fade-in slide-in-from-bottom-2 motion-reduce:animate-none sm:p-8">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Job Board
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Find your next role
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Every posting here was reviewed by RoleCue and is accepting
          applications now.
        </p>
      </header>

      <FilterForm
        key={`${filters.search}\n${filters.location}\n${filters.type}`}
        filters={filters}
      />

      {loading ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-28 rounded-xl" />
          ))}
          <span className="sr-only">Loading open job postings…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border bg-muted/60 p-6">
          <h2 className="font-semibold">
            You don&apos;t have access to the Job Board
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            candidate account.
          </p>
        </div>
      ) : query.isError ? (
        <div
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-6"
        >
          <h2 className="font-semibold">We couldn&apos;t load the Job Board</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Your search is still here. Please try again in a moment.
          </p>
          <Button
            className="mt-4 h-11 bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </div>
      ) : page && page.items.length === 0 && total > 0 ? (
        <div className="rounded-xl border p-8 text-center">
          <h2 className="font-semibold">
            This page is past the end of the list.
          </h2>
          <Button asChild variant="outline" className="mt-4 h-11">
            <FilterLink filters={{ ...filters, page: 1 }}>
              Go to the first page
            </FilterLink>
          </Button>
        </div>
      ) : page && page.items.length === 0 ? (
        <div className="rounded-2xl border border-dashed bg-muted/30 p-10 text-center duration-300 animate-in fade-in zoom-in-95 motion-reduce:animate-none">
          <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-workspace-accent/10 text-workspace-accent">
            <BriefcaseBusiness className="size-6" aria-hidden />
          </span>
          <h2 className="font-semibold">
            {filtered
              ? "No open postings match your search."
              : "There are no open job postings yet."}
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {filtered
              ? "Try fewer words, or check the employment type is spelled the way the posting uses it."
              : "Check back soon: new postings appear here once they are approved."}
          </p>
          {filtered && (
            <Button asChild variant="outline" className="mt-4 h-11">
              <FilterLink filters={defaultBoardFilters}>
                Clear search
              </FilterLink>
            </Button>
          )}
        </div>
      ) : page ? (
        <>
          <p role="status" className="sr-only">
            {switching ? "Loading job postings…" : ""}
          </p>
          <ul
            aria-label={`Open job postings, ${firstRow}–${lastRow} of ${total}`}
            aria-busy={switching}
            className={cn(
              "space-y-3 transition-opacity duration-200 motion-reduce:transition-none",
              switching && "opacity-60",
            )}
          >
            {page.items.map((posting, index) => (
              <PostingCard
                key={posting.id}
                posting={posting}
                filters={filters}
                index={index}
              />
            ))}
          </ul>
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
                filters={{ ...filters, page: shownPage - 1 }}
                disabled={switching || shownPage <= 1}
              />
              <PageLink
                label="Next"
                filters={{ ...filters, page: shownPage + 1 }}
                disabled={switching || lastRow >= total}
              />
            </div>
          </nav>
        </>
      ) : null}
    </section>
  );
}

function PostingCard({
  posting,
  filters,
  index,
}: {
  posting: BoardPosting;
  filters: BoardFilters;
  index: number;
}) {
  const salary = salaryText(posting);
  const facts = [
    posting.employment_type,
    levelText[posting.seniority_level],
    salary,
  ].filter(Boolean);
  return (
    <li
      className="group relative rounded-xl has-[a:focus-visible]:ring-3 has-[a:focus-visible]:ring-ring/50 border bg-card p-5 shadow-sm transition-colors duration-300 animate-in fade-in slide-in-from-bottom-1 fill-mode-both hover:border-workspace-accent/40 motion-reduce:animate-none"
      // A short stagger so cards settle in order; capped so long pages are not slow.
      style={{ animationDelay: `${Math.min(index, 10) * 30}ms` }}
    >
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <div className="min-w-0 space-y-1">
          <h2 className="text-base font-semibold [overflow-wrap:anywhere]">
            {/* The stretched link makes the whole card clickable with one tab stop. */}
            <Link
              href={postingHref(posting.id, filters)}
              className="outline-none after:absolute after:inset-0 after:rounded-xl group-hover:underline underline-offset-4"
            >
              {posting.title}
            </Link>
          </h2>
          <p className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
            {posting.company_name && (
              <span className="inline-flex items-center gap-1.5 [overflow-wrap:anywhere]">
                <Building2 className="size-3.5 shrink-0" aria-hidden />
                {posting.company_name}
              </span>
            )}
            {posting.location && (
              <span className="inline-flex items-center gap-1.5 [overflow-wrap:anywhere]">
                <MapPin className="size-3.5 shrink-0" aria-hidden />
                {posting.location}
              </span>
            )}
          </p>
        </div>
        <ChevronRight
          className="mt-1 size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none"
          aria-hidden
        />
      </div>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs">
        <ul aria-label="Details" className="flex flex-wrap gap-1.5">
          {facts.map((fact, i) => (
            <li
              key={i}
              className="rounded-full border bg-muted/50 px-2.5 py-1 font-medium [overflow-wrap:anywhere]"
            >
              {fact}
            </li>
          ))}
        </ul>
        <time
          dateTime={posting.created_at}
          title={fullDate(posting.created_at)}
          className="text-muted-foreground"
        >
          {postedText(posting.created_at)}
        </time>
      </div>
    </li>
  );
}

function PageLink({
  label,
  filters,
  disabled,
}: {
  label: string;
  filters: BoardFilters;
  disabled: boolean;
}) {
  // The link stays in place when disabled, so keyboard focus is not lost on the first or last page.
  return (
    <Button
      asChild
      variant="outline"
      className="h-11 aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
    >
      <FilterLink
        filters={filters}
        aria-disabled={disabled || undefined}
        onClick={(event) => {
          if (disabled) event.preventDefault();
        }}
      >
        {label}
      </FilterLink>
    </Button>
  );
}

const fields = [
  { name: "search", id: "board-search", label: "Job title" },
  { name: "location", id: "board-location", label: "Location" },
  {
    name: "type",
    id: "board-type",
    label: "Employment type",
    hint: "Exact match, e.g. Full-time",
  },
] as const;
type FieldName = (typeof fields)[number]["name"];

function FilterForm({ filters }: { filters: BoardFilters }) {
  const [values, setValues] = useState<Record<FieldName, string>>({
    search: filters.search,
    location: filters.location,
    type: filters.type,
  });
  const tooLong = fields.some((field) => isFilterTooLong(values[field.name]));
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (tooLong) return;
    pushFilters({
      search: values.search.trim(),
      location: values.location.trim(),
      type: values.type.trim(),
      page: 1,
    });
  };
  return (
    <form
      role="search"
      noValidate
      aria-label="Search open job postings"
      onSubmit={submit}
      className="grid gap-3 rounded-xl border bg-card p-4 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_1fr_auto] lg:items-start"
    >
      {fields.map((field) => {
        const invalid = isFilterTooLong(values[field.name]);
        const hint = "hint" in field ? `${field.id}-hint` : null;
        const describedBy = [hint, invalid ? `${field.id}-error` : null]
          .filter(Boolean)
          .join(" ");
        return (
          <div key={field.name} className="space-y-1">
            <label htmlFor={field.id} className="text-xs font-medium">
              {field.label}
            </label>
            <Input
              id={field.id}
              type="search"
              value={values[field.name]}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  [field.name]: event.target.value,
                }))
              }
              className="h-11"
              aria-invalid={invalid}
              aria-describedby={describedBy || undefined}
            />
            {"hint" in field && (
              <p
                id={hint ?? undefined}
                className="text-xs text-muted-foreground"
              >
                {field.hint}
              </p>
            )}
            {invalid && (
              <p
                id={`${field.id}-error`}
                role="alert"
                className="text-xs text-destructive"
              >
                Keep this to {FILTER_MAX} characters or fewer.
              </p>
            )}
          </div>
        );
      })}
      <div className="flex gap-2 sm:col-span-2 lg:col-span-1 lg:pt-5">
        <Button type="submit" className="h-11" disabled={tooLong}>
          <Search aria-hidden />
          Search
        </Button>
        {hasTextFilters(filters) && (
          <Button asChild variant="ghost" className="h-11">
            <FilterLink filters={defaultBoardFilters}>
              <X aria-hidden />
              Clear
            </FilterLink>
          </Button>
        )}
      </div>
    </form>
  );
}
