"use client";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Target } from "lucide-react";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getRoadmap, roadmapKeys } from "../api/roadmap";

export function PracticeFocus({
  profileId,
  focusItemId,
  requestedFocusId,
  resumed = false,
}: {
  profileId: string;
  focusItemId?: string | null;
  requestedFocusId?: string;
  resumed?: boolean;
}) {
  const t = useTranslations("candidate.roadmap");
  const user = useSessionUser();
  const roadmap = useQuery({
    queryKey: roadmapKeys.detail(user.id, profileId),
    queryFn: ({ signal }) => getRoadmap(profileId, signal),
    enabled: Boolean(focusItemId),
    retry: false,
    staleTime: 0,
  });
  const item = roadmap.data?.items.find((topic) => topic.id === focusItemId);
  const differs =
    resumed && (focusItemId ?? null) !== (requestedFocusId ?? null);
  return (
    <div
      className="space-y-2 rounded-lg border bg-muted/20 p-4 text-sm"
      role="status"
    >
      <p className="flex items-start gap-2 font-medium">
        <Target
          className="mt-0.5 size-4 shrink-0 text-rolecue-brand"
          aria-hidden="true"
        />
        {!focusItemId
          ? t("generalFocus")
          : item
            ? t("topicFocus", { title: item.title })
            : t(roadmap.isPending ? "loadingFocus" : "unknownFocus")}
      </p>
      {differs && (
        <p className="text-muted-foreground">{t("resumeDifferent")}</p>
      )}
      {focusItemId && !roadmap.isPending && !item && (
        <Link
          className="inline-flex min-h-11 items-center underline"
          href={routes.roleProfiles.roadmap(profileId)}
        >
          {t("chooseAnotherTopic")}
        </Link>
      )}
    </div>
  );
}
