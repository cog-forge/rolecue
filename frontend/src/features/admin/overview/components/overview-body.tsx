import {
  Activity,
  ArrowDownRight,
  ArrowRight,
  ArrowUpRight,
  Banknote,
  ClipboardCheck,
  FileText,
  LockKeyhole,
  UsersRound,
} from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Card } from "@/components/ui/card";
import { routes } from "@/config/routes";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Overview } from "../schemas/overview-schema";
import { InterviewActivityChart } from "./interview-activity-chart";

function MetricChange({
  value,
  previous,
}: {
  value: number;
  previous?: number;
}) {
  const t = useTranslations("admin.overview.metrics");
  const fmt = useFormatters();
  if (previous === undefined) return null;
  if (value === previous) return <span>{t("unchanged")}</span>;
  if (previous === 0) return <span>{t("newActivity")}</span>;

  const change = ((value - previous) / previous) * 100;
  const direction = change > 0 ? "up" : "down";
  const Icon = direction === "up" ? ArrowUpRight : ArrowDownRight;
  const magnitude = Math.abs(change);
  const percent =
    magnitude < 0.1
      ? t("lessThan", { percent: fmt.percent(0.1) })
      : fmt.percent(magnitude);

  return (
    <span className="inline-flex items-center gap-1 font-medium text-foreground">
      <Icon className="size-4" aria-hidden />
      {t(direction, { percent })}
    </span>
  );
}

