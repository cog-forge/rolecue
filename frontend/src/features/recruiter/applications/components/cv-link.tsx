"use client";

import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { applicationKeys } from "../api/applications";
import type { ApplicationDetail } from "../schemas/application-detail-schema";

// The API's CV links expire after five minutes, so the detail is reloaded a minute before that.
const REFRESH_BEFORE_EXPIRY_MS = 60 * 1000;
// Never reload more often than this, even if the expiry is already close or the clocks disagree.
const MAX_TIMER_DELAY_MS = 2 ** 31 - 1;
const MIN_REFRESH_DELAY_MS = 30 * 1000;

type CvProps = {
  postingId: string;
  applicationId: string;
  cv: ApplicationDetail["cv"];
};

// The link comes from the detail response, so the refresh before expiry reloads the detail rather
// than a separate link.
function useCvRefresh({ postingId, applicationId, cv }: CvProps) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const expiresAt = cv?.expires_at;

  useEffect(() => {
    if (!expiresAt) return;
    // setTimeout wraps around past 2^31-1 ms, which would refetch in a tight loop.
    const delay = Math.min(
      MAX_TIMER_DELAY_MS,
      Math.max(
        MIN_REFRESH_DELAY_MS,
        Date.parse(expiresAt) - Date.now() - REFRESH_BEFORE_EXPIRY_MS,
      ),
    );
    const timer = window.setTimeout(() => {
      void cache.invalidateQueries({
        queryKey: applicationKeys.detail(user.id, postingId, applicationId),
      });
    }, delay);
    return () => window.clearTimeout(timer);
  }, [cache, user.id, postingId, applicationId, expiresAt]);
}

function OpenCv({ url }: { url: string }) {
  return (
    <Button asChild variant="outline" className="h-11">
      <a href={url} target="_blank" rel="noopener noreferrer">
        <ExternalLink aria-hidden />
        Open the CV
        <span className="sr-only"> (opens in a new tab)</span>
      </a>
    </Button>
  );
}

// Opens the Application's CV in a new tab.
export function CvLink(props: CvProps) {
  useCvRefresh(props);
  if (!props.cv)
    return <p className="text-sm text-muted-foreground">No CV stored.</p>;
  return <OpenCv url={props.cv.url} />;
}

// Shows the CV inside the page so screening needs no tab switching. The frame keeps the link it
// first loaded: a loaded PDF stays readable after the link expires, and swapping in the refreshed
// link would reload it and lose the reader's place. The new-tab link always uses the fresh one.
// Render it with key={applicationId} so another Application starts a new frame.
export function CvPreview({
  candidateName,
  ...props
}: CvProps & { candidateName: string }) {
  useCvRefresh(props);
  const [frameUrl] = useState(props.cv?.url ?? null);
  if (!props.cv)
    return <p className="text-sm text-muted-foreground">No CV stored.</p>;
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">
          If the preview stays blank, open the CV in a new tab.
        </p>
        <OpenCv url={props.cv.url} />
      </div>
      <iframe
        src={frameUrl ?? props.cv.url}
        title={`CV of ${candidateName}`}
        referrerPolicy="no-referrer"
        className="h-[72dvh] min-h-[28rem] w-full rounded-lg border bg-muted/30"
      />
    </div>
  );
}
