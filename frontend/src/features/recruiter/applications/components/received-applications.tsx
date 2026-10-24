"use client";

import { headerBandClass } from "@/features/recruiter/job-postings/components/notice";
import { cn } from "@/lib/utils";
import { ApplicationList } from "./application-list";

// Every Application on the Recruiter's own postings. Opening one (Application Detail) comes with
// KAN-129, so rows are not links yet.
export function ReceivedApplications() {
  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <header className={cn(headerBandClass, "space-y-2")}>
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Recruitment
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Received Applications
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Candidates who applied to your job postings, newest first. Search by
          name or email, or filter by where they are in screening.
        </p>
      </header>
      <ApplicationList />
    </section>
  );
}
