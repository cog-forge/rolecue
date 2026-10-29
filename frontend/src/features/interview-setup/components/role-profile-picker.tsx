"use client";

import { useId } from "react";
import Link from "next/link";
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
  roleError,
} from "@/features/role-profile/api/requests";
import { useUnfinishedPractice } from "../api/setup-catalog";
import { setupHref } from "../lib/setup-params";
import { PracticeProgress, ResumeCallout } from "./resume-callout";
import { SetupHeader } from "./setup-header";
import { useFormatters } from "@/lib/i18n/use-formatters";

// Step 3: the confirmed Role Profile the interview is built from. The choice lives in the URL.
export function RoleProfilePicker({
  roleProfileId,
}: {
  roleProfileId?: string;
}) {
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
          context="Interview setup"
          title="Choose a Role Profile"
          description="Checking your saved Role Profiles…"
        />
        <div role="status" className="space-y-3">
          {[0, 1, 2].map((row) => (
            <Skeleton key={row} className="h-[4.5rem] rounded-xl" />
          ))}
          <span className="sr-only">Loading your Role Profiles…</span>
        </div>
      </section>
    );

  if (profiles.isError)
    return (
      <section className="space-y-8">
        <SetupHeader
          context="Interview setup"
          title="Choose a Role Profile"
          description="Your interview is built from a confirmed Role Profile."
        />
        <div
          role="alert"
          className="space-y-4 rounded-xl border border-destructive/30 bg-destructive/5 p-5"
        >
          <p className="text-sm">{roleError(profiles.error)}</p>
          <Button
            variant="outline"
            className="h-11 bg-background"
            disabled={profiles.isFetching}
            onClick={() => void profiles.refetch()}
          >
            Try again
          </Button>
        </div>
      </section>
    );

  if (linked && linked.status !== "confirmed")
    return (
      <section className="space-y-6">
        <SetupHeader
          context="Interview setup"
          title="Confirm this role first."
          description="Review and confirm your requirements and seniority before setup."
        />
        <Button asChild className="h-11">
          <Link href={routes.roleProfiles.detail(linked.id)}>
            Return to Role Profile
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
        ? "We couldn't find that Role Profile. Choose one below."
        : roleError(deepLink.error)
      : null;

  if (confirmed.length === 0 && !profiles.hasNextPage)
    return (
      <section className="space-y-6">
        <SetupHeader
          context="Interview setup"
          title="Choose your Role Profile first."
          description="Review and confirm a role before setting up your interview."
        />
        <div className="flex flex-wrap gap-3">
          <Button asChild className="h-11">
            <Link href={routes.roleProfiles.new}>
              <Plus aria-hidden="true" /> Add a Role Profile
            </Link>
          </Button>
          <Button asChild variant="outline" className="h-11">
            <Link href={routes.roleProfiles.root}>Open Role Profiles</Link>
          </Button>
        </div>
      </section>
    );

  const resume = selected ? unfinished.data?.get(selected.id) : undefined;

  return (
    <section className="space-y-8">
      <SetupHeader
        context="Interview setup"
        title="Choose a Role Profile"
        description="Your interview asks about the requirements of the confirmed role you pick. Its questions stay hidden until you start."
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
          Confirmed Role Profiles
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
                      <span className="capitalize">
                        {profile.seniorityLevel ?? "Seniority not set"}
                      </span>
                      {" · Created "}
                      <time dateTime={profile.createdAt}>
                        {fmt.date(profile.createdAt, "dayMonthYear")}
                      </time>
                    </span>
                  </span>
                  {hasUnfinished && (
                    <Badge variant="secondary" className="h-6 gap-1.5">
                      <History aria-hidden="true" />
                      Unfinished interview
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
            className="h-11"
            disabled={profiles.isFetchingNextPage}
            onClick={() => void profiles.fetchNextPage()}
          >
            {profiles.isFetchingNextPage
              ? "Loading more…"
              : "Show more Role Profiles"}
          </Button>
        )}
      </div>

      {resume ? (
        <ResumeCallout
          title="You have an unfinished interview for this role"
          details={<PracticeProgress session={resume} />}
          note="Resuming is free and keeps its interviewer and voice."
        >
          <Button asChild className="h-11 w-full sm:w-auto">
            <Link
              href={setupHref(routes.interviews.new.preflight, {
                roleProfileId: resume.roleProfileId,
              })}
            >
              Resume interview <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        </ResumeCallout>
      ) : (
        <div className="flex flex-col-reverse gap-3 border-t pt-6 sm:flex-row sm:items-center sm:justify-between">
          <Button asChild variant="ghost" className="h-11">
            <Link href={routes.roleProfiles.new}>
              <Plus aria-hidden="true" /> Add a Role Profile
            </Link>
          </Button>
          <div className="flex flex-col gap-2 sm:items-end">
            {selected && !unfinished.isPending ? (
              <Button asChild className="h-11">
                <Link
                  href={setupHref(routes.interviews.new.interviewer, {
                    roleProfileId: selected.id,
                  })}
                >
                  Continue to interviewer <ArrowRight aria-hidden="true" />
                </Link>
              </Button>
            ) : (
              <Button
                className="h-11"
                disabled
                aria-describedby={`${headingId}-hint`}
              >
                Continue to interviewer <ArrowRight aria-hidden="true" />
              </Button>
            )}
            {(!selected || unfinished.isPending) && (
              <p
                id={`${headingId}-hint`}
                role="status"
                className="text-xs text-muted-foreground"
              >
                {selected
                  ? "Checking for an unfinished interview…"
                  : "Choose a Role Profile to continue."}
              </p>
            )}
          </div>
        </div>
      )}
    </section>
  );
}
