import { useTranslations } from "next-intl";
import { Card } from "@/components/ui/card";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Overview } from "../schemas/overview-schema";

type Tile = { label: string; value: string };

// One section card: its title, the definition text, and label + number tiles. Zero is shown as 0.
function OverviewSection({
  section,
  title,
  definition,
  tiles,
}: {
  section: string;
  title: string;
  definition: string;
  tiles: Tile[];
}) {
  const id = `overview-${section}`;
  return (
    <section aria-labelledby={id} aria-describedby={`${id}-def`}>
      <Card className="h-full">
        <div className="space-y-1 px-4">
          <h2 id={id} className="text-base font-semibold">
            {title}
          </h2>
          <p id={`${id}-def`} className="text-sm text-muted-foreground">
            {definition}
          </p>
        </div>
        <dl className="grid grid-cols-2 gap-3 px-4 md:grid-cols-3">
          {tiles.map((tile) => (
            <div key={tile.label} className="rounded-lg border p-3">
              <dt className="text-xs text-muted-foreground">{tile.label}</dt>
              <dd className="mt-1 text-xl font-semibold tabular-nums">
                {tile.value}
              </dd>
            </div>
          ))}
        </dl>
      </Card>
    </section>
  );
}

// The sections for loaded numbers. Definitions say what each count covers.
export function OverviewBody({ overview }: { overview: Overview }) {
  const fmt = useFormatters();
  const t = useTranslations("admin.overview.sections");
  const n = (value: number) => fmt.number(value);
  const o = overview;

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <OverviewSection
        section="accounts"
        title={t("accounts.title")}
        definition={t("accounts.definition")}
        tiles={[
          { label: t("accounts.candidates"), value: n(o.accounts.candidates) },
          { label: t("accounts.recruiters"), value: n(o.accounts.recruiters) },
          { label: t("accounts.locked"), value: n(o.accounts.locked) },
          {
            label: t("accounts.newInWindow"),
            value: n(o.accounts.new_in_window),
          },
        ]}
      />
      <OverviewSection
        section="job-postings"
        title={t("jobPostings.title")}
        definition={t("jobPostings.definition")}
        tiles={[
          { label: t("jobPostings.pending"), value: n(o.job_postings.pending) },
          { label: t("jobPostings.open"), value: n(o.job_postings.open) },
          {
            label: t("jobPostings.intakeClosed"),
            value: n(o.job_postings.intake_closed),
          },
          { label: t("jobPostings.closed"), value: n(o.job_postings.closed) },
          {
            label: t("jobPostings.rejected"),
            value: n(o.job_postings.rejected),
          },
        ]}
      />
      <OverviewSection
        section="applications"
        title={t("applications.title")}
        definition={t("applications.definition")}
        tiles={[
          {
            label: t("applications.pending"),
            value: n(o.applications.pending),
          },
          {
            label: t("applications.interviewEligible"),
            value: n(o.applications.interview_eligible),
          },
          {
            label: t("applications.interviewed"),
            value: n(o.applications.interviewed),
          },
          {
            label: t("applications.approved"),
            value: n(o.applications.approved),
          },
          {
            label: t("applications.rejected"),
            value: n(o.applications.rejected),
          },
          {
            label: t("applications.submittedInWindow"),
            value: n(o.applications.submitted_in_window),
          },
        ]}
      />
      <OverviewSection
        section="interviews"
        title={t("interviews.title")}
        definition={t("interviews.definition")}
        tiles={[
          {
            label: t("interviews.practiceStarted"),
            value: n(o.interviews.practice_started_in_window),
          },
          {
            label: t("interviews.recruitmentStarted"),
            value: n(o.interviews.recruitment_started_in_window),
          },
          {
            label: t("interviews.completed"),
            value: n(o.interviews.completed_in_window),
          },
          {
            label: t("interviews.evaluationFailed"),
            value: n(o.interviews.evaluation_failed),
          },
        ]}
      />
      <div className="md:col-span-2">
        <OverviewSection
          section="finance"
          title={t("finance.title")}
          definition={t("finance.definition")}
          tiles={[
            {
              label: t("finance.revenue"),
              value: fmt.vnd(o.finance.revenue_vnd),
            },
            {
              label: t("finance.successfulDeposits"),
              value: n(o.finance.successful_deposits),
            },
            { label: t("finance.coinsSold"), value: n(o.finance.coins_sold) },
            { label: t("finance.coinsSpent"), value: n(o.finance.coins_spent) },
            {
              label: t("finance.coinsRefunded"),
              value: n(o.finance.coins_refunded),
            },
            {
              label: t("finance.uniquePayers"),
              value: n(o.finance.unique_payers),
            },
            {
              label: t("finance.successRate"),
              value: fmt.percent(o.finance.success_rate),
            },
          ]}
        />
      </div>
    </div>
  );
}
