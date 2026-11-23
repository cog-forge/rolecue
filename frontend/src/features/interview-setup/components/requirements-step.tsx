"use client";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowRight, Pencil } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  getRoleProfile,
  roleProfileKeys,
} from "@/features/role-profile/api/requests";
import {
  proficiencyLabel,
  requirementLabel,
} from "@/features/role-profile/lib/labels";
import {
  isCustomTarget,
  isKnownTarget,
} from "@/features/role-profile/lib/targets";
import { setupHref } from "../lib/setup-params";
import { SetupHeader } from "./setup-header";

// Step 2: the requirements of the selected Role Profile, read-only. They are edited on the profile
// itself, so this step only shows them and links there, keeping the wizard around the reader.
export function RequirementsStep({
  roleProfileId,
  focusItemId,
}: {
  roleProfileId: string;
  focusItemId?: string;
}) {
  const t = useTranslations("candidate.setup.requirements");
  const setup = useTranslations("candidate.setup");
  const labels = useTranslations("candidate.roleProfiles.labels");
  const user = useSessionUser();
  const profile = useQuery({
    queryKey: roleProfileKeys.detail(user.id, roleProfileId),
    queryFn: ({ signal }) => getRoleProfile(roleProfileId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const header = (
    <SetupHeader
      context={setup("context")}
      title={t("title")}
      description={t("intro")}
    />
  );
  if (profile.isPending)
    return (
      <section className="space-y-8" aria-busy="true">
        {header}
        <div role="status" className="space-y-3">
          {[0, 1, 2].map((row) => (
            <Skeleton key={row} className="h-16 rounded-xl" />
          ))}
          <span className="sr-only">{t("loading")}</span>
        </div>
      </section>
    );
  if (profile.isError)
    return (
      <section className="space-y-8">
        {header}
        <ErrorState
          description={t("loadFailed")}
          onRetry={() => void profile.refetch()}
          retryLabel={t("tryAgain")}
          isRetrying={profile.isFetching}
        />
      </section>
    );
  const { requirements, title } = profile.data;
  const confirmed = profile.data.status === "confirmed";
  const edit = routes.roleProfiles.detail(roleProfileId);
  return (
    <section className="space-y-8">
      {header}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold [overflow-wrap:anywhere]">
          {title}{" "}
          <span className="ml-2 text-sm font-normal text-muted-foreground">
            {requirements.length}
          </span>
        </h2>
        {requirements.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("empty")}</p>
        ) : (
          <ul className="divide-y rounded-xl border bg-card">
            {requirements.map((item) => (
              <li
                key={`${item.kind}-${item.name}`}
                className="flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-3"
              >
                <div className="min-w-0">
                  <p className="font-medium break-words">{item.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {requirementLabel(labels, item)}
                  </p>
                </div>
                <Badge variant="secondary">
                  {isKnownTarget(item.target)
                    ? proficiencyLabel(labels, item.target)
                    : isCustomTarget(item)
                      ? labels("savedTarget")
                      : proficiencyLabel(labels, "Unspecified")}
                </Badge>
              </li>
            ))}
          </ul>
        )}
      </div>
      {!confirmed && (
        <p role="status" className="text-sm text-muted-foreground">
          {t("notConfirmed")}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <Button
          asChild
          variant="outline"
          className="h-auto min-h-11 py-2 whitespace-normal"
        >
          <Link href={edit}>
            <Pencil aria-hidden="true" /> {t("edit")}
          </Link>
        </Button>
        {confirmed && (
          <Button asChild className="h-auto min-h-11 py-2 whitespace-normal">
            <Link
              href={setupHref(routes.interviews.new.setup, {
                roleProfileId,
                focusItemId,
              })}
            >
              {t("continue")} <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        )}
      </div>
    </section>
  );
}
