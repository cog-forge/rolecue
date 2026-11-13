"use client";
import { EmptyState } from "@/components/ui/empty-state";

import {
  useEffect,
  useState,
  type ComponentProps,
  type FormEvent,
  type MouseEvent,
} from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
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
import { useFormatters } from "@/lib/i18n/use-formatters";

// Filter values; their labels come from the catalog (`list.filter.*` and `status.*`).
const statusOptions: ListStatus[] = [
  "all",
  "preparing",
  "pending",
  "approved",
  "open",
  "intake_closed",
  "closed",
  "rejected",
];
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
  const t = useTranslations("recruiter.postings.list");
  const ts = useTranslations("recruiter.postings.status");
  const tCommon = useTranslations("common");
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
        title={t("title")}
        description={t("description")}
        actions={
          <Button asChild className={pillClass}>
            <Link href={routes.recruiter.jobPostingNew}>
              <Plus aria-hidden />
              {t("create")}
            </Link>
          </Button>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2">
          <label htmlFor="posting-status" className="text-sm font-medium">
            {t("statusLabel")}
          </label>
          <Select
            value={filters.status}
            onValueChange={(status) =>
              pushFilters({ ...filters, status: status as ListStatus, page: 1 })
            }
          >
            <SelectTrigger
              id="posting-status"
              className="w-56 max-w-full rounded-full bg-background shadow-none data-[size=default]:h-9"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="start">
              {statusOptions.map((status) => (
                <SelectItem key={status} value={status}>
                  {status === "all"
                    ? tCommon("labels.all")
                    : status === "approved"
                      ? t("filter.needsFunding")
                      : ts(status)}
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
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <Notice role="alert" tone="locked" title={t("denied.title")}>
          <p className="text-muted-foreground">{t("denied.description")}</p>
        </Notice>
      ) : query.isError ? (
        <Notice role="alert" tone="danger" title={t("error.title")}>
          <p className="text-muted-foreground">
            {tCommon("messages.tryAgainLater")}
          </p>
          <Button
            className={cn("mt-2", pillClass)}
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            {tCommon("actions.tryAgain")}
          </Button>
        </Notice>
      ) : page && page.items.length === 0 && total > 0 ? (
        <EmptyState title={t("pastEnd.title")} description="">
          <Button asChild variant="outline" className={cn("mt-4", pillClass)}>
            <FilterLink filters={{ ...filters, page: 1 }}>
              {tCommon("actions.firstPage")}
            </FilterLink>
          </Button>
        </EmptyState>
      ) : page && page.items.length === 0 ? (
        <EmptyState
          title={
            filters.search
              ? t("empty.noMatch", { query: filters.search })
              : t(`empty.${filters.status}`)
          }
          description=""
        >
          {filtered ? (
            <Button asChild variant="outline" className={cn("mt-4", pillClass)}>
              <FilterLink filters={{ status: "all", search: "", page: 1 }}>
                {t("empty.showAll")}
              </FilterLink>
            </Button>
          ) : (
            <>
              <p className="mt-2 text-sm text-muted-foreground">
                {t("empty.hint")}
              </p>
              <Button asChild className={cn("mt-4", pillClass)}>
                <Link href={routes.recruiter.jobPostingNew}>{t("create")}</Link>
              </Button>
            </>
          )}
        </EmptyState>
      ) : page ? (
        <>
          <p role="status" className="sr-only">
            {switching ? t("switching") : ""}
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
                <col className="w-56" />
              </colgroup>
              <caption className="sr-only">
                {t("caption", { first: firstRow, last: lastRow, total })}
              </caption>
              <thead className="hidden xl:table-header-group">
                <tr className="border-b text-xs text-muted-foreground">
                  <th scope="col" className="h-10 pr-4 pl-5 font-normal">
                    {t("columns.posting")}
                  </th>
                  <th scope="col" className="h-10 pr-4 font-normal">
                    {t("columns.status")}
                  </th>
                  <th scope="col" className="h-10 pr-4 text-left font-normal">
                    {t("columns.questions")}
                  </th>
                  <th scope="col" className="h-10 pr-4 text-left font-normal">
                    {t("columns.created")}
                  </th>
                  <th scope="col" className="h-10 pr-5 text-right font-normal">
                    <span className="sr-only">{t("columns.actions")}</span>
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
            aria-label={tCommon("labels.pagination")}
            className="flex flex-wrap items-center justify-between gap-3 text-sm"
          >
            <p className="text-muted-foreground tabular-nums">
              {t("pagination.showing", {
                first: firstRow,
                last: lastRow,
                total,
              })}
            </p>
            <div className="flex gap-2">
              <PageLink
                label={tCommon("actions.previous")}
                filters={{ ...filters, page: filters.page - 1 }}
                disabled={filters.page <= 1}
              />
              <PageLink
                label={tCommon("actions.next")}
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
  const t = useTranslations("recruiter.postings.list.row");
  const fmt = useFormatters();
  const details = [posting.location, posting.employment_type]
    .filter(Boolean)
    .join(" · ");
  const editable =
    posting.status === "pending" || posting.status === "rejected";
  const needsFunding =
    posting.status === "open" && posting.interview_slot === null;
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
          {details || t("noDetails")}
          <span className="xl:hidden">
            {" · "}
            {t("summary", {
              count,
              date: fmt.date(posting.created_at, "calendarDay"),
            })}
          </span>
        </span>
        {posting.status === "rejected" && posting.reject_reason && (
          <span className="mt-1 block text-[13px] text-destructive [overflow-wrap:anywhere]">
            {t("reason", { reason: posting.reject_reason })}
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
          {fmt.date(posting.created_at, "calendarDay")}
        </time>
      </td>
      <td className="col-span-2 xl:py-2.5 xl:pr-5">
        <div className="flex flex-wrap gap-2 xl:justify-end">
          {!needsFunding && (
            <Button asChild variant="ghost" className="h-9 rounded-full px-3">
              <Link
                href={routes.recruiter.jobPostingQuestions(posting.id)}
                aria-label={t("questionBankFor", { title: posting.title })}
              >
                {t("questions")}
              </Link>
            </Button>
          )}
          <Button asChild variant="outline" className="h-9 rounded-full px-3">
            <Link
              href={`${routes.recruiter.jobPosting(posting.id)}${needsFunding ? "#interview-capacity" : ""}`}
              aria-label={
                needsFunding
                  ? t("fundFor", { title: posting.title })
                  : editable
                    ? t("editFor", { title: posting.title })
                    : t("viewFor", { title: posting.title })
              }
            >
              {needsFunding ? t("fund") : editable ? t("edit") : t("view")}
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
  const t = useTranslations("recruiter.postings.list.search");
  const tCommon = useTranslations("common");
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
      aria-label={t("form")}
      onSubmit={submit}
      className="flex flex-wrap items-center gap-2"
    >
      <label htmlFor="posting-search" className="sr-only">
        {t("label")}
      </label>
      <div className="relative">
        <Search
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden
        />
        <Input
          id="posting-search"
          type="search"
          placeholder={t("label")}
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
        {tCommon("actions.search")}
      </Button>
      {filters.search && (
        <Button asChild variant="ghost" className="h-9 rounded-full px-3">
          <FilterLink filters={{ ...filters, search: "", page: 1 }}>
            <X aria-hidden />
            {tCommon("actions.clearSearch")}
          </FilterLink>
        </Button>
      )}
      {tooLong && (
        <p
          id="posting-search-error"
          role="alert"
          className="w-full text-xs text-destructive"
        >
          {t("tooLong", { max: SEARCH_MAX })}
        </p>
      )}
    </form>
  );
}
