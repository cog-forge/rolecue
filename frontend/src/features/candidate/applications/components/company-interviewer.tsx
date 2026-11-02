"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Building2 } from "lucide-react";
import { renderAvatarPortrait } from "@/components/avatar-viewer/render-portrait";
import {
  companyInterviewerKey,
  getCompanyInterviewer,
} from "../api/company-interviewer";

// This portrait has its own short-lived grant; it never calls the Recruiter's inventory API.
export function CompanyInterviewer({
  userId,
  applicationId,
}: {
  userId: string;
  applicationId: string;
}) {
  const t = useTranslations("candidate.applications.interviewer");
  const detail = useQuery({
    queryKey: companyInterviewerKey(userId, applicationId),
    queryFn: ({ signal }) => getCompanyInterviewer(applicationId, signal),
    staleTime: 0,
    gcTime: 0,
    refetchOnMount: "always",
    refetchInterval: 60_000,
    retry: false,
  });
  const [portrait, setPortrait] = useState<{
    url: string;
    accessUrl: string;
  } | null>(null);
  const grant = detail.data?.model_access;
  useEffect(() => {
    const controller = new AbortController();
    let url: string | undefined;
    if (grant && !detail.isError) {
      void renderAvatarPortrait(
        {
          url: grant.url,
          method: grant.method,
          headers: grant.headers,
          expiresAt: grant.expires_at,
        },
        controller.signal,
      )
        .then((result) => {
          if (controller.signal.aborted) {
            URL.revokeObjectURL(result);
            return;
          }
          url = result;
          setPortrait({ url: result, accessUrl: grant.url });
        })
        .catch(() => {
          if (!controller.signal.aborted) setPortrait(null);
        });
    }
    return () => {
      controller.abort();
      if (url) URL.revokeObjectURL(url);
    };
  }, [grant, detail.isError]);
  const visiblePortrait =
    portrait?.accessUrl === grant?.url ? portrait?.url : null;
  return (
    <section
      className="flex items-center gap-4 rounded-xl border bg-card p-4"
      aria-label={t("title")}
    >
      <div className="relative grid h-28 w-24 shrink-0 place-items-center overflow-hidden rounded-lg bg-muted">
        {visiblePortrait && !detail.isError ? (
          <Image
            src={visiblePortrait}
            alt=""
            fill
            onError={() => setPortrait(null)}
            unoptimized
            className="object-cover"
          />
        ) : (
          <Building2 className="size-8 text-muted-foreground" aria-hidden />
        )}
      </div>
      <div className="space-y-1 text-sm">
        <h3 className="font-semibold">{t("title")}</h3>
        {detail.data && !detail.isError && (
          <>
            <p>{detail.data.name}</p>
            <p>{t("voice", { name: detail.data.voice_name })}</p>
          </>
        )}
        <p className="text-xs text-muted-foreground">
          {visiblePortrait && !detail.isError ? t("locked") : t("fallback")}
        </p>
      </div>
    </section>
  );
}
