import { overviewDays, type OverviewDays } from "../schemas/overview-schema";

export const defaultOverviewDays: OverviewDays = 30;

// Only the supported windows are read from the URL; anything else shows the default window.
export function parseOverviewDays(value: string | null): OverviewDays {
  return (
    overviewDays.find((days) => String(days) === value) ?? defaultOverviewDays
  );
}

// The default window is left out of the query string to keep the plain URL short.
export function overviewSearch(days: OverviewDays) {
  return days === defaultOverviewDays ? "" : `?days=${days}`;
}
