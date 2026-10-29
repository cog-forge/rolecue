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
import { ChevronRight, Plus, Search, X } from "lucide-react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Notice } from "./notice";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { PostingStatus } from "./posting-status";

const statusOptions: { status: ListStatus; label: string }[] = [
  { status: "all", label: "All" },
  { status: "preparing", label: "Preparing" },
  { status: "pending", label: "Pending review" },
  { status: "approved", label: "Needs funding" },
  { status: "open", label: "Live" },
  { status: "intake_closed", label: "Intake closed" },
  { status: "closed", label: "Closed" },
  { status: "rejected", label: "Rejected" },
];
const emptyText: Record<ListStatus, string> = {
  all: "You haven't created a job posting yet.",
  preparing: "No postings are being prepared.",
  approved: "No approved postings are waiting for funding.",
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
      <PageHeader
        title="Job postings"
        description="Prepare your posting, questions and evaluation criteria. After approval, fund interview slots to publish it."
        actions={
          <Button asChild className={pillClass}>
            <Link href={routes.recruiter.jobPostingNew}>
              <Plus aria-hidden />
              Create job posting
            </Link>
          </Button>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <label htmlFor="posting-status" className="text-sm font-medium">
            Status
          </label>
          <Select
            value={filters.status}
            onValueChange={(status) =>
              pushFilters({ ...filters, status: status as ListStatus, page: 1 })
            }
          >
            <SelectTrigger
              id="posting-status"
              className="w-48 rounded-full bg-background shadow-none data-[size=default]:h-9"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start">
              {statusOptions.map((option) => (
                <SelectItem key={option.status} value={option.status}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <SearchForm key={filters.search} filters={filters} />
      </div>

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-16 rounded-lg" />
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
            className={cn("mt-2", pillClass)}
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </Notice>
      ) : page && page.items.length === 0 && total > 0 ? (
        <div className="rounded-xl border border-dashed p-8 text-center">
          <h2 className="font-semibold">
            This page is past the end of the list.
          </h2>
          <Button asChild variant="outline" className={cn("mt-4", pillClass)}>
            <FilterLink filters={{ ...filters, page: 1 }}>
              Go to the first page
            </FilterLink>
          </Button>
        </div>
      ) : page && page.items.length === 0 ? (
        <div className="rounded-xl border border-dashed p-10 text-center">
          <h2 className="font-semibold">
            {filters.search
              ? `No postings match “${filters.search}”.`
              : emptyText[filters.status]}
          </h2>
          {filtered ? (
            <Button asChild variant="outline" className={cn("mt-4", pillClass)}>
              <FilterLink filters={{ status: "all", search: "", page: 1 }}>
                Show all postings
              </FilterLink>
            </Button>
          ) : (
            <>
              <p className="mt-2 text-sm text-muted-foreground">
                Upload a job description and RoleCue suggests the requirements
                to interview for.
              </p>
              <Button asChild className={cn("mt-4", pillClass)}>
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
              "rounded-xl border transition-opacity duration-[var(--duration-fast)] motion-reduce:transition-none",
              switching && "opacity-60",
            )}
          >
            {/* A table from xl up; below that each posting is a two-line row, so the page never
                scrolls sideways. Fixed widths keep every status tab laid out the same. */}
            <table className="block w-full text-left text-sm xl:table xl:table-fixed">
              <colgroup>
                <col />
                <col className="w-56" />
                <col className="w-24" />
                <col className="w-32" />
                <col className="w-48" />
              </colgroup>
              <caption className="sr-only">
                Your job postings, {firstRow}–{lastRow} of {total}
              </caption>
              <thead className="hidden xl:table-header-group">
                <tr className="border-b text-xs text-muted-foreground">
                  <th scope="col" className="h-10 pr-4 pl-5 font-normal">
                    Job posting
                  </th>
                  <th scope="col" className="h-10 pr-4 font-normal">
                    Status
                  </th>
                  <th scope="col" className="h-10 pr-4 text-left font-normal">
                    Questions
                  </th>
                  <th scope="col" className="h-10 pr-4 text-left font-normal">
                    Created
                  </th>
                  <th scope="col" className="h-10 pr-5 text-right font-normal">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="block divide-y xl:table-row-group">
                {page.items.map((posting) => (
                  <PostingRow key={posting.id} posting={posting} />
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

function PostingRow({ posting }: { posting: PostingSummary }) {
  const details = [posting.location, posting.employment_type]
    .filter(Boolean)
    .join(" · ");
  const editable =
    posting.status === "pending" || posting.status === "rejected";
  const count = posting.core_question_count;
  return (
    <tr className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-2 px-4 py-4 align-top xl:align-middle xl:table-row xl:p-0">
      <td className="min-w-0 xl:py-3.5 xl:pr-4 xl:pl-5">
        <Link
          href={routes.recruiter.jobPosting(posting.id)}
          className="rounded-sm font-medium underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {posting.title}
        </Link>
        <span className="block text-[13px] text-muted-foreground [overflow-wrap:anywhere]">
          {details || "No location or type"}
          <span className="xl:hidden">
            {" · "}
            {count} {count === 1 ? "question" : "questions"} · Created{" "}
            {dateFormat.format(new Date(posting.created_at))}
          </span>
        </span>
        {posting.status === "rejected" && posting.reject_reason && (
          <span className="mt-1 block text-[13px] text-destructive [overflow-wrap:anywhere]">
            Reason: {posting.reject_reason}
          </span>
        )}
      </td>
      <td className="xl:py-3.5 xl:pr-4">
        <PostingStatus
          status={posting.status}
          questionBankReady={count > 0}
          reviewSubmitted={posting.review_submitted}
          interviewSlot={posting.interview_slot}
        />
      </td>
      <td className="hidden text-left tabular-nums xl:table-cell xl:py-3.5 xl:pr-4">
        {count > 0 ? count : <span className="text-muted-foreground">0</span>}
      </td>
      <td className="hidden text-left whitespace-nowrap text-muted-foreground tabular-nums xl:table-cell xl:py-3.5 xl:pr-4">
        <time dateTime={posting.created_at} title={posting.created_at}>
          {dateFormat.format(new Date(posting.created_at))}
        </time>
      </td>
      <td className="col-span-2 xl:py-2.5 xl:pr-5">
        <div className="flex gap-2 xl:justify-end">
          {!(posting.status === "open" && posting.interview_slot === null) && (
            <Button asChild variant="ghost" className="h-9 rounded-full px-3">
              <Link
                href={routes.recruiter.jobPostingQuestions(posting.id)}
                aria-label={`Question bank for ${posting.title}`}
              >
                Questions
              </Link>
            </Button>
          )}
          <Button asChild variant="outline" className="h-9 rounded-full px-3">
            <Link
              href={`${routes.recruiter.jobPosting(posting.id)}${posting.status === "open" && posting.interview_slot === null ? "#interview-capacity" : ""}`}
              aria-label={`${posting.status === "open" && posting.interview_slot === null ? "Fund & publish" : editable ? "Edit" : "View"} ${posting.title}`}
            >
              {posting.status === "open" && posting.interview_slot === null
                ? "Fund & publish"
                : editable
                  ? "Edit"
                  : "View"}
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
      <Button variant="outline" className={pillClass} disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className={pillClass}>
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
      className="flex flex-wrap items-center gap-2"
    >
      <label htmlFor="posting-search" className="sr-only">
        Search by title
      </label>
      <div className="relative">
        <Search
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden
        />
        <Input
          id="posting-search"
          type="search"
          placeholder="Search by title"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          className="h-9 w-60 max-w-full rounded-full pl-9"
          aria-invalid={tooLong}
          aria-describedby={tooLong ? "posting-search-error" : undefined}
        />
      </div>
      <Button
        type="submit"
        variant="outline"
        className="h-9 rounded-full px-4"
        disabled={tooLong}
      >
        Search
      </Button>
      {filters.search && (
        <Button asChild variant="ghost" className="h-9 rounded-full px-3">
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
