"use client";

import { useEffect } from "react";
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/layout/page-header";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import { getOverview, isAccessDenied, overviewKeys } from "../api/overview";
import { parseOverviewDays } from "../utils/overview-days";
import { OverviewBody } from "./overview-body";
import { WindowSelector } from "./window-selector";

// The reporting window lives in the URL (?days=) so reload, back and sharing keep it.
export function AdminOverview() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const fmt = useFormatters();
  const days = parseOverviewDays(useSearchParams().get("days"));
  const query = useQuery({
    queryKey: overviewKeys.overview(user.id, days),
    queryFn: ({ signal }) => getOverview(days, signal),
    retry: false,
    // Keep the previous window on screen while the next one loads.
    placeholderData: keepPreviousData,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const overview = query.data;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <PageHeader
        title="Admin dashboard"
        description="Platform operations overview."
        meta={
          overview ? (
            <span>
              Last {overview.window.days} days · Updated{" "}
              {fmt.date(overview.generated_at, "localDateTime")}
            </span>
          ) : undefined
        }
      />

      <WindowSelector days={days} />

      {query.isPending ? (
        <div role="status" className="space-y-4">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-40 rounded-xl" />
          ))}
          <span className="sr-only">Loading admin overview…</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">
            You don&apos;t have access to the admin overview
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Checking your account access… If this stays, sign in again with an
            administrator account.
          </p>
        </div>
      ) : query.isError || !overview ? (
        <div role="alert" className="rounded-xl border p-6">
          <h2 className="font-semibold">We couldn&apos;t load the overview</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please try again in a moment.
          </p>
          <Button
            className="mt-4 h-11"
            variant="outline"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            Try again
          </Button>
        </div>
      ) : (
        <div
          aria-busy={query.isPlaceholderData}
          className={cn(
            "transition-opacity duration-200 motion-reduce:transition-none",
            // The numbers still belong to the previous window until the new one arrives.
            query.isPlaceholderData && "opacity-60",
          )}
        >
          <p role="status" className="sr-only">
            {query.isPlaceholderData ? `Loading the last ${days} days…` : ""}
          </p>
          <OverviewBody overview={overview} />
        </div>
      )}

      <nav aria-label="Admin tools" className="rounded-xl border p-5">
        <h2 className="font-semibold">Go to</h2>
        <ul className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {quickLinks.map((link) => (
            <li key={link.href}>
              <Link
                href={link.href}
                className="inline-flex min-h-11 items-center rounded-md px-2 text-sm font-medium underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {link.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </section>
  );
}

const quickLinks = [
  { href: routes.admin.jobPostings, label: "Moderation queue" },
  { href: routes.admin.interviews, label: "Interview sessions" },
  { href: routes.admin.voices, label: "Voice profiles" },
  { href: routes.admin.billing, label: "Payment transactions" },
  { href: routes.admin.revenue, label: "Revenue reports" },
  { href: routes.admin.users, label: "Accounts" },
];
