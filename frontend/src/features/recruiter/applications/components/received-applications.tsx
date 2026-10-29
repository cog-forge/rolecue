"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { PageHeader } from "@/components/layout/page-header";
import { cn } from "@/lib/utils";
import type { Application } from "../schemas/application-schema";
import {
  inboxHref,
  parseSelectedApplication,
} from "../utils/application-filters";
import {
  ApplicationResults,
  ApplicationToolbar,
  pushUrl,
  useApplicationPage,
  useSplitView,
} from "./application-list";
import { ReviewPanel } from "./review-panel";

// The Recruiter's review inbox: every Application on their postings, with the selected one open
// beside the list. Without a selection the first CV awaiting review opens, and a screening
// decision moves straight to the next one, so repeated screening needs no page changes.
export function ReceivedApplications() {
  const params = useSearchParams();
  const split = useSplitView();
  const { filters, query } = useApplicationPage();
  // A decided Application can drop out of a filtered page; the panel keeps showing it.
  const [pinned, setPinned] = useState<{
    id: string;
    postingId: string;
  } | null>(null);
  const [caughtUp, setCaughtUp] = useState(false);

  const items = query.data?.items ?? [];
  const requested = parseSelectedApplication(params.get("application"));
  let index = requested
    ? items.findIndex((item) => item.id === requested)
    : items.findIndex((item) => item.status === "pending");
  if (index === -1 && !requested && items.length > 0) index = 0;
  const current = index >= 0 ? items[index] : null;
  const selectedId = current?.id ?? requested;
  const selectedPosting =
    current?.job_posting_id ??
    (pinned && pinned.id === requested ? pinned.postingId : null) ??
    (requested ? filters.posting : null);

  const select = (application: Application) => {
    setCaughtUp(false);
    pushUrl(inboxHref(filters, application.id));
  };
  const nextPending = () => {
    for (let step = 1; step <= items.length; step++) {
      const candidate = items[(index + step) % items.length];
      if (candidate.status === "pending" && candidate.id !== selectedId)
        return candidate;
    }
    return null;
  };
  const panelOpen = split && !!selectedId && !!selectedPosting;

  return (
    <section className="mx-auto w-full max-w-7xl space-y-6">
      <PageHeader
        title="Received Applications"
        description="Review each CV beside the list. After you approve or reject one, the next CV waiting for review opens."
      />
      <ApplicationToolbar />
      <div
        className={cn(
          "grid items-start gap-6",
          panelOpen && "lg:grid-cols-[minmax(0,24rem)_minmax(0,1fr)]",
        )}
      >
        <ApplicationResults
          selectedId={panelOpen ? selectedId : null}
          onSelect={select}
        />
        {panelOpen && selectedId && selectedPosting && (
          // The panel scrolls on its own so the CV and decision stay reachable beside a long list.
          <div className="lg:sticky lg:top-4 lg:max-h-[calc(100dvh-2rem)] lg:overflow-y-auto lg:overscroll-contain">
            <ReviewPanel
              key={selectedId}
              postingId={selectedPosting}
              applicationId={selectedId}
              position={index >= 0 ? { index, count: items.length } : null}
              onPrevious={
                index > 0 ? () => select(items[index - 1]) : undefined
              }
              onNext={
                index >= 0 && index < items.length - 1
                  ? () => select(items[index + 1])
                  : undefined
              }
              caughtUp={caughtUp}
              onDecided={() => {
                const next = nextPending();
                if (next) {
                  select(next);
                  return;
                }
                setPinned({ id: selectedId, postingId: selectedPosting });
                setCaughtUp(true);
                pushUrl(inboxHref(filters, selectedId));
              }}
            />
          </div>
        )}
      </div>
    </section>
  );
}
