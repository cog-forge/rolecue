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
import { BriefcaseBusiness, ChevronRight, Plus, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { cn } from "@/lib/utils";
import { jobPostingKeys, listPostings } from "../api/job-postings";
import type { PostingSummary } from "../schemas/job-posting-schema";
import { isAccessDenied } from "../utils/api-error";
import {
  LIST_PAGE_SIZE,
  SEARCH_MAX,
  listSearch,
  parseListFilters,
  type ListFilters,
  type ListStatus,
} from "../utils/list-filters";
import { headerBandClass, Notice } from "./notice";
import { PostingStatus } from "./posting-status";

const tabs: { status: ListStatus; label: string }[] = [
  { status: "all", label: "All" },
  { status: "pending", label: "Pending" },
  { status: "open", label: "Open" },
  { status: "intake_closed", label: "Intake closed" },
  { status: "closed", label: "Closed" },
  { status: "rejected", label: "Rejected" },
];
const emptyText: Record<ListStatus, string> = {
  all: "You haven't created a job posting yet.",
  pending: "No postings are waiting for review.",
  open: "No postings are open for applications.",
  intake_closed: "No postings have closed intake.",
  closed: "No postings are closed.",
  rejected: "No postings were rejected.",
};
const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});
const listHref = (filters: ListFilters) =>
  `${routes.recruiter.jobPostings}${listSearch(filters)}`;

// Filter changes update the URL with the History API instead of a route navigation, so the page
// is not re-rendered by the server (no loading skeleton, no replayed entrance animations) and the
// current rows stay visible while the next page of results loads.
function pushFilters(filters: ListFilters) {
  window.history.pushState(null, "", listHref(filters));
}

// A real link (open in new tab and copy link still work) whose plain click stays on the page.
function FilterLink({
  filters,
  onClick,
  ...props
}: Omit<ComponentProps<typeof Link>, "href"> & { filters: ListFilters }) {
  return (
    <Link
      {...props}
      href={listHref(filters)}
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

function useListFilters() {
  const params = useSearchParams();
  const record: Record<string, string | string[]> = {};
  for (const key of new Set(params.keys())) {
    const values = params.getAll(key);
    record[key] = values.length === 1 ? values[0] : values;
  }
  return parseListFilters(record);
}

export function JobPostingList() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const filters = useListFilters();
  const query = useQuery({
    queryKey: jobPostingKeys.list(user.id, filters),
    queryFn: ({ signal }) => listPostings(filters, signal),
    refetchOnMount: "always",
    retry: false,
    // Keep showing the previous rows while another tab or page loads.
    placeholderData: keepPreviousData,
  });
  const switching = query.isPlaceholderData;
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const page = query.data;
  const total = page?.pagination.total ?? 0;
  const firstRow = total === 0 ? 0 : (filters.page - 1) * LIST_PAGE_SIZE + 1;
  const lastRow = Math.min(filters.page * LIST_PAGE_SIZE, total);
  const filtered = filters.status !== "all" || filters.search !== "";

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <header
        className={cn(
          headerBandClass,
          "flex flex-wrap items-end justify-between gap-4",
        )}
      >
        <div className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
            Recruitment
          </p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            My Job Postings
          </h1>
          <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
            Each posting is reviewed by an administrator before candidates can
            see it.
          </p>
        </div>
        <Button
          asChild
          className="h-11 shadow-sm transition-transform hover:-translate-y-0.5 motion-reduce:transition-none motion-reduce:hover:translate-y-0"
        >
          <Link href={routes.recruiter.jobPostingNew}>
            <Plus aria-hidden />
            Create job posting
          </Link>
        </Button>
      </header>

      <div className="flex flex-wrap items-end justify-between gap-4">
        <nav aria-label="Filter by status">
          <ul className="inline-flex flex-wrap gap-1 rounded-xl border bg-muted/70 p-1">
            {tabs.map((tab) => {
              const active = filters.status === tab.status;
              return (
                <li key={tab.status}>
                  <FilterLink
                    filters={{ ...filters, status: tab.status, page: 1 }}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "inline-flex min-h-11 items-center rounded-md px-4 text-sm font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
                      active &&
                        "bg-background text-workspace-accent shadow-sm ring-1 ring-workspace-accent/20",
                    )}
                  >
                    {tab.label}
                  </FilterLink>
                </li>
              );
            })}
          </ul>
        </nav>
        <SearchForm key={filters.search} filters={filters} />
      </div>

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-14 rounded-md" />
          ))}
          <span className="sr-only">Loading your job postings…</span>
        </div>
      ) : denied ? (
        <Notice
          role="alert"
          tone="locked"
          title="You don't have access to job postings"
        >
          <p className="text-muted-foreground">
            Checking your account access… If this stays, sign in again with a
            recruiter account.
          </p>
        </Notice>
      ) : query.isError ? (
        <Notice
          role="alert"
          tone="danger"
          title="We couldn't load your job postings"
        >
          <p className="text-muted-foreground">Please try again in a moment.</p>
          <Button
            className="mt-2 h-11 bg-background"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </Notice>
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
            {filters.search
              ? `No postings match “${filters.search}”.`
              : emptyText[filters.status]}
          </h2>
          {filtered ? (
            <Button asChild variant="outline" className="mt-4 h-11">
              <FilterLink filters={{ status: "all", search: "", page: 1 }}>
                Show all postings
              </FilterLink>
            </Button>
          ) : (
            <>
              <p className="mt-2 text-sm text-muted-foreground">
                Paste a job description and RoleCue suggests the requirements to
                interview for.
              </p>
              <Button asChild className="mt-4 h-11">
                <Link href={routes.recruiter.jobPostingNew}>
                  Create job posting
                </Link>
              </Button>
            </>
          )}
        </div>
      ) : page ? (
        <>
          <p role="status" className="sr-only">
            {switching ? "Loading job postings…" : ""}
          </p>
          <div
            aria-busy={switching}
            className={cn(
              "overflow-x-auto rounded-2xl border bg-card shadow-sm transition-opacity duration-200 motion-reduce:transition-none",
              switching && "opacity-60",
            )}
          >
            {/* Fixed column widths keep every status tab laid out the same, whatever the content. */}
            <table className="w-full min-w-[760px] table-fixed text-left text-sm">
              <colgroup>
                <col />
                <col className="w-56" />
                <col className="w-32" />
                <col className="w-52" />
              </colgroup>
              <caption className="sr-only">
                Your job postings, {firstRow}–{lastRow} of {total}
              </caption>
              <thead>
                <tr className="border-b text-xs tracking-[0.08em] text-muted-foreground uppercase">
                  <th scope="col" className="py-3 pr-4 pl-5 font-medium">
                    Job posting
                  </th>
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Status
                  </th>
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Created
                  </th>
                  <th scope="col" className="py-3 pr-5 text-right font-medium">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {page.items.map((posting, index) => (
                  <PostingRow
                    key={posting.id}
                    posting={posting}
                    index={index}
                  />
                ))}
              </tbody>
            </table>
          </div>
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
                filters={{ ...filters, page: filters.page - 1 }}
                disabled={filters.page <= 1}
              />
              <PageLink
                label="Next"
                filters={{ ...filters, page: filters.page + 1 }}
                disabled={lastRow >= total}
              />
            </div>
          </nav>
        </>
      ) : null}
    </section>
  );
}