function StatusBreakdown({
  title,
  description,
  rows,
  href,
  linkLabel,
}: {
  title: string;
  description: string;
  rows: { label: string; count: number }[];
  href?: string;
  linkLabel?: string;
}) {
  const fmt = useFormatters();
  const total = rows.reduce((sum, row) => sum + row.count, 0);

  return (
    <Card
      role="region"
      aria-label={title}
      className="min-w-0 gap-4 px-4 sm:px-6 xl:col-span-6"
    >
      <div className="flex flex-wrap items-start justify-between gap-x-3">
        <div className="space-y-1 pt-1">
          <h2 className="text-base font-semibold tracking-tight">{title}</h2>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        {href && (
          <Link
            href={href}
            className="inline-flex min-h-11 items-center gap-1 rounded-md text-xs font-medium underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring"
          >
            {linkLabel}
            <ArrowRight className="size-4" aria-hidden />
          </Link>
        )}
      </div>
      <dl className="space-y-3">
        {rows.map(({ label, count }) => (
          <div
            key={label}
            className="relative grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 pb-3"
          >
            <dt className="text-sm text-muted-foreground">{label}</dt>
            <dd className="text-sm font-medium tabular-nums">
              {fmt.number(count)}
              <span
                aria-hidden
                className="absolute inset-x-0 bottom-0 block h-1.5 overflow-hidden rounded-full bg-muted"
              >
                <span
                  className="block h-full rounded-full bg-rolecue-brand/65"
                  style={{ width: `${total ? (count / total) * 100 : 0}%` }}
                />
              </span>
            </dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}

export function OverviewBody({ overview }: { overview: Overview }) {
  const t = useTranslations("admin.overview");
  const fmt = useFormatters();
  const {
    accounts,
    applications,
    finance,
    interviews,
    interview_activity,
    job_postings,
  } = overview;
  const previousStarts = interview_activity?.buckets.reduce(
    (sum, bucket) =>
      sum + bucket.previous_practice + bucket.previous_recruitment,
    0,
  );
  const metrics = [
    {
      label: t("metrics.newAccounts"),
      value: accounts.new_in_window,
      previous: accounts.new_in_previous_window,
      icon: UsersRound,
    },
    {
      label: t("metrics.newApplications"),
      value: applications.submitted_in_window,
      previous: applications.submitted_in_previous_window,
      icon: FileText,
    },
    {
      label: t("metrics.interviewsStarted"),
      value:
        interviews.practice_started_in_window +
        interviews.recruitment_started_in_window,
      previous: previousStarts,
      icon: Activity,
    },
    {
      label: t("metrics.payosTopups"),
      value: finance.revenue_vnd,
      previous: finance.previous_revenue_vnd,
      icon: Banknote,
      href: routes.admin.revenue,
    },
  ];
  const operations = [
    {
      label: t("attention.postings"),
      value: job_postings.pending,
      href: routes.admin.jobPostings,
      icon: ClipboardCheck,
    },
    {
      label: t("attention.evaluations"),
      value: interviews.evaluation_failed,
      href: routes.admin.interviews,
      icon: Activity,
    },
    {
      label: t("attention.lockedAccounts"),
      value: accounts.locked,
      href: routes.admin.users,
      icon: LockKeyhole,
    },
  ];
  const previous = overview.previous_window;

  return (
    <div className="grid gap-4 sm:gap-5 xl:grid-cols-12">
      <section
        aria-labelledby="admin-overview-key-metrics"
        className="min-w-0 space-y-3 xl:col-span-12"
      >
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <h2 id="admin-overview-key-metrics" className="text-sm font-semibold">
            {t("metrics.title")}
          </h2>
          <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>
              {t("metrics.currentRange", {
                from: fmt.date(overview.window.from, "vietnamDateTime"),
                to: fmt.date(overview.window.to, "vietnamDateTime"),
              })}
            </span>
            {previous && (
              <span>
                {t("metrics.comparison", {
                  from: fmt.date(previous.from, "vietnamDateTime"),
                  to: fmt.date(previous.to, "vietnamDateTime"),
                })}
              </span>
            )}
            <span>{t("metrics.timeZone")}</span>
          </div>
        </div>
        <ul className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
          {metrics.map(({ label, value, previous, icon: Icon, href }) => (
            <li key={label} className="min-w-0">
              <Card className="relative h-full px-3 sm:px-4">
                <dl className="flex h-full flex-col gap-2">
                  <dt className="flex grow items-start justify-between gap-2 text-xs leading-5 font-medium text-muted-foreground sm:text-sm">
                    {href ? (
                      <Link
                        href={href}
                        className="inline-flex items-center gap-1 outline-none underline-offset-4 after:absolute after:inset-0 after:rounded-xl hover:underline focus-visible:after:ring-2 focus-visible:after:ring-ring focus-visible:after:ring-inset"
                      >
                        {label}
                        <ArrowUpRight
                          className="size-3.5 shrink-0"
                          aria-hidden
                        />
                      </Link>
                    ) : (
                      label
                    )}
                    <Icon
                      className="size-4 shrink-0 text-rolecue-brand"
                      aria-hidden
                    />
                  </dt>
                  <dd className="min-w-0 space-y-1">
                    <p className="text-lg leading-tight font-semibold tracking-tight break-words tabular-nums sm:text-2xl">
                      {href ? fmt.vnd(value) : fmt.number(value)}
                    </p>
                    <p className="min-h-5 text-xs text-muted-foreground">
                      <MetricChange value={value} previous={previous} />
                    </p>
                  </dd>
                </dl>
              </Card>
            </li>
          ))}
        </ul>
      </section>

      <Card
        role="region"
        aria-labelledby="admin-overview-attention"
        className="min-w-0 gap-4 px-4 sm:px-6 xl:col-start-9 xl:col-end-13 xl:row-start-2"
      >
        <div className="space-y-1 pt-1">
          <h2
            id="admin-overview-attention"
            className="text-base font-semibold tracking-tight"
          >
            {t("attention.title")}
          </h2>
          <p className="text-sm text-muted-foreground">
            {t("attention.snapshot")}
          </p>
        </div>
        <ul className="divide-y divide-border">
          {operations.map(({ label, value, href, icon: Icon }) => (
            <li key={href}>
              <Link
                href={href}
                className="group flex min-h-14 items-center gap-3 rounded-md px-1 py-2 outline-none transition-colors hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring motion-reduce:transition-none"
              >
                <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
                  <Icon className="size-4" aria-hidden />
                </span>
                <span className="min-w-0 flex-1 text-sm font-medium">
                  {label}
                </span>{" "}
                <span className="shrink-0 text-base font-semibold tabular-nums">
                  {fmt.number(value)}
                </span>
                <ArrowRight
                  className="size-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none motion-reduce:group-hover:translate-x-0"
                  aria-hidden
                />
              </Link>
            </li>
          ))}
        </ul>
        <div className="mt-auto border-t pt-4">
          <h3 className="text-sm font-medium">{t("attention.accounts")}</h3>
          <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-2">
            <div className="min-w-0 space-y-1">
              <dt className="text-xs text-muted-foreground">
                {t("attention.candidates")}
              </dt>
              <dd className="break-words text-lg font-semibold tabular-nums">
                {fmt.number(accounts.candidates)}
              </dd>
            </div>
            <div className="min-w-0 space-y-1">
              <dt className="text-xs text-muted-foreground">
                {t("attention.recruiters")}
              </dt>
              <dd className="break-words text-lg font-semibold tabular-nums">
                {fmt.number(accounts.recruiters)}
              </dd>
            </div>
          </dl>
        </div>
      </Card>

      <InterviewActivityChart overview={overview} />

      <StatusBreakdown
        title={t("sections.jobPostings.title")}
        description={t("status.description")}
        href={`${routes.admin.jobPostings}?status=all`}
        linkLabel={t("status.viewPostings")}
        rows={[
          {
            label: t("sections.jobPostings.pending"),
            count: job_postings.pending,
          },
          { label: t("sections.jobPostings.open"), count: job_postings.open },
          {
            label: t("sections.jobPostings.intakeClosed"),
            count: job_postings.intake_closed,
          },
          {
            label: t("sections.jobPostings.closed"),
            count: job_postings.closed,
          },
          {
            label: t("sections.jobPostings.rejected"),
            count: job_postings.rejected,
          },
        ]}
      />
      <StatusBreakdown
        title={t("sections.applications.title")}
        description={t("status.description")}
        rows={[
          {
            label: t("sections.applications.pending"),
            count: applications.pending,
          },
          {
            label: t("sections.applications.interviewEligible"),
            count: applications.interview_eligible,
          },
          {
            label: t("sections.applications.interviewed"),
            count: applications.interviewed,
          },
          {
            label: t("sections.applications.approved"),
            count: applications.approved,
          },
          {
            label: t("sections.applications.rejected"),
            count: applications.rejected,
          },
        ]}
      />
    </div>
  );
}
