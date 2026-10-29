import { Card } from "@/components/ui/card";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Overview } from "../schemas/overview-schema";

type Tile = { label: string; value: string };

// One section card: its title, the definition text, and label + number tiles. Zero is shown as 0.
function OverviewSection({
  title,
  definition,
  tiles,
}: {
  title: string;
  definition: string;
  tiles: Tile[];
}) {
  const id = `overview-${title.toLowerCase().replace(/\s+/g, "-")}`;
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
  const n = (value: number) => fmt.number(value);
  const o = overview;

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <OverviewSection
        title="Accounts"
        definition="Candidates and recruiters only. Locked is the number locked right now; new accounts count the window."
        tiles={[
          { label: "Candidates", value: n(o.accounts.candidates) },
          { label: "Recruiters", value: n(o.accounts.recruiters) },
          { label: "Locked now", value: n(o.accounts.locked) },
          { label: "New in window", value: n(o.accounts.new_in_window) },
        ]}
      />
      <OverviewSection
        title="Job postings"
        definition="All-time counts by status."
        tiles={[
          { label: "Pending review", value: n(o.job_postings.pending) },
          { label: "Open", value: n(o.job_postings.open) },
          { label: "Intake closed", value: n(o.job_postings.intake_closed) },
          { label: "Closed", value: n(o.job_postings.closed) },
          { label: "Rejected", value: n(o.job_postings.rejected) },
        ]}
      />
      <OverviewSection
        title="Applications"
        definition="Counts by status are all-time. Submitted counts the window."
        tiles={[
          { label: "Awaiting CV review", value: n(o.applications.pending) },
          {
            label: "Interview eligible",
            value: n(o.applications.interview_eligible),
          },
          { label: "Interviewed", value: n(o.applications.interviewed) },
          { label: "Approved", value: n(o.applications.approved) },
          { label: "Rejected", value: n(o.applications.rejected) },
          {
            label: "Submitted in window",
            value: n(o.applications.submitted_in_window),
          },
        ]}
      />
      <OverviewSection
        title="Interviews"
        definition="Started and completed count the window. Evaluation failed is all-time."
        tiles={[
          {
            label: "Practice started",
            value: n(o.interviews.practice_started_in_window),
          },
          {
            label: "Recruitment started",
            value: n(o.interviews.recruitment_started_in_window),
          },
          { label: "Completed", value: n(o.interviews.completed_in_window) },
          { label: "Evaluation failed", value: n(o.interviews.evaluation_failed) },
        ]}
      />
      <div className="md:col-span-2">
        <OverviewSection
          title="Finance"
          definition="Revenue is successful PayOS top-ups in the window. Coin totals are for the window. Coin debits and refunds are not revenue."
          tiles={[
            { label: "Revenue", value: fmt.vnd(o.finance.revenue_vnd) },
            {
              label: "Successful top-ups",
              value: n(o.finance.successful_deposits),
            },
            { label: "Coins sold", value: n(o.finance.coins_sold) },
            { label: "Coins spent", value: n(o.finance.coins_spent) },
            { label: "Coins refunded", value: n(o.finance.coins_refunded) },
            { label: "Unique payers", value: n(o.finance.unique_payers) },
            {
              label: "Payment success rate",
              value: fmt.percent(o.finance.success_rate),
            },
          ]}
        />
      </div>
    </div>
  );
}
