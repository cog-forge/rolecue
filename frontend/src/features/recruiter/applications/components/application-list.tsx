"use client";

import {
  useEffect,
  useState,
  type ComponentProps,
  type FormEvent,
  type MouseEvent,
} from "react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Inbox, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
} from "../api/applications";
import type { Application } from "../schemas/application-schema";
import {
  APPLICATION_PAGE_SIZE,
  SEARCH_MAX,
  applicationSearch,
  parseApplicationFilters,
  type ApplicationFilters,
  type ApplicationStatusFilter,
} from "../utils/application-filters";
import { ApplicationStatus } from "./application-status";

const tabs: { status: ApplicationStatusFilter; label: string }[] = [
  { status: "all", label: "All" },
  { status: "pending", label: "Awaiting review" },
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
const dateFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeZone: "UTC",
});
// The deadline is a moment, so it is shown in the viewer's own time zone, named to avoid misreading.
const deadlineFormat = new Intl.DateTimeFormat("en", {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  timeZoneName: "short",
});

// Filter changes update the URL with the History API instead of a route navigation, so the page
// is not re-rendered by the server and the current rows stay visible while the next page loads.
function pushFilters(href: string) {
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
        pushFilters(href);
      }}
    />
  );
}

function useApplicationFilters() {
  const params = useSearchParams();
  const record: Record<string, string | string[]> = {};
  for (const key of new Set(params.keys())) {
    const values = params.getAll(key);
    record[key] = values.length === 1 ? values[0] : values;
  }
  return parseApplicationFilters(record);
}

// The Recruiter's Applications. With a postingId it lists that posting's; without one it lists
// every Application on the Recruiter's own postings.
export function ApplicationList({ postingId }: { postingId?: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const pathname = usePathname();
  const filters = useApplicationFilters();
  // The single-posting list takes its posting from the path, never from the URL filter.
  const scoped: ApplicationFilters = postingId
    ? { ...filters, posting: null }
    : filters;
  const href = (next: ApplicationFilters) =>
    `${pathname}${applicationSearch(next)}`;

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
    // Keep showing the previous rows while another tab or page loads.
    placeholderData: keepPreviousData,
  });
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
    total === 0 ? 0 : (scoped.page - 1) * APPLICATION_PAGE_SIZE + 1;
  const lastRow = Math.min(scoped.page * APPLICATION_PAGE_SIZE, total);
  const filtered = scoped.status !== "all" || scoped.search !== "";
  const showPosting = !postingId;
  const reset: ApplicationFilters = {
    status: "all",
    search: "",
    page: 1,
    // "All applications" means all of them, so the one-posting narrowing goes too.
    posting: null,
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <nav aria-label="Filter applications by status">
          <ul className="inline-flex flex-wrap gap-1 rounded-xl border bg-muted/70 p-1">
            {tabs.map((tab) => {
              const active = scoped.status === tab.status;
              return (
                <li key={tab.status}>
                  <FilterLink
                    href={href({ ...scoped, status: tab.status, page: 1 })}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "inline-flex min-h-11 items-center rounded-md px-3 text-sm font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
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
        <SearchForm
          key={scoped.search}
          filters={scoped}
          href={href}
          pathname={pathname}
        />
      </div>

      {showPosting && scoped.posting && (
        <Notice tone="info">
          <p className="flex flex-wrap items-center gap-x-3 gap-y-1">
            Showing the applications of one job posting.
            <FilterLink
              href={href({ ...scoped, posting: null, page: 1 })}
              className="rounded-sm font-medium underline underline-offset-4 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              Show all my postings
            </FilterLink>
          </p>
        </Notice>
      )}

      {query.isPending ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-14 rounded-md" />
          ))}
          <span className="sr-only">Loading applications…</span>
        </div>
      ) : denied ? (
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
      ) : notFound ? (
        <Notice
          role="alert"
          tone="locked"
          title="We couldn't find this job posting"
        >
          <p className="text-muted-foreground">
            It may have been removed, or it belongs to another recruiter.
          </p>
        </Notice>
      ) : query.isError ? (
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
      ) : page && page.items.length === 0 && total > 0 ? (
        <div className="rounded-xl border p-8 text-center">
          <h3 className="font-semibold">
            This page is past the end of the list.
          </h3>
          <Button asChild variant="outline" className="mt-4 h-11">
            <FilterLink href={href({ ...scoped, page: 1 })}>
              Go to the first page
            </FilterLink>
          </Button>
        </div>
      ) : page && page.items.length === 0 ? (
        <div className="rounded-2xl border border-dashed bg-muted/30 p-10 text-center duration-300 animate-in fade-in zoom-in-95 motion-reduce:animate-none">
          <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-workspace-accent/10 text-workspace-accent">
            <Inbox className="size-6" aria-hidden />
          </span>
          <h3 className="font-semibold">
            {scoped.search
              ? `No applications match “${scoped.search}”.`
              : emptyText[scoped.status]}
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
      ) : page ? (
        <>
          <p role="status" className="sr-only">
            {switching ? "Loading applications…" : ""}
          </p>
          <div
            aria-busy={switching}
            className={cn(
              "overflow-x-auto rounded-2xl border bg-card shadow-sm transition-opacity duration-200 motion-reduce:transition-none",
              switching && "opacity-60",
            )}
          >
            <table className="w-full min-w-[720px] table-fixed text-left text-sm">
              <colgroup>
                <col />
                {showPosting && <col className="w-56" />}
                <col className="w-32" />
                <col className="w-64" />
              </colgroup>
              <caption className="sr-only">
                Applications, {firstRow}–{lastRow} of {total}
              </caption>
              <thead>
                <tr className="border-b text-xs tracking-[0.08em] text-muted-foreground uppercase">
                  <th scope="col" className="py-3 pr-4 pl-5 font-medium">
                    Candidate
                  </th>
                  {showPosting && (
                    <th scope="col" className="py-3 pr-4 font-medium">
                      Job posting
                    </th>
                  )}
                  <th scope="col" className="py-3 pr-4 font-medium">
                    Submitted
                  </th>
                  <th scope="col" className="py-3 pr-5 font-medium">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody>
                {page.items.map((application, index) => (
                  <ApplicationRow
                    key={application.id}
                    application={application}
                    index={index}
                    showPosting={showPosting}
                  />
                ))}
              </tbody>
            </table>
          </div>
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
                href={href({ ...scoped, page: scoped.page - 1 })}
                disabled={scoped.page <= 1}
              />
              <PageLink
                label="Next"
                href={href({ ...scoped, page: scoped.page + 1 })}
                disabled={lastRow >= total}
              />
            </div>
          </nav>
        </>
      ) : null}
    </div>
  );
}

