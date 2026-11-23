"use client";
import type { UseQueryResult } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  useSaveSetup,
  type SavedSetup,
  type SetupTarget,
} from "../api/saved-setup";
import { EnvironmentPicker } from "./environment-picker";
import { SaveStatus } from "./save-status";
import { SetupBlockers } from "./setup-blockers";

// Recruitment: the recruiter locked the avatar and voice, so the Candidate's only choice is the
// room. It is saved to the setup API on every pick (the body carries the environment only), and the
// answer's blockers say what still stops the interview from starting.
export function RecruitmentEnvironment({
  applicationId,
  setup,
}: {
  applicationId: string;
  setup: UseQueryResult<SavedSetup>;
}) {
  const t = useTranslations("candidate.setup");
  const user = useSessionUser();
  const target: SetupTarget = { mode: "recruitment", applicationId };
  const save = useSaveSetup(user.id, target);
  const view = setup.data;
  if (!view)
    return setup.isError ? (
      <ErrorState
        description={t("blockers.error")}
        onRetry={() => void setup.refetch()}
        retryLabel={t("tryAgain")}
        isRetrying={setup.isFetching}
      ></ErrorState>
    ) : (
      <div role="status" className="space-y-3">
        <Skeleton className="h-40 rounded-xl" />
        <span className="sr-only">{t("blockers.checking")}</span>
      </div>
    );
  const choose = (environment: string) =>
    save.mutate({ avatarId: null, voiceId: null, environment });
  const state = save.isPending
    ? "saving"
    : save.isError
      ? "error"
      : save.isSuccess
        ? "saved"
        : "idle";
  return (
    <div className="space-y-4">
      <SetupBlockers blockers={view.blockers} target={target} params={{}} />
      <section className="rounded-xl border bg-card p-5 sm:p-6">
        <EnvironmentPicker value={view.environment} onChange={choose} />
        <SaveStatus
          state={state}
          onRetry={() =>
            save.variables
              ? save.mutate(save.variables)
              : choose(view.environment ?? "")
          }
        />
      </section>
    </div>
  );
}
