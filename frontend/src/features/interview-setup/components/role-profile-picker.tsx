"use client";

import { useId } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { ArrowRight, History, Plus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import type { RoleSummary } from "@/features/role-profile/api/contract";
import { roleProfileKeys } from "@/features/role-profile/api/keys";
import {
  getRoleProfile,
  listRoleProfiles,
  ROLE_PROFILES_PAGE_SIZE,
} from "@/features/role-profile/api/requests";
import { useUnfinishedPractice } from "../api/setup-catalog";
import { setupHref } from "../lib/setup-params";
import { PracticeProgress, ResumeCallout } from "./resume-callout";
import { SetupHeader } from "./setup-header";
import { apiErrorMessage } from "@/lib/i18n/api-errors";
import { useFormatters } from "@/lib/i18n/use-formatters";

// Step 3: the confirmed Role Profile the interview is built from. The choice lives in the URL.
export function RoleProfilePicker({
  roleProfileId,
}: {
  roleProfileId?: string;
}) {
  const t = useTranslations("candidate.setup");
  const errors = useTranslations("errors");
  const fmt = useFormatters();
  const user = useSessionUser();
  const router = useRouter();
  const headingId = useId();
  const profiles = useInfiniteQuery({
    queryKey: [...roleProfileKeys.lists(user.id), "setup"],
    queryFn: ({ pageParam, signal }) => listRoleProfiles(pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: ({ pagination }) =>
      pagination.offset + ROLE_PROFILES_PAGE_SIZE < pagination.total
        ? pagination.offset + ROLE_PROFILES_PAGE_SIZE
        : undefined,
    refetchOnMount: "always",
    retry: false,
  });
  const unfinished = useUnfinishedPractice(user.id);
  const loaded = profiles.data?.pages.flatMap((page) => page.items) ?? [];
  const listed = roleProfileId
    ? loaded.find((profile) => profile.id === roleProfileId)
    : undefined;
  // A deep link to a profile on a later page (or not listed) is read on its own.
  const deepLink = useQuery({
    queryKey: roleProfileKeys.detail(user.id, roleProfileId ?? "missing"),
    queryFn: ({ signal }) => getRoleProfile(roleProfileId ?? "", signal),
    enabled: Boolean(roleProfileId) && profiles.isSuccess && !listed,
    retry: false,
  });
  const linked: RoleSummary | undefined =
    listed ??
    (deepLink.data && {
      id: deepLink.data.id,
      title: deepLink.data.title,
      seniorityLevel: deepLink.data.seniorityLevel,
      status: deepLink.data.status,
      createdAt: deepLink.data.createdAt,
    });

  if (profiles.isPending || (roleProfileId && !listed && deepLink.isFetching))
    return (
      <section className="space-y-8" aria-busy="true">
        <SetupHeader
          context={t("context")}
          title={t("roleProfile.title")}
          description={t("roleProfile.loading.description")}
        />
        <div role="status" className="space-y-3">
          {[0, 1, 2].map((row) => (
            <Skeleton key={row} className="h-[4.5rem] rounded-xl" />
          ))}
          <span className="sr-only">{t("roleProfile.loading.status")}</span>
        </div>
      </section>
    );

  if (profiles.isError)
    return (
      <section className="space-y-8">
        <SetupHeader
          context={t("context")}
          title={t("roleProfile.title")}
          description={t("roleProfile.intro")}
        />
        <div
          role="alert"
          className="space-y-4 rounded-xl border border-destructive/30 bg-destructive/5 p-5"
        >
          <p className="text-sm">
            {apiErrorMessage(
              errors,
              profiles.error,
              t("roleProfile.loadFailed"),
            )}
          </p>
          <Button
            variant="outline"
            className="h-auto min-h-11 whitespace-normal py-2 bg-background"
            disabled={profiles.isFetching}
            onClick={() => void profiles.refetch()}
          >
            {t("tryAgain")}
          </Button>
        </div>
      </section>
    );

  if (linked && linked.status !== "confirmed")
    return (
      <section className="space-y-6">
        <SetupHeader
          context={t("context")}
          title={t("roleProfile.confirmFirst.title")}
          description={t("roleProfile.confirmFirst.description")}
        />
        <Button asChild className="h-auto min-h-11 whitespace-normal py-2">
          <Link href={routes.roleProfiles.detail(linked.id)}>
            {t("roleProfile.confirmFirst.action")}
          </Link>
        </Button>
      </section>
    );

  const confirmed = loaded.filter((profile) => profile.status === "confirmed");
  if (linked && !listed) confirmed.unshift(linked);
  const selected = linked?.status === "confirmed" ? linked : undefined;
  const linkProblem =
    roleProfileId && !linked && deepLink.isError
      ? isAxiosError(deepLink.error) && deepLink.error.response?.status === 404
        ? t("roleProfile.notFound")
        : apiErrorMessage(errors, deepLink.error, t("roleProfile.loadFailed"))
      : null;

  if (confirmed.length === 0 && !profiles.hasNextPage)
    return (
      <section className="space-y-6">
        <SetupHeader
          context={t("context")}
          title={t("roleProfile.empty.title")}
          description={t("roleProfile.empty.description")}
        />
        <div className="flex flex-wrap gap-3">
          <Button asChild className="h-auto min-h-11 whitespace-normal py-2">
            <Link href={routes.roleProfiles.new}>
              <Plus aria-hidden="true" /> {t("roleProfile.add")}
            </Link>
          </Button>
          <Button
            asChild
            variant="outline"
            className="h-auto min-h-11 whitespace-normal py-2"
          >
            <Link href={routes.roleProfiles.root}>
              {t("roleProfile.empty.openAll")}
            </Link>
          </Button>
        </div>
      </section>
    );

  const resume = selected ? unfinished.data?.get(selected.id) : undefined;

  return (
    <section className="space-y-8">
      <SetupHeader
        context={t("context")}
        title={t("roleProfile.title")}
        description={t("roleProfile.description")}
      />

      {linkProblem && (
        <p
          role="alert"
          className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm"
        >
          {linkProblem}
        </p>
      )}

      <div className="space-y-3">
        <h2 id={headingId} className="text-sm font-medium">
          {t("roleProfile.listLabel")}
        </h2>
        <RadioGroup
          aria-labelledby={headingId}
          value={selected?.id ?? ""}
          onValueChange={(id) =>
            router.replace(
              setupHref(routes.interviews.new.setup, { roleProfileId: id }),
              { scroll: false },
            )
          }
          className="gap-3"
        >
          {confirmed.map((profile) => {
            const hasUnfinished = unfinished.data?.has(profile.id) ?? false;
            const inputId = `${headingId}-${profile.id}`;
            return (
              <label
                key={profile.id}
                htmlFor={inputId}
                className="group flex min-h-[4.5rem] cursor-pointer items-center gap-4 rounded-xl border bg-card p-4 transition-[border-color,box-shadow] duration-(--duration-fast) ease-(--ease-smooth-out) hover:border-foreground/30 has-focus-visible:ring-3 has-focus-visible:ring-ring/50 has-data-[state=checked]:border-foreground has-data-[state=checked]:shadow-[inset_0_0_0_1px_var(--foreground)] motion-reduce:transition-none"
              >
                <RadioGroupItem id={inputId} value={profile.id} />
                <span className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
                  <span className="min-w-0">
                    <span className="block font-semibold [overflow-wrap:anywhere]">
                      {profile.title}
                    </span>
                    <span className="mt-0.5 block text-sm text-muted-foreground">
                      <span>
                        {profile.seniorityLevel
                          ? t(`roleProfile.seniority.${profile.seniorityLevel}`)
                          : t("roleProfile.seniority.notSet")}
                      </span>
                      {" · "}
                      {t.rich("roleProfile.created", {
                        date: fmt.date(profile.createdAt, "dayMonthYear"),
                        time: (chunks) => (
                          <time dateTime={profile.createdAt}>{chunks}</time>
                        ),
                      })}
                    </span>
                  </span>
                  {hasUnfinished && (
                    <Badge variant="secondary" className="h-6 gap-1.5">
                      <History aria-hidden="true" />
                      {t("roleProfile.unfinishedBadge")}
                    </Badge>
                  )}
                </span>
              </label>
            );
          })}
        </RadioGroup>
        {profiles.hasNextPage && (
          <Button
            variant="ghost"
            className="h-auto min-h-11 whitespace-normal py-2"
            disabled={profiles.isFetchingNextPage}
            onClick={() => void profiles.fetchNextPage()}
          >
            {profiles.isFetchingNextPage
              ? t("roleProfile.loadingMore")
              : t("roleProfile.loadMore")}
          </Button>
        )}
      </div>

      {resume ? (
        <ResumeCallout
          title={t("resume.title")}
          details={<PracticeProgress session={resume} />}
          note={t("resume.note")}
        >
          <Button
            asChild
            className="h-auto min-h-11 whitespace-normal py-2 w-full sm:w-auto"
          >
            <Link
              href={setupHref(routes.interviews.new.preflight, {
                roleProfileId: resume.roleProfileId,
              })}
            >
              {t("resume.action")} <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        </ResumeCallout>
      ) : (
        <div className="flex flex-col-reverse gap-3 border-t pt-6 sm:flex-row sm:items-center sm:justify-between">
          <Button
            asChild
            variant="ghost"
            className="h-auto min-h-11 whitespace-normal py-2"
          >
            <Link href={routes.roleProfiles.new}>
              <Plus aria-hidden="true" /> {t("roleProfile.add")}
            </Link>
          </Button>
          <div className="flex flex-col gap-2 sm:items-end">
            {selected && !unfinished.isPending ? (
              <Button
                asChild
                className="h-auto min-h-11 whitespace-normal py-2"
              >
                <Link
                  href={setupHref(routes.interviews.new.interviewer, {
                    roleProfileId: selected.id,
                  })}
                >
                  {t("roleProfile.continue")} <ArrowRight aria-hidden="true" />
                </Link>
              </Button>
            ) : (
              <Button
                className="h-auto min-h-11 whitespace-normal py-2"
                disabled
                aria-describedby={`${headingId}-hint`}
              >
                {t("roleProfile.continue")} <ArrowRight aria-hidden="true" />
              </Button>
            )}
            {(!selected || unfinished.isPending) && (
              <p
                id={`${headingId}-hint`}
                role="status"
                className="text-xs text-muted-foreground"
              >
                {selected
                  ? t("roleProfile.hint.checking")
                  : t("roleProfile.hint.choose")}
              </p>
            )}
          </div>
        </div>
      )}
    </section>
  );
}
