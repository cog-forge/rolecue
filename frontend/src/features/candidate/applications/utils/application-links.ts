import { routes } from "@/config/routes";

// The Candidate's own Application detail. Built from the applications list route; the detail route
// is not in routes.ts yet.
export function applicationHref(id: string) {
  return `${routes.applications}/${encodeURIComponent(id)}`;
}
