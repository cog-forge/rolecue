"use client";

import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { applicationKeys } from "../api/applications";
import type { ApplicationDetail } from "../schemas/application-detail-schema";

// The API's CV links expire after five minutes, so the detail is reloaded a minute before that.
const REFRESH_BEFORE_EXPIRY_MS = 60 * 1000;
// Never reload more often than this, even if the expiry is already close or the clocks disagree.
const MIN_REFRESH_DELAY_MS = 30 * 1000;

// Opens the Application's CV in a new tab. The link comes from the detail response, so the
// refresh before expiry reloads the detail rather than a separate link.
export function CvLink({
  postingId,
  applicationId,
  cv,
}: {
  postingId: string;
  applicationId: string;
  cv: ApplicationDetail["cv"];
}) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const expiresAt = cv?.expires_at;

  useEffect(() => {
    if (!expiresAt) return;
    const delay = Math.max(
      MIN_REFRESH_DELAY_MS,
      Date.parse(expiresAt) - Date.now() - REFRESH_BEFORE_EXPIRY_MS,
    );
    const timer = window.setTimeout(() => {
      void cache.invalidateQueries({
        queryKey: applicationKeys.detail(user.id, postingId, applicationId),
      });
    }, delay);
    return () => window.clearTimeout(timer);
  }, [cache, user.id, postingId, applicationId, expiresAt]);

  if (!cv)
    return <p className="text-sm text-muted-foreground">No CV stored.</p>;
  return (
    <Button asChild variant="outline" className="h-11">
      <a href={cv.url} target="_blank" rel="noopener noreferrer">
        <ExternalLink aria-hidden />
        Open the CV
        <span className="sr-only"> (opens in a new tab)</span>
      </a>
    </Button>
  );
}
