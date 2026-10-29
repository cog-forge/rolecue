"use client";

import { PageHeader } from "@/components/layout/page-header";
import { ApplicationList } from "./application-list";

// Every Application on the Recruiter's own postings. Each candidate opens the Application's detail.
export function ReceivedApplications() {
  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <PageHeader
        title="Received Applications"
        description="Candidates who applied to your job postings, newest first. Search by name or email, or filter by where they are in screening."
      />
      <ApplicationList />
    </section>
  );
}
