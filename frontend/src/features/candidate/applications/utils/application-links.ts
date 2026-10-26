import { routes } from "@/config/routes";

// The Candidate's own Application detail.
export function applicationHref(id: string) {
  return routes.application(id);
}
