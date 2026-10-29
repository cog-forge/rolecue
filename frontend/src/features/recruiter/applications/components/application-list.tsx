"use client";

import {
  useEffect,
  useState,
  useSyncExternalStore,
  type ComponentProps,
  type FormEvent,
  type MouseEvent,
} from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { ChevronRight, Inbox, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import { cn } from "@/lib/utils";
import {
  applicationKeys,
  listOwnApplications,
  listPostingApplications,
  listPostingOptions,
} from "../api/applications";
import type { Application } from "../schemas/application-schema";
import {
  APPLICATION_PAGE_SIZE,
  SEARCH_MAX,
  applicationSearch,
  defaultApplicationFilters,
  inboxHref,
  parseApplicationFilters,
  type ApplicationFilters,
  type ApplicationStatusFilter,
} from "../utils/application-filters";
import { ApplicationStatus } from "./application-status";
import { formatDeadline, formatSubmitted } from "../utils/application-dates";

const statusOptions: { status: ApplicationStatusFilter; label: string }[] = [
  { status: "all", label: "All" },
  { status: "pending", label: "Awaiting CV review" },
  { status: "interview_eligible", label: "Interview eligible" },
  { status: "interviewed", label: "Interviewed" },
  { status: "approved", label: "Approved" },
  { status: "rejected", label: "Rejected" },
];
const emptyText: Record<ApplicationStatusFilter, string> = {
  all: "No applications yet.",
  pending: "No applications are waiting for CV review.",
  interview_eligible: "No candidates are cleared to interview.",
  interviewed: "No candidates have finished an interview yet.",
  approved: "You haven't approved anyone yet.",
  rejected: "No applications were rejected.",
};
const ALL_POSTINGS = "all";
// Wide screens review beside the list; narrower ones open the Application's own page.
const SPLIT_VIEW = "(min-width: 1024px)";

export function isSplitView() {
  return (
    typeof window.matchMedia === "function" &&
    window.matchMedia(SPLIT_VIEW).matches
  );
}

// Whether the review panel can sit beside the list; false on the server and in older browsers.
export function useSplitView() {
  return useSyncExternalStore(
    (onChange) => {
      if (typeof window.matchMedia !== "function") return () => {};
      const media = window.matchMedia(SPLIT_VIEW);
      media.addEventListener("change", onChange);
      return () => media.removeEventListener("change", onChange);
    },
    isSplitView,
    () => false,
  );
}

// Filter changes update the URL with the History API instead of a route navigation, so the page
// is not re-rendered by the server and the current rows stay visible while the next page loads.
export function pushUrl(href: string) {
  window.history.pushState(null, "", href);
}

// A real link (open in new tab and copy link still work) whose plain click stays on the page.
function FilterLink({
  href,
  onClick,
  ...props
}: Omit<ComponentProps<typeof Link>, "href"> & { href: string }) {
  return (
    <Link
      {...props}
      href={href}
      scroll={false}
      onClick={(event: MouseEvent<HTMLAnchorElement>) => {
        onClick?.(event);
        if (event.defaultPrevented || modified(event)) return;
        event.preventDefault();
        pushUrl(href);
      }}
    />
  );
}

function modified(event: MouseEvent) {
  return (
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  );
}

// The list's filters and its page of Applications. The review inbox and the list share the query,
// so the panel can pick the next CV from the rows on screen without a second request.
export function useApplicationPage(postingId?: string) {
  const user = useSessionUser();
  const params = useSearchParams();
  const pathname = usePathname();
  const record: Record<string, string | string[]> = {};
  for (const key of new Set(params.keys())) {
    const values = params.getAll(key);
    record[key] = values.length === 1 ? values[0] : values;
  }
  const filters = parseApplicationFilters(record);
  // The single-posting list takes its posting from the path, never from the URL filter.
  const scoped: ApplicationFilters = postingId
    ? { ...filters, posting: null }
    : filters;
  const query = useQuery({
    queryKey: postingId
      ? applicationKeys.posting(user.id, postingId, scoped)
      : applicationKeys.own(user.id, scoped),
    queryFn: ({ signal }) =>
      postingId
        ? listPostingApplications(postingId, scoped, signal)
        : listOwnApplications(scoped, signal),
    refetchOnMount: "always",
    retry: false,
    // Keep showing the previous rows while another filter or page loads.
    placeholderData: keepPreviousData,
  });
  const href = (next: ApplicationFilters) =>
    `${pathname}${applicationSearch(next)}`;
  return { filters: scoped, query, href };
}

// One posting's Applications, used on the posting's page. On wide screens a row opens the review
// inbox narrowed to this posting; on narrow ones it opens the Application's own page.
export function ApplicationList({ postingId }: { postingId: string }) {
  const router = useRouter();
  return (
    <div className="space-y-4">
      <ApplicationToolbar postingId={postingId} />
      <ApplicationResults
        postingId={postingId}
        onOpen={(application) =>
          router.push(
            inboxHref(
              { ...defaultApplicationFilters, posting: postingId },
              application.id,
            ),
          )
        }
      />
    </div>
  );
}

export function ApplicationToolbar({ postingId }: { postingId?: string }) {
  const { filters, href } = useApplicationPage(postingId);
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
      <div className="flex items-center gap-2">
        <label htmlFor="application-status" className="text-sm font-medium">
          Status
        </label>
        <Select
          value={filters.status}
          onValueChange={(status) =>
            pushUrl(
              href({
                ...filters,
                status: status as ApplicationStatusFilter,
                page: 1,
              }),
            )
          }
        >
          <SelectTrigger
            id="application-status"
            className="w-44 rounded-full bg-background shadow-none data-[size=default]:h-9"
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
      {!postingId && <PostingFilter filters={filters} href={href} />}
      <SearchForm key={filters.search} filters={filters} href={href} />
    </div>
  );
}

