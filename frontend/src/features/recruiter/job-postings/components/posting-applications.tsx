"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  applicationKeys,
  listPostingApplications,
} from "@/features/recruiter/applications/api/applications";
import {
  defaultApplicationFilters,
  type ApplicationFilters,
} from "@/features/recruiter/applications/utils/application-filters";
import type { JobPosting } from "../schemas/job-posting-schema";
import { postingInboxHref, takesApplications } from "../utils/posting-guidance";
import { pillClass } from "@/components/layout/page-header";

const awaitingFilters: ApplicationFilters = {
  ...defaultApplicationFilters,
  status: "pending",
};

// A short summary of the posting's applications. Screening happens in the review inbox, so this
// only says how many there are and how many wait, and links there.
export function PostingApplications({ posting }: { posting: JobPosting }) {
  const t = useTranslations("recruiter.postings.applications");
  const user = useSessionUser();
  const enabled = takesApplications(posting.status);
  // Same key as the capacity panel's count, so the two share one request.
  const all = useQuery({
    queryKey: applicationKeys.posting(
      user.id,
      posting.id,
      defaultApplicationFilters,
    ),
    queryFn: ({ signal }) =>
      listPostingApplications(posting.id, defaultApplicationFilters, signal),
    enabled,
    retry: false,
  });
  const awaiting = useQuery({
    queryKey: applicationKeys.posting(user.id, posting.id, awaitingFilters),
    queryFn: ({ signal }) =>
      listPostingApplications(posting.id, awaitingFilters, signal),
    enabled,
    retry: false,
  });

  const total = all.data?.pagination.total;
  const waiting = awaiting.data?.pagination.total;
  return (
    <section
      aria-labelledby="applications-heading"
      className="space-y-3 border-t pt-8"
    >
      <h2 id="applications-heading" className="text-base font-semibold">
        {t("title")}
      </h2>
      {!enabled ? (
        <p className="text-sm text-muted-foreground">{t("notOpen")}</p>
      ) : all.isPending || awaiting.isPending ? (
        <div role="status">
          <Skeleton className="h-10 w-72 rounded-md" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : total === undefined || waiting === undefined ? (
        <p role="alert" className="text-sm text-muted-foreground">
          {t.rich("loadFailed", {
            inbox: (chunks) => (
              <Link
                href={postingInboxHref(posting.id)}
                className="font-medium underline underline-offset-4"
              >
                {chunks}
              </Link>
            ),
          })}
        </p>
      ) : (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm">
            {t.rich("summary", {
              total,
              waiting,
              count: (chunks) => (
                <span className="font-medium tabular-nums">{chunks}</span>
              ),
            })}
          </p>
          <Button
            asChild
            variant={waiting > 0 ? "default" : "outline"}
            className={pillClass}
          >
            <Link
              href={postingInboxHref(
                posting.id,
                waiting > 0 ? "pending" : "all",
              )}
            >
              {waiting > 0 ? t("reviewWaiting", { waiting }) : t("open")}
            </Link>
          </Button>
        </div>
      )}
    </section>
  );
}
