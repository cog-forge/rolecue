import { Suspense } from "react";
import { CandidateApplicationList } from "@/features/candidate/applications/components/application-list";

// The list reads its page from the URL on the client, so paging does not re-render this route.
export default function Page() {
  return (
    <section className="mx-auto w-full max-w-4xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Candidate
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          My Applications
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Every CV you have sent, with where it stands now, newest first.
        </p>
      </header>
      <Suspense>
        <CandidateApplicationList />
      </Suspense>
    </section>
  );
}
