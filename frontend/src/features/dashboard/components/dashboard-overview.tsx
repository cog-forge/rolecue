"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowUpRight } from "lucide-react";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { BentoGrid } from "@/components/ui/bento-grid";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  dashboardKeys,
  listPracticeInterviews,
  PRACTICE_WINDOW_SIZE,
} from "../api/dashboard";
import { ApplicationsCard } from "./applications-card";
import { PracticeActivityCard } from "./practice-activity-card";
import { PracticeProgressCard } from "./practice-progress-card";
import { PracticeFocusCard } from "./practice-focus-card";
import { NextStepCard } from "./next-step-card";
import { RoleProfilesCard } from "./role-profiles-card";
import { WalletCard } from "./wallet-card";

export function DashboardOverview() {
  const t = useTranslations("candidate.dashboard");
  const user = useSessionUser();
  const practice = useQuery({
    queryKey: dashboardKeys.practice(user.id),
    queryFn: ({ signal }) =>
      listPracticeInterviews(PRACTICE_WINDOW_SIZE, signal),
    retry: false,
  });
  return (
    <div className="mx-auto w-full max-w-6xl space-y-6 pb-6">
      <PageHeader
        title={t("heading")}
        description={t("description")}
        actions={
          <Button asChild className={pillClass}>
            <Link href={routes.interviews.new.jobDescription}>
              {t("practice.start")}
              <ArrowUpRight aria-hidden className="size-4" />
            </Link>
          </Button>
        }
      />
      <BentoGrid>
        <NextStepCard
          practice={practice}
          className="md:col-span-6 xl:col-span-4 xl:col-start-9 xl:row-start-1"
        />
        <PracticeProgressCard
          query={practice}
          className="md:col-span-6 xl:col-span-8 xl:col-start-1 xl:row-span-2 xl:row-start-1"
        />
        <WalletCard className="md:col-span-6 xl:col-span-4 xl:col-start-9 xl:row-start-2" />
        <PracticeActivityCard
          query={practice}
          className="md:col-span-6 xl:col-span-7"
        />
        <PracticeFocusCard
          query={practice}
          className="md:col-span-6 xl:col-span-5"
        />
        <RoleProfilesCard className="md:col-span-3 xl:col-span-5" />
        <ApplicationsCard className="md:col-span-3 xl:col-span-7" />
      </BentoGrid>
    </div>
  );
}
