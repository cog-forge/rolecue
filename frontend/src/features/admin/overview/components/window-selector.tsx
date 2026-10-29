"use client";

import type { MouseEvent } from "react";
import Link from "next/link";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { overviewDays, type OverviewDays } from "../schemas/overview-schema";
import { overviewSearch } from "../utils/overview-days";

function modified(event: MouseEvent) {
  return (
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  );
}

// Real links (open in new tab still works); a plain click swaps the window without a reload.
export function WindowSelector({ days }: { days: OverviewDays }) {
  return (
    <nav aria-label="Reporting window">
      <ul className="flex flex-wrap gap-2">
        {overviewDays.map((option) => {
          const active = option === days;
          const href = `${routes.admin.dashboard}${overviewSearch(option)}`;
          return (
            <li key={option}>
              <Link
                href={href}
                scroll={false}
                aria-current={active ? "page" : undefined}
                onClick={(event: MouseEvent<HTMLAnchorElement>) => {
                  if (event.defaultPrevented || modified(event)) return;
                  event.preventDefault();
                  window.history.pushState(null, "", href);
                }}
                className={cn(
                  "inline-flex min-h-11 items-center rounded-full border px-4 text-sm outline-none transition-colors duration-150 hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
                  active
                    ? "border-foreground font-medium text-foreground"
                    : "text-muted-foreground",
                )}
              >
                Last {option} days
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
