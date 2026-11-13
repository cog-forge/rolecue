"use client";
import { EmptyState } from "@/components/ui/empty-state";

import { ErrorState } from "@/components/ui/error-state";

import {
  useEffect,
  useState,
  useSyncExternalStore,
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
import { PageHeader } from "@/components/layout/page-header";
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
  parseSelectedJob,
  type BoardFilters,
} from "../utils/board-filters";
import {
  fullDate,
  levelText,
  publicationText,
  salaryText,
} from "../utils/format";
import { JobPostingDetail } from "./job-posting-detail";
import { useFormatters } from "@/lib/i18n/use-formatters";

// The list and the selected posting sit side by side from Tailwind's lg breakpoint.
const DESKTOP = "(min-width: 1024px)";
// How many skills a card previews before "+N".
const CARD_SKILLS = 4;

function subscribeDesktop(onChange: () => void) {
  const media = window.matchMedia(DESKTOP);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}
// False on the server and in the first client render, so the markup matches before hydration.
function useDesktop() {
  return useSyncExternalStore(
    subscribeDesktop,
    () => window.matchMedia(DESKTOP).matches,
    () => false,
  );
}

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

function useBoardParams() {
  const params = useSearchParams();
  const record: Record<string, string | string[]> = {};
  for (const key of new Set(params.keys())) {
    const values = params.getAll(key);
    record[key] = values.length === 1 ? values[0] : values;
  }
  return { filters: parseBoardFilters(record), job: parseSelectedJob(record) };
}

export function JobBoard() {
  const t = useTranslations("candidate.jobBoard.board");
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
  const { filters, job } = useBoardParams();
  const desktop = useDesktop();
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
  const counts = {
    first: fmt.number(firstRow),
    last: fmt.number(lastRow),
    total: fmt.number(total),
  };
  const filtered = hasTextFilters(filters);
  // An empty previous result says nothing about the new search, so show loading instead.
  const loading = query.isPending || (switching && page?.items.length === 0);
  // On wide screens a posting is always open beside the list: the one in the URL, else the first.
  const selected = desktop ? (job ?? page?.items[0]?.id ?? null) : null;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <PageHeader title={t("title")} description={t("description")} />

      <FilterForm
        key={`${filters.search}\n${filters.location}\n${filters.type}`}
        filters={filters}
      />

      {loading ? (
        <div role="status" className="space-y-3">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-28 rounded-xl" />
          ))}
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <ErrorState
          title={t("deniedTitle")}
          description={t("accessChecking")}
        ></ErrorState>
      ) : query.isError ? (
        <ErrorState
          title={t("errorTitle")}
          description={t("errorText")}
          onRetry={() => void query.refetch()}
          retryLabel={t("retry")}
          isRetrying={query.isFetching}
        ></ErrorState>
      ) : page && page.items.length === 0 && total > 0 ? (
        <EmptyState title={t("pastEnd")} description="">
          <Button
            asChild
            variant="outline"
            className="mt-4 h-auto min-h-11 py-2 whitespace-normal"
          >
            <FilterLink filters={{ ...filters, page: 1 }}>
              {t("firstPage")}
            </FilterLink>
          </Button>
        </EmptyState>
      ) : page && page.items.length === 0 ? (
        <EmptyState
          icons={[BriefcaseBusiness]}
          title={filtered ? t("noMatchTitle") : t("emptyTitle")}
          description={filtered ? t("noMatchText") : t("emptyText")}
        >
          {filtered && (
            <Button
              asChild
              variant="outline"
              className="mt-4 h-auto min-h-11 py-2 whitespace-normal"
            >
              <FilterLink filters={defaultBoardFilters}>
                {t("clearSearch")}
              </FilterLink>
            </Button>
          )}
        </EmptyState>
      ) : page ? (
        <>
          <p role="status" className="sr-only">
            {switching ? t("switching") : ""}
          </p>
          <div className="lg:grid lg:grid-cols-[minmax(0,25rem)_minmax(0,1fr)] lg:items-start lg:gap-6">
            <div className="space-y-4">
              <ul
                aria-label={t("list", counts)}
                aria-busy={switching}
                className={cn(
                  "divide-y rounded-xl border transition-opacity duration-[var(--duration-fast)] motion-reduce:transition-none",
                  switching && "opacity-60",
                )}
              >
                {page.items.map((posting) => (
                  <PostingCard
                    key={posting.id}
                    posting={posting}
                    filters={filters}
                    selected={posting.id === selected}
                  />
                ))}
              </ul>
              <nav
                aria-label={t("pagination")}
                className="flex flex-wrap items-center justify-between gap-3 text-sm"
              >
                <p className="text-muted-foreground tabular-nums">
                  {t("showing", counts)}
                </p>
                <div className="flex gap-2">
                  <PageLink
                    label={t("previous")}
                    filters={{ ...filters, page: shownPage - 1 }}
                    disabled={switching || shownPage <= 1}
                  />
                  <PageLink
                    label={t("next")}
                    filters={{ ...filters, page: shownPage + 1 }}
                    disabled={switching || lastRow >= total}
                  />
                </div>
              </nav>
            </div>
            {selected && (
              <aside
                aria-label={t("selected")}
                className="hidden rounded-xl border p-6 lg:sticky lg:top-6 lg:block lg:max-h-[calc(100dvh-3rem)] lg:overflow-y-auto"
              >
                <JobPostingDetail
                  key={selected}
                  id={selected}
                  variant="panel"
                />
              </aside>
            )}
          </div>
        </>
      ) : null}
    </section>
  );
}

