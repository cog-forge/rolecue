"use client";

import { useQuery } from "@tanstack/react-query";
import { ExternalLink, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getDocumentLink, jobPostingKeys } from "../api/job-postings";

// The API's PDF links expire after five minutes, so the link is refreshed before that.
const LINK_REFRESH_MS = 4 * 60 * 1000;

// Opens the posting's JD PDF in a new tab. The link is fetched ahead of the click, so the browser
// treats the new tab as a direct result of it.
export function JdDocumentLink({ postingId }: { postingId: string }) {
  const user = useSessionUser();
  const link = useQuery({
    queryKey: jobPostingKeys.document(user.id, postingId),
    queryFn: ({ signal }) => getDocumentLink(postingId, signal),
    staleTime: LINK_REFRESH_MS,
    refetchInterval: LINK_REFRESH_MS,
    retry: false,
  });

  if (link.isPending)
    return (
      <Button variant="outline" className="h-11" disabled aria-busy>
        <LoaderCircle
          className="size-4 animate-spin motion-reduce:animate-none"
          aria-hidden
        />
        Preparing the PDF…
      </Button>
    );
  if (link.isError)
    return (
      <div role="alert" className="flex flex-wrap items-center gap-2 text-sm">
        <span className="text-destructive">We couldn&apos;t open the PDF.</span>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          onClick={() => void link.refetch()}
        >
          Try again
        </Button>
      </div>
    );
  return (
    <Button asChild variant="outline" className="h-11">
      <a href={link.data.url} target="_blank" rel="noopener noreferrer">
        <ExternalLink aria-hidden />
        Open the PDF
        <span className="sr-only"> (job description, opens in a new tab)</span>
      </a>
    </Button>
  );
}