function PostingRow({
  posting,
  index,
}: {
  posting: PostingSummary;
  index: number;
}) {
  const details = [posting.location, posting.employment_type]
    .filter(Boolean)
    .join(" · ");
  const editable =
    posting.status === "pending" || posting.status === "rejected";
  return (
    <tr
      className="border-b align-top transition-colors duration-300 animate-in fade-in slide-in-from-bottom-1 fill-mode-both last:border-b-0 hover:bg-workspace-accent/[0.04] motion-reduce:animate-none"
      // A short stagger so rows settle in order; capped so long pages are not slow.
      style={{ animationDelay: `${Math.min(index, 10) * 30}ms` }}
    >
      <td className="py-3.5 pr-4 pl-5">
        <Link
          href={routes.recruiter.jobPosting(posting.id)}
          className="rounded-sm font-medium underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {posting.title}
        </Link>
        {details && (
          <span className="block text-xs text-muted-foreground [overflow-wrap:anywhere]">
            {details}
          </span>
        )}
      </td>
      <td className="py-3.5 pr-4">
        <PostingStatus status={posting.status} />
        {posting.status === "rejected" && posting.reject_reason && (
          <p className="mt-1.5 max-w-xs text-xs text-destructive/90 [overflow-wrap:anywhere]">
            Reason: {posting.reject_reason}
          </p>
        )}
      </td>
      <td className="py-3.5 pr-4 whitespace-nowrap text-muted-foreground tabular-nums">
        <time dateTime={posting.created_at} title={posting.created_at}>
          {dateFormat.format(new Date(posting.created_at))}
        </time>
      </td>
      <td className="py-3.5 pr-5 text-right">
        <div className="flex justify-end gap-2">
          <Button asChild variant="ghost" size="sm" className="h-9">
            <Link
              href={routes.recruiter.jobPostingQuestions(posting.id)}
              aria-label={`Question bank for ${posting.title}`}
            >
              Questions
            </Link>
          </Button>
          <Button asChild variant="outline" size="sm" className="h-9">
            <Link
              href={routes.recruiter.jobPosting(posting.id)}
              aria-label={`${editable ? "Edit" : "View"} ${posting.title}`}
            >
              {editable ? "Edit" : "View"}
              <ChevronRight aria-hidden />
            </Link>
          </Button>
        </div>
      </td>
    </tr>
  );
}

function PageLink({
  label,
  filters,
  disabled,
}: {
  label: string;
  filters: ListFilters;
  disabled: boolean;
}) {
  if (disabled)
    return (
      <Button variant="outline" className="h-11" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-11">
      <FilterLink filters={filters}>{label}</FilterLink>
    </Button>
  );
}

function SearchForm({ filters }: { filters: ListFilters }) {
  const [value, setValue] = useState(filters.search);
  const tooLong = Array.from(value.trim()).length > SEARCH_MAX;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (tooLong) return;
    pushFilters({ ...filters, search: value.trim(), page: 1 });
  };
  return (
    <form
      role="search"
      noValidate
      aria-label="Search your job postings"
      onSubmit={submit}
      className="flex flex-wrap items-end gap-2"
    >
      <div className="space-y-1">
        <label htmlFor="posting-search" className="text-xs font-medium">
          Search by title
        </label>
        <Input
          id="posting-search"
          type="search"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          className="h-11 w-64 max-w-full"
          aria-invalid={tooLong}
          aria-describedby={tooLong ? "posting-search-error" : undefined}
        />
      </div>
      <Button
        type="submit"
        variant="outline"
        className="h-11"
        disabled={tooLong}
      >
        <Search aria-hidden />
        Search
      </Button>
      {filters.search && (
        <Button asChild variant="ghost" className="h-11">
          <FilterLink filters={{ ...filters, search: "", page: 1 }}>
            <X aria-hidden />
            Clear search
          </FilterLink>
        </Button>
      )}
      {tooLong && (
        <p
          id="posting-search-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          Keep the search to {SEARCH_MAX} characters or fewer.
        </p>
      )}
    </form>
  );
}