// Narrows the inbox to one of the Recruiter's postings that can take Applications.
function PostingFilter({
  filters,
  href,
}: {
  filters: ApplicationFilters;
  href: (filters: ApplicationFilters) => string;
}) {
  const user = useSessionUser();
  const options = useQuery({
    queryKey: applicationKeys.postingOptions(user.id),
    queryFn: ({ signal }) => listPostingOptions(signal),
    retry: false,
  });
  const postings = (options.data ?? []).filter(
    (posting) => posting.status !== "pending" && posting.status !== "rejected",
  );
  // A posting from a shared link that is not in the first page of options still shows by name.
  const value = filters.posting ?? ALL_POSTINGS;
  const known =
    value === ALL_POSTINGS || postings.some((posting) => posting.id === value);
  return (
    <div className="flex items-center gap-2">
      <label htmlFor="application-posting" className="text-sm font-medium">
        Job posting
      </label>
      <Select
        value={value}
        onValueChange={(posting) =>
          pushUrl(
            href({
              ...filters,
              posting: posting === ALL_POSTINGS ? null : posting,
              page: 1,
            }),
          )
        }
      >
        <SelectTrigger
          id="application-posting"
          className="w-56 rounded-full bg-background shadow-none data-[size=default]:h-9"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent position="popper" align="start">
          <SelectItem value={ALL_POSTINGS}>All job postings</SelectItem>
          {!known && <SelectItem value={value}>Selected posting</SelectItem>}
          {postings.map((posting) => (
            <SelectItem key={posting.id} value={posting.id}>
              {posting.title}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

// The rows. With onSelect (the review inbox) a wide-screen click selects the row in place; with
// onOpen it hands the row to the caller. Narrow screens and modified clicks follow the link to the
// Application's own page.
export function ApplicationResults({
  postingId,
  selectedId,
  onSelect,
  onOpen,
}: {
  postingId?: string;
  selectedId?: string | null;
  onSelect?: (application: Application) => void;
  onOpen?: (application: Application) => void;
}) {
  const cache = useQueryClient();
  const { filters, query, href } = useApplicationPage(postingId);
  const switching = query.isPlaceholderData;
  const denied = isAccessDenied(query.error);
  const notFound = apiError(query.error).status === 404;
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const page = query.data;
  const total = page?.pagination.total ?? 0;
  const firstRow =
    total === 0 ? 0 : (filters.page - 1) * APPLICATION_PAGE_SIZE + 1;
  const lastRow = Math.min(filters.page * APPLICATION_PAGE_SIZE, total);
  const filtered =
    filters.status !== "all" || filters.search !== "" || !!filters.posting;
  const reset: ApplicationFilters = defaultApplicationFilters;
  const handler = onSelect ?? onOpen;

  if (query.isPending)
    return (
      <div role="status" className="space-y-2">
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className="h-20 rounded-lg" />
        ))}
        <span className="sr-only">Loading applications…</span>
      </div>
    );
  if (denied)
    return (
      <Notice
        role="alert"
        tone="locked"
        title="You don't have access to applications"
      >
        <p className="text-muted-foreground">
          Checking your account access… If this stays, sign in again with a
          recruiter account.
        </p>
      </Notice>
    );
  if (notFound)
    return (
      <Notice
        role="alert"
        tone="locked"
        title="We couldn't find this job posting"
      >
        <p className="text-muted-foreground">
          It may have been removed, or it belongs to another recruiter.
        </p>
      </Notice>
    );
  if (query.isError)
    return (
      <Notice
        role="alert"
        tone="danger"
        title="We couldn't load the applications"
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
    );
  if (!page) return null;
  if (page.items.length === 0 && total > 0)
    return (
      <div className="rounded-xl border p-8 text-center">
        <h3 className="font-semibold">
          This page is past the end of the list.
        </h3>
        <Button asChild variant="outline" className="mt-4 h-11">
          <FilterLink href={href({ ...filters, page: 1 })}>
            Go to the first page
          </FilterLink>
        </Button>
      </div>
    );
  if (page.items.length === 0)
    return (
      <div className="rounded-xl border border-dashed p-10 text-center">
        <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <Inbox className="size-6" aria-hidden />
        </span>
        <h3 className="font-semibold">
          {filters.search
            ? `No applications match “${filters.search}”.`
            : emptyText[filters.status]}
        </h3>
        {filtered ? (
          <Button asChild variant="outline" className="mt-4 h-11">
            <FilterLink href={href(reset)}>Show all applications</FilterLink>
          </Button>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">
            Candidates appear here once they apply with a CV.
          </p>
        )}
      </div>
    );

  return (
    <div className="space-y-3">
      <p role="status" className="sr-only">
        {switching ? "Loading applications…" : ""}
      </p>
      <ul
        aria-label={`Applications, ${firstRow}–${lastRow} of ${total}`}
        aria-busy={switching}
        className={cn(
          "divide-y rounded-xl border bg-card transition-opacity duration-200 motion-reduce:transition-none",
          switching && "opacity-60",
        )}
      >
        {page.items.map((application) => (
          <ApplicationRow
            key={application.id}
            application={application}
            showPosting={!postingId}
            selected={application.id === selectedId}
            onActivate={
              handler
                ? (event) => {
                    if (event.defaultPrevented || modified(event)) return;
                    if (!isSplitView()) return;
                    event.preventDefault();
                    handler(application);
                  }
                : undefined
            }
          />
        ))}
      </ul>
      <nav
        aria-label="Applications pagination"
        className="flex flex-wrap items-center justify-between gap-3 text-sm"
      >
        <p className="text-muted-foreground tabular-nums">
          Showing {firstRow}–{lastRow} of {total}
        </p>
        <div className="flex gap-2">
          <PageLink
            label="Previous"
            href={href({ ...filters, page: filters.page - 1 })}
            disabled={filters.page <= 1}
          />
          <PageLink
            label="Next"
            href={href({ ...filters, page: filters.page + 1 })}
            disabled={lastRow >= total}
          />
        </div>
      </nav>
    </div>
  );
}

export function Initials({ name }: { name: string }) {
  const letters = name
    .split(/\s+/)
    .filter((part) => /\p{L}/u.test(part))
    .slice(0, 2)
    .map((part) => Array.from(part)[0].toUpperCase())
    .join("");
  return (
    <span
      aria-hidden
      className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-foreground/80"
    >
      {letters || "?"}
    </span>
  );
}

function ApplicationRow({
  application,
  showPosting,
  selected,
  onActivate,
}: {
  application: Application;
  showPosting: boolean;
  selected: boolean;
  onActivate?: (event: MouseEvent<HTMLAnchorElement>) => void;
}) {
  const { candidate } = application;
  const pending = application.status === "pending";
  return (
    <li>
      <Link
        href={routes.recruiter.application(
          application.job_posting_id,
          application.id,
        )}
        scroll={false}
        aria-current={selected ? "true" : undefined}
        aria-label={`${pending ? "Review CV of" : "Open application of"} ${candidate.name}`}
        onClick={onActivate}
        className={cn(
          "flex items-start gap-3 px-4 py-3.5 outline-none transition-colors first:rounded-t-xl last:rounded-b-xl hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:ring-inset",
          selected && "bg-muted hover:bg-muted",
        )}
      >
        <Initials name={candidate.name} />
        <span className="min-w-0 flex-1 space-y-1">
          <span className="flex items-start justify-between gap-3">
            <span className="min-w-0 font-medium [overflow-wrap:anywhere]">
              {candidate.name}
            </span>
            <ApplicationStatus status={application.status} />
          </span>
          <span className="block truncate text-xs text-muted-foreground">
            {candidate.email}
          </span>
          <span className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
            <span className="min-w-0 truncate">
              {showPosting && `${application.job_posting_title} · `}
              Applied{" "}
              <time
                dateTime={application.submitted_at}
                title={application.submitted_at}
              >
                {formatSubmitted(application.submitted_at)}
              </time>
            </span>
            {pending ? (
              <span className="inline-flex shrink-0 items-center gap-0.5 font-medium text-foreground">
                Review
                <ChevronRight className="size-3.5" aria-hidden />
              </span>
            ) : (
              <ChevronRight className="size-3.5 shrink-0" aria-hidden />
            )}
          </span>
          <StatusDetail application={application} />
        </span>
      </Link>
    </li>
  );
}

// What the status alone does not say: the interview deadline, or why the CV was rejected.
function StatusDetail({ application }: { application: Application }) {
  const {
    status,
    interview_deadline: deadline,
    reject_reason: reason,
  } = application;
  if (status === "interview_eligible" && deadline) {
    return (
      <span className="block text-xs text-muted-foreground">
        {/* Decision 4: the interview must be completed, not just started, before the deadline. */}
        {application.interview_eligible
          ? "Complete interview by "
          : "Deadline passed "}
        <time dateTime={deadline} title={deadline}>
          {formatDeadline(deadline)}
        </time>
      </span>
    );
  }
  if (status === "rejected" && reason) {
    return (
      <span className="block text-xs text-destructive/90 [overflow-wrap:anywhere]">
        Reason: {reason}
      </span>
    );
  }
  return null;
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
      <Button variant="outline" className="h-9 rounded-full" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-9 rounded-full">
      <FilterLink href={href}>{label}</FilterLink>
    </Button>
  );
}

function SearchForm({
  filters,
  href,
}: {
  filters: ApplicationFilters;
  href: (filters: ApplicationFilters) => string;
}) {
  const [value, setValue] = useState(filters.search);
  const tooLong = Array.from(value.trim()).length > SEARCH_MAX;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (tooLong) return;
    pushUrl(href({ ...filters, search: value.trim(), page: 1 }));
  };
  return (
    <form
      role="search"
      noValidate
      aria-label="Search applications"
      onSubmit={submit}
      className="flex flex-wrap items-center gap-2"
    >
      <label htmlFor="application-search" className="sr-only">
        Search by name or email
      </label>
      <div className="relative">
        <Search
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden
        />
        <Input
          id="application-search"
          type="search"
          placeholder="Search name or email"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          className="h-9 w-60 max-w-full rounded-full pl-9"
          aria-invalid={tooLong}
          aria-describedby={tooLong ? "application-search-error" : undefined}
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
          <FilterLink href={href({ ...filters, search: "", page: 1 })}>
            <X aria-hidden />
            Clear search
          </FilterLink>
        </Button>
      )}
      {tooLong && (
        <p
          id="application-search-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          Keep the search to {SEARCH_MAX} characters or fewer.
        </p>
      )}
    </form>
  );
}
