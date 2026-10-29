import Link from "next/link";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import type { PostingStatus } from "../schemas/job-posting-schema";
import { postingInboxHref, takesApplications } from "../utils/posting-guidance";

export type PostingTab =
  "overview" | "applications" | "questions" | "evaluation";

const tabClass =
  "inline-flex min-h-11 items-center border-b-2 border-transparent text-sm whitespace-nowrap transition-colors duration-[var(--duration-quick)] outline-none focus-visible:ring-3 focus-visible:ring-ring/50";

// The sections of one posting, as real links so each has its own address. Applications opens the
// review inbox for this posting and stays off until the posting can take applications.
export function PostingTabs({
  postingId,
  active,
  status,
}: {
  postingId: string;
  active: PostingTab;
  status: PostingStatus;
}) {
  const tabs: { id: PostingTab; label: string; href: string }[] = [
    {
      id: "overview",
      label: "Overview",
      href: routes.recruiter.jobPosting(postingId),
    },
    {
      id: "applications",
      label: "Applications",
      href: postingInboxHref(postingId),
    },
    {
      id: "questions",
      label: "Questions",
      href: routes.recruiter.jobPostingQuestions(postingId),
    },
    {
      id: "evaluation",
      label: "Evaluation",
      href: routes.recruiter.jobPostingEvaluation(postingId),
    },
  ];
  return (
    <div className="border-b">
      <nav
        aria-label="Posting sections"
        className="-mb-px max-w-full overflow-x-auto"
      >
        <ul className="flex gap-6">
          {tabs.map((tab) => {
            const selected = tab.id === active;
            if (tab.id === "applications" && !takesApplications(status))
              return (
                <li key={tab.id}>
                  {/* A link with no href is the standard disabled link; the reason is read aloud. */}
                  <a
                    role="link"
                    aria-disabled="true"
                    title="Available once an administrator approves the posting and you fund interview slots"
                    className={cn(
                      tabClass,
                      "cursor-not-allowed text-muted-foreground/60",
                    )}
                  >
                    {tab.label}
                    <span className="sr-only">
                      {" "}
                      (available once an administrator approves the posting and
                      you fund interview slots)
                    </span>
                  </a>
                </li>
              );
            return (
              <li key={tab.id}>
                <Link
                  href={tab.href}
                  aria-current={selected ? "page" : undefined}
                  className={cn(
                    tabClass,
                    selected
                      ? "border-foreground font-medium text-foreground"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {tab.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </div>
  );
}
