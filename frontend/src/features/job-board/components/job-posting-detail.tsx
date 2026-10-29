"use client";

import { useEffect, useId, useState, type ReactNode } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import {
  ArrowLeft,
  ArrowUpRight,
  Building2,
  CircleCheck,
  Clock,
  ExternalLink,
  MapPin,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { ApplyDialog } from "@/features/candidate/applications/components/apply-dialog";
import { HiringSteps } from "@/features/candidate/applications/components/hiring-steps";
import { applicationHref } from "@/features/candidate/applications/utils/application-links";
import { isHttpsProfileUrl } from "@/features/profile/schemas/profile-schema";
import { cn } from "@/lib/utils";
import { getJobBoardPosting, jobBoardKeys } from "../api/job-board";
import type {
  BoardPostingDetail,
  BoardRequirement,
} from "../schemas/job-board-schema";
import { apiStatus, isAccessDenied } from "../utils/api-error";
import {
  fullDate,
  levelText,
  publicationText,
  salaryText,
  targetLabel,
} from "../utils/format";

// Only a website the profile rules accept (https, a host, no credentials) becomes a link.
function safeWebsite(value: string | null) {
  return value && isHttpsProfileUrl(value) ? new URL(value) : null;
}

// How many skills show before "Show all"; a long JD is clamped until the Candidate expands it.
const SKILLS_SHOWN = 10;
const LONG_JD_CHARACTERS = 800;
const LONG_JD_LINES = 14;

type Variant = "page" | "panel";

// One open posting. The page variant is the /jobs/[id] route (and the mobile view); the panel
// variant sits beside the board list on wide screens, so it has no back link and an h2 title.
export function JobPostingDetail({
  id,
  backQuery = "",
  variant = "page",
}: {
  id: string;
  backQuery?: string;
  variant?: Variant;
}) {
  const user = useSessionUser();
  const cache = useQueryClient();
  // Go rejects an ID that is not a UUID, so a mistyped link is not sent at all.
  const validId = z.uuid().safeParse(id).success;
  const query = useQuery({
    queryKey: jobBoardKeys.detail(user.id, id),
    queryFn: ({ signal }) => getJobBoardPosting(id, signal),
    enabled: validId,
    // "Already applied" and intake can change at any time, so always ask again.
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);
  // Go answers the same 404 for a posting that is missing, not open or closed to intake.
  const unavailable = apiStatus(query.error) === 404;
  const backHref = `${routes.jobs}${backQuery ? `?${backQuery}` : ""}`;
  const Heading = variant === "page" ? "h1" : "h2";
  const page = variant === "page";

  const content = !validId ? (
    <StateMessage
      heading={<Heading>This link isn&apos;t valid</Heading>}
      text="Check the address, or find the posting on the Job Board."
      action={
        page && (
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={backHref}>Browse open postings</Link>
          </Button>
        )
      }
    />
  ) : query.isPending ? (
    <div role="status" className="space-y-4">
      <Skeleton className="h-9 w-2/3" />
      <Skeleton className="h-5 w-1/2" />
      <Skeleton className="h-11 w-48 rounded-md" />
      <Skeleton className="h-64 rounded-xl" />
      <span className="sr-only">Loading the job posting…</span>
    </div>
  ) : denied ? (
    <StateMessage
      heading={<Heading>You don&apos;t have access to this posting</Heading>}
      text="Checking your account access… If this stays, sign in again with a candidate account."
    />
  ) : unavailable ? (
    <StateMessage
      heading={<Heading>This posting is no longer available</Heading>}
      text="It stopped accepting applications or was removed. Other open postings are waiting on the Job Board."
      action={
        page && (
          <Button asChild variant="outline" className="mt-4 h-11">
            <Link href={backHref}>Browse open postings</Link>
          </Button>
        )
      }
    />
  ) : query.isError ? (
    <StateMessage
      tone="danger"
      heading={<Heading>We couldn&apos;t load this job posting</Heading>}
      text="Please try again in a moment."
      action={
        <Button
          className="mt-4 h-11 bg-background"
          variant="outline"
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          Try again
        </Button>
      }
    />
  ) : (
    <PostingContent
      posting={query.data}
      canApply={user.role === "candidate"}
      variant={variant}
    />
  );

  if (!page) return content;
  return (
    <section className="mx-auto w-full max-w-3xl space-y-6">
      <Link
        href={backHref}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to Job Board
      </Link>
      {content}
    </section>
  );
}

function StateMessage({
  heading,
  text,
  action,
  tone,
}: {
  heading: ReactNode;
  text: string;
  action?: ReactNode;
  tone?: "danger";
}) {
  return (
    <div
      role="alert"
      className={cn(
        "rounded-xl border p-6 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold",
        tone === "danger" && "border-destructive/30 bg-destructive/5",
      )}
    >
      {heading}
      <p className="mt-2 text-sm text-muted-foreground">{text}</p>
      {action}
    </div>
  );
}

function PostingContent({
  posting,
  canApply,
  variant,
}: {
  posting: BoardPostingDetail;
  canApply: boolean;
  variant: Variant;
}) {
  const [applying, setApplying] = useState(false);
  const website = safeWebsite(posting.company_website);
  const salary = salaryText(posting);
  const Heading = variant === "page" ? "h1" : "h2";
  const Section = variant === "page" ? "h2" : "h3";
  const facts: { label: string; value: string }[] = [
    { label: "Level", value: levelText[posting.seniority_level] },
    ...(posting.employment_type
      ? [{ label: "Employment type", value: posting.employment_type }]
      : []),
    ...(salary ? [{ label: "Salary", value: salary }] : []),
    ...(posting.interview_duration
      ? [
          {
            label: "Interview length",
            value: `${posting.interview_duration} minutes`,
          },
        ]
      : []),
  ];

  return (
    <article className="space-y-7 duration-300 animate-in fade-in motion-reduce:animate-none">
      <header className="space-y-4">
        <div className="space-y-2">
          <Heading
            className={cn(
              "font-semibold tracking-tight [overflow-wrap:anywhere]",
              variant === "page" ? "text-3xl sm:text-4xl" : "text-2xl",
            )}
          >
            {posting.title}
          </Heading>
          <p className="flex flex-wrap gap-x-5 gap-y-1 text-sm text-muted-foreground">
            {website ? (
              <a
                href={website.href}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex min-h-11 items-center gap-1.5 rounded-sm font-medium text-foreground underline-offset-4 outline-none [overflow-wrap:anywhere] hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 sm:min-h-0"
              >
                <Building2 className="size-4 shrink-0" aria-hidden />
                {/* Without a company name, the website's host still says whose posting it is. */}
                {posting.company_name ?? website.hostname}
                <ExternalLink className="size-3.5 shrink-0" aria-hidden />
                <span className="sr-only">(opens in a new tab)</span>
              </a>
            ) : (
              posting.company_name && (
                <span className="inline-flex items-center gap-1.5 font-medium text-foreground [overflow-wrap:anywhere]">
                  <Building2 className="size-4 shrink-0" aria-hidden />
                  {posting.company_name}
                </span>
              )
            )}
            {posting.location && (
              <span className="inline-flex items-center gap-1.5 [overflow-wrap:anywhere]">
                <MapPin className="size-4 shrink-0" aria-hidden />
                {posting.location}
              </span>
            )}
            <span className="inline-flex items-center gap-1.5">
              <Clock className="size-4 shrink-0" aria-hidden />
              <time
                dateTime={posting.published_at ?? posting.created_at}
                title={fullDate(posting.published_at ?? posting.created_at)}
              >
                {publicationText(posting)}
              </time>
            </span>
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {canApply &&
            (posting.already_applied ? (
              <Button asChild variant="outline" className="h-11">
                <Link
                  href={
                    posting.application_id
                      ? applicationHref(posting.application_id)
                      : routes.applications
                  }
                >
                  <CircleCheck aria-hidden className="text-emerald-600" />
                  Applied · View application
                </Link>
              </Button>
            ) : (
              <Button className="h-11 px-6" onClick={() => setApplying(true)}>
                Apply with your CV
              </Button>
            ))}
          {variant === "panel" && (
            <Link
              href={routes.job(posting.id)}
              className="inline-flex min-h-11 items-center gap-1 rounded-sm text-sm font-medium text-muted-foreground underline-offset-4 outline-none hover:text-foreground hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              Open posting page
              <ArrowUpRight className="size-4" aria-hidden />
            </Link>
          )}
        </div>
        {canApply && posting.already_applied && (
          <p role="status" className="text-sm text-muted-foreground">
            You already applied to this posting.
          </p>
        )}
      </header>

      <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border bg-border text-sm sm:grid-cols-4">
        {facts.map((fact) => (
          <div key={fact.label} className="space-y-1 bg-background p-3">
            <dt className="text-xs text-muted-foreground">{fact.label}</dt>
            <dd className="font-medium [overflow-wrap:anywhere]">
              {fact.value}
            </dd>
          </div>
        ))}
      </dl>

      <section className="space-y-3">
        <Section className="text-base font-semibold">Skills</Section>
        {posting.requirements.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            This posting doesn&apos;t list specific skills. Read the job
            description below.
          </p>
        ) : (
          <Skills requirements={posting.requirements} />
        )}
      </section>

      <section className="space-y-3">
        <Section className="text-base font-semibold">How hiring works</Section>
        <HiringSteps />
      </section>

      <section className="space-y-3">
        <Section className="text-base font-semibold">Job description</Section>
        <JobDescription text={posting.raw_text} />
      </section>

      {/* Kept mounted while open: the "applied" refetch must not take the success step away. */}
      {canApply && (!posting.already_applied || applying) && (
        <ApplyDialog
          posting={posting}
          open={applying}
          onOpenChange={setApplying}
        />
      )}
    </article>
  );
}

// Required skills come first; past SKILLS_SHOWN the rest wait behind "Show all".
function Skills({ requirements }: { requirements: BoardRequirement[] }) {
  const [expanded, setExpanded] = useState(false);
  const ordered = [
    ...requirements.filter((r) => r.type === "required"),
    ...requirements.filter((r) => r.type === "preferred"),
  ];
  const shown = expanded ? ordered : ordered.slice(0, SKILLS_SHOWN);
  const groups = [
    {
      title: "Required",
      items: shown.filter((r) => r.type === "required"),
    },
    {
      title: "Nice to have",
      items: shown.filter((r) => r.type === "preferred"),
    },
  ];
  return (
    <div className="space-y-4">
      {groups.map(
        (group) =>
          group.items.length > 0 && (
            <div key={group.title} className="space-y-2">
              <p className="text-xs font-medium text-muted-foreground">
                {group.title}
              </p>
              <ul aria-label={group.title} className="flex flex-wrap gap-2">
                {group.items.map((item, i) => (
                  <li
                    key={`${item.name}-${i}`}
                    title={targetLabel(item.target)}
                    className={cn(
                      "rounded-md border px-2.5 py-1 text-sm [overflow-wrap:anywhere]",
                      item.type === "preferred" &&
                        "border-dashed text-muted-foreground",
                    )}
                  >
                    {item.name}
                  </li>
                ))}
              </ul>
            </div>
          ),
      )}
      {ordered.length > SKILLS_SHOWN && (
        <Button
          type="button"
          variant="ghost"
          className="h-11 px-2"
          aria-expanded={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "Show fewer skills" : `Show all ${ordered.length} skills`}
        </Button>
      )}
    </div>
  );
}

function JobDescription({ text }: { text: string }) {
  const id = useId();
  const [expanded, setExpanded] = useState(false);
  if (!text.trim())
    return (
      <p className="text-sm text-muted-foreground">
        The recruiter hasn&apos;t added a job description.
      </p>
    );
  const long =
    text.length > LONG_JD_CHARACTERS || text.split("\n").length > LONG_JD_LINES;
  const clamped = long && !expanded;
  return (
    <div className="space-y-2">
      {/* Recruiter text is untrusted: render it as text, never as HTML. */}
      <p
        id={id}
        className={cn(
          "text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]",
          clamped &&
            "max-h-72 overflow-hidden [mask-image:linear-gradient(to_bottom,black_70%,transparent)]",
        )}
      >
        {text}
      </p>
      {long && (
        <Button
          type="button"
          variant="ghost"
          className="h-11 px-2"
          aria-expanded={expanded}
          aria-controls={id}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "Show less" : "Show the full description"}
        </Button>
      )}
    </div>
  );
}