function PostingCard({
  posting,
  filters,
  selected,
}: {
  posting: BoardPosting;
  filters: BoardFilters;
  selected: boolean;
}) {
  const t = useTranslations("candidate.jobBoard");
  const fmt = useFormatters();
  const salary = salaryText(posting, fmt, t);
  const facts = [
    posting.employment_type,
    levelText(posting.seniority_level, t),
    salary,
  ].filter(Boolean);
  const skills = posting.top_skills?.slice(0, CARD_SKILLS) ?? [];
  const more = Math.max(
    0,
    (posting.required_skill_count ?? skills.length) - skills.length,
  );
  return (
    <li
      aria-current={selected || undefined}
      className={cn(
        "group relative p-4 transition-colors duration-[var(--duration-quick)] first:rounded-t-xl last:rounded-b-xl hover:bg-muted/30 has-[a:focus-visible]:ring-3 has-[a:focus-visible]:ring-ring/50 motion-reduce:transition-none",
        selected &&
          "bg-muted/50 before:absolute before:inset-y-3 before:left-0 before:w-0.5 before:rounded-full before:bg-foreground hover:bg-muted/50",
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h2 className="text-base font-semibold [overflow-wrap:anywhere]">
            {/* The stretched link makes the whole card clickable with one tab stop. On wide screens a
                plain click opens the posting beside the list; below that it opens the posting page. */}
            <Link
              href={postingHref(posting.id, filters)}
              scroll={false}
              onClick={(event: MouseEvent<HTMLAnchorElement>) => {
                if (
                  event.button !== 0 ||
                  event.metaKey ||
                  event.ctrlKey ||
                  event.shiftKey ||
                  event.altKey ||
                  !window.matchMedia(DESKTOP).matches
                )
                  return;
                event.preventDefault();
                window.history.pushState(
                  null,
                  "",
                  `${routes.jobs}${boardSearch(filters, posting.id)}`,
                );
              }}
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
          className="mt-1 size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none lg:hidden"
          aria-hidden
        />
      </div>
      {skills.length > 0 && (
        <ul
          aria-label={t("board.skills")}
          className="mt-3 flex flex-wrap gap-1.5"
        >
          {skills.map((skill) => (
            <li
              key={skill}
              className="rounded-md bg-muted px-2 py-0.5 text-xs [overflow-wrap:anywhere]"
            >
              {skill}
            </li>
          ))}
          {more > 0 && (
            <li className="px-1 py-0.5 text-xs text-muted-foreground">
              +{fmt.number(more)}
              <span className="sr-only"> {t("board.moreSkills")}</span>
            </li>
          )}
        </ul>
      )}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs">
        <ul
          aria-label={t("board.details")}
          className="flex flex-wrap gap-x-4 gap-y-1"
        >
          {facts.map((fact, i) => (
            <li
              key={i}
              className="text-muted-foreground [overflow-wrap:anywhere]"
            >
              {fact}
            </li>
          ))}
        </ul>
        <time
          dateTime={posting.published_at ?? posting.created_at}
          title={fullDate(posting.published_at ?? posting.created_at, fmt)}
          className="text-muted-foreground"
        >
          {publicationText(posting, fmt, t)}
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
      className="h-auto min-h-11 py-2 whitespace-normal aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
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
  { name: "search", id: "board-search" },
  { name: "location", id: "board-location" },
  { name: "type", id: "board-type" },
] as const;
type FieldName = (typeof fields)[number]["name"];

function FilterForm({ filters }: { filters: BoardFilters }) {
  const t = useTranslations("candidate.jobBoard.filters");
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
      aria-label={t("form")}
      onSubmit={submit}
      className="grid gap-3 border-b pb-5 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_1fr_auto] lg:items-start"
    >
      {fields.map((field) => {
        const invalid = isFilterTooLong(values[field.name]);
        const hint = field.name === "type" ? `${field.id}-hint` : null;
        const describedBy = [hint, invalid ? `${field.id}-error` : null]
          .filter(Boolean)
          .join(" ");
        return (
          <div key={field.name} className="space-y-1">
            <label htmlFor={field.id} className="text-xs font-medium">
              {t(`${field.name}.label`)}
            </label>
            <Input
              id={field.id}
              type="search"
              placeholder={t(`${field.name}.placeholder`)}
              value={values[field.name]}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  [field.name]: event.target.value,
                }))
              }
              className="h-11 rounded-full"
              aria-invalid={invalid}
              aria-describedby={describedBy || undefined}
            />
            {field.name === "type" && (
              <p
                id={hint ?? undefined}
                className="text-xs text-muted-foreground"
              >
                {t("type.hint")}
              </p>
            )}
            {invalid && (
              <p
                id={`${field.id}-error`}
                role="alert"
                className="text-xs text-destructive"
              >
                {t("tooLong", { max: FILTER_MAX })}
              </p>
            )}
          </div>
        );
      })}
      <div className="flex gap-2 sm:col-span-2 lg:col-span-1 lg:pt-5">
        <Button
          type="submit"
          className="h-auto min-h-11 py-2 whitespace-normal rounded-full"
          disabled={tooLong}
        >
          <Search aria-hidden />
          {t("submit")}
        </Button>
        {hasTextFilters(filters) && (
          <Button
            asChild
            variant="ghost"
            className="h-auto min-h-11 py-2 whitespace-normal rounded-full"
          >
            <FilterLink filters={defaultBoardFilters}>
              <X aria-hidden />
              {t("clear")}
            </FilterLink>
          </Button>
        )}
      </div>
    </form>
  );
}