function ApplicationRow({
  application,
  index,
  showPosting,
}: {
  application: Application;
  index: number;
  showPosting: boolean;
}) {
  const { candidate } = application;
  return (
    <tr
      className="border-b align-top transition-colors duration-300 animate-in fade-in slide-in-from-bottom-1 fill-mode-both last:border-b-0 hover:bg-workspace-accent/[0.04] motion-reduce:animate-none"
      // A short stagger so rows settle in order; capped so long pages are not slow.
      style={{ animationDelay: `${Math.min(index, 10) * 30}ms` }}
    >
      <td className="py-3.5 pr-4 pl-5">
        <span className="block font-medium [overflow-wrap:anywhere]">
          {candidate.name}
        </span>
        <span className="block text-xs text-muted-foreground [overflow-wrap:anywhere]">
          {candidate.email}
        </span>
      </td>
      {showPosting && (
        <td className="py-3.5 pr-4">
          <Link
            href={routes.recruiter.jobPosting(application.job_posting_id)}
            className="rounded-sm underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {application.job_posting_title}
          </Link>
        </td>
      )}
      <td className="py-3.5 pr-4 whitespace-nowrap text-muted-foreground tabular-nums">
        <time
          dateTime={application.submitted_at}
          title={application.submitted_at}
        >
          {dateFormat.format(new Date(application.submitted_at))}
        </time>
      </td>
      <td className="py-3.5 pr-5">
        <ApplicationStatus status={application.status} />
        <StatusDetail application={application} />
      </td>
    </tr>
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
      <p className="mt-1.5 text-xs text-muted-foreground">
        {/* Decision 4: the interview must be completed, not just started, before the deadline. */}
        {application.interview_eligible
          ? "Complete interview by "
          : "Deadline passed "}
        <time dateTime={deadline} title={deadline}>
          {deadlineFormat.format(new Date(deadline))}
        </time>
      </p>
    );
  }
  if (status === "rejected" && reason) {
    return (
      <p className="mt-1.5 max-w-xs text-xs text-destructive/90 [overflow-wrap:anywhere]">
        Reason: {reason}
      </p>
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
      <Button variant="outline" className="h-11" disabled>
        {label}
      </Button>
    );
  return (
    <Button asChild variant="outline" className="h-11">
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
  pathname: string;
}) {
  const [value, setValue] = useState(filters.search);
  const tooLong = Array.from(value.trim()).length > SEARCH_MAX;
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (tooLong) return;
    pushFilters(href({ ...filters, search: value.trim(), page: 1 }));
  };
  return (
    <form
      role="search"
      noValidate
      aria-label="Search applications"
      onSubmit={submit}
      className="flex flex-wrap items-end gap-2"
    >
      <div className="space-y-1">
        <label htmlFor="application-search" className="text-xs font-medium">
          Search by name or email
        </label>
        <Input
          id="application-search"
          type="search"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          className="h-11 w-64 max-w-full"
          aria-invalid={tooLong}
          aria-describedby={tooLong ? "application-search-error" : undefined}
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
