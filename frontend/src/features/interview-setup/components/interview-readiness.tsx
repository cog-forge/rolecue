"use client";
import { getRoadmap, roadmapKeys } from "@/features/roadmap/api/roadmap";
import { PracticeFocus } from "@/features/roadmap/components/practice-focus";
import { ErrorState } from "@/components/ui/error-state";

import {
  Suspense,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { avatarKeys, getAvatar } from "@/features/avatar/api/inventory";
import { syncDepositStatus, walletKeys } from "@/features/billing/api/wallet";
import {
  PaymentStatusBanner,
  type PaymentReturnEvent,
} from "@/features/billing/components/payment-status-banner";
import {
  candidateApplicationKeys,
  getCandidateApplication,
} from "@/features/candidate/applications/api/applications";
import {
  formatDeadline,
  timeLeftText,
} from "@/features/candidate/applications/utils/application-text";
import {
  getRoleProfile,
  roleProfileKeys,
} from "@/features/role-profile/api/requests";
import { useSavedSetup, type SetupTarget } from "../api/saved-setup";
import { useUnfinishedPractice } from "../api/setup-catalog";
import { setupHref, type SetupParams } from "../lib/setup-params";
import { CompanyInterviewer } from "@/features/candidate/applications/components/company-interviewer";
import { RecordingPreflight } from "@/features/interview/components/recording-preflight";
import {
  forgetRecordingConsent,
  rememberRecordingConsent,
} from "@/features/interview/recording/recording-consent";
import { CandidateApplicationStatusBadge } from "@/features/candidate/applications/components/application-status";
import { useNow } from "@/features/candidate/applications/utils/use-now";
import { AvatarPortrait } from "./avatar-portrait";
import { PreflightCheck, type MicrophoneReadiness } from "./preflight-check";
import { PracticeProgress, ResumeCallout } from "./resume-callout";
import { RecruitmentEnvironment } from "./recruitment-environment";
import { SetupBlockers } from "./setup-blockers";
import { SetupHeader } from "./setup-header";
import { StartPanel } from "./start-panel";
import { apiErrorMessage } from "@/lib/i18n/api-errors";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  roomBackgrounds,
  type RoomBackgroundId,
} from "@/features/interview/components/room-backgrounds";

// Step 5: the microphone check and the one real operation of setup, start or resume. The layout
// reads summary, microphone, start on a phone; on a desktop the microphone takes the main column
// and the summary and start panel sit beside it.
export function InterviewReadiness(params: SetupParams) {
  const t = useTranslations("candidate.setup");
  const [readiness, setReadiness] = useState<MicrophoneReadiness>("not-ready");
  const onReadiness = useCallback(
    (value: MicrophoneReadiness) => setReadiness(value),
    [],
  );
  // A recruitment interview is recorded, so its Start also waits for the camera (KAN-87).
  const [cameraReady, setCameraReady] = useState(false);
  // ...and the Candidate's consent to it. The tick is remembered per account so the recording upload,
  // which happens after the interview, can send the version that was accepted (KAN-297).
  const sessionUser = useSessionUser();
  const [consentGiven, setConsentGiven] = useState(false);
  const onConsentChange = useCallback(
    (accepted: boolean) => {
      setConsentGiven(accepted);
      if (accepted) rememberRecordingConsent(sessionUser.id);
      else forgetRecordingConsent(sessionUser.id);
    },
    [sessionUser.id],
  );
  const tRecording = useTranslations(
    "candidate.interviewRoom.recording.preflight",
  );
  const microphone = <PreflightCheck onReadinessChange={onReadiness} />;
  if (params.applicationId)
    return (
      <RecruitmentReadiness
        applicationId={params.applicationId}
        readiness={readiness}
        cameraReady={cameraReady}
        consentGiven={consentGiven}
        microphone={
          <>
            <PreflightCheck
              onReadinessChange={onReadiness}
              releaseNote={tRecording("releaseNote")}
            />
            <RecordingPreflight
              onReadyChange={setCameraReady}
              onConsentChange={onConsentChange}
            />
          </>
        }
      />
    );
  if (params.roleProfileId)
    return (
      <PracticeReadiness
        roleProfileId={params.roleProfileId}
        focusItemId={params.focusItemId}
        avatarId={params.avatarId}
        voiceId={params.voiceId}
        readiness={readiness}
        microphone={microphone}
      />
    );
  return (
    <section className="space-y-8">
      <SetupHeader
        context={t("context")}
        title={t("readiness.generic.title")}
        description={t("readiness.generic.description")}
      />
      <Layout
        microphone={microphone}
        side={
          <div className="space-y-3 rounded-xl border bg-card p-5 text-sm">
            <h2 className="font-semibold">
              {t("readiness.generic.sideTitle")}
            </h2>
            <p className="text-muted-foreground">
              {t("readiness.generic.sideText")}
            </p>
            <Button
              asChild
              className="h-auto min-h-11 whitespace-normal py-2 w-full"
            >
              <Link href={routes.interviews.new.setup}>
                {t("readiness.generic.action")}
              </Link>
            </Button>
          </div>
        }
      />
    </section>
  );
}

function Layout({
  summary,
  microphone,
  side,
}: {
  summary?: ReactNode;
  microphone: ReactNode;
  side: ReactNode;
}) {
  return (
    // The microphone spans both rows; its extra height goes to the 1fr row, so the summary and
    // the start panel stay stacked at the top of the side column instead of stretching.
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,24rem)] lg:grid-rows-[auto_1fr] lg:items-start lg:gap-x-10">
      {summary && (
        <div className="min-w-0 lg:col-start-2 lg:row-start-1">{summary}</div>
      )}
      <div className="min-w-0 rounded-xl border bg-card p-5 sm:p-6 lg:col-start-1 lg:row-span-2 lg:row-start-1">
        {microphone}
      </div>
      <div
        className={
          summary
            ? "min-w-0 lg:col-start-2 lg:row-start-2"
            : "min-w-0 lg:col-start-2 lg:row-start-1"
        }
      >
        {side}
      </div>
    </div>
  );
}

function PracticeReadiness({
  roleProfileId,
  focusItemId,
  avatarId,
  voiceId,
  readiness,
  microphone,
}: {
  roleProfileId: string;
  focusItemId?: string;
  avatarId?: string;
  voiceId?: string;
  readiness: MicrophoneReadiness;
  microphone: ReactNode;
}) {
  const t = useTranslations("candidate.setup");
  const errors = useTranslations("errors");
  const user = useSessionUser();
  const cache = useQueryClient();
  const profile = useQuery({
    queryKey: roleProfileKeys.detail(user.id, roleProfileId),
    queryFn: ({ signal }) => getRoleProfile(roleProfileId, signal),
    retry: false,
  });
  const unfinished = useUnfinishedPractice(user.id);
  const roadmap = useQuery({
    queryKey: roadmapKeys.detail(user.id, roleProfileId),
    queryFn: ({ signal }) => getRoadmap(roleProfileId, signal),
    enabled: Boolean(focusItemId),
    staleTime: 0,
    refetchOnMount: "always",
    retry: false,
  });
  const roadmapT = useTranslations("candidate.roadmap");
  // The saved setup is the source of truth for what the interview will use; the avatarId and
  // voiceId in the URL are only carried along for the steps' own links.
  const target: SetupTarget = { mode: "practice", roleProfileId };
  const setup = useSavedSetup(user.id, target);
  const view = setup.data;
  const savedAvatarId = view?.avatar?.id;
  const avatar = useQuery({
    queryKey: avatarKeys.detail(user.id, savedAvatarId ?? "none"),
    queryFn: ({ signal }) => getAvatar(savedAvatarId ?? "", signal),
    enabled: Boolean(savedAvatarId),
    retry: false,
  });
  // PayOS returns here after Add coins; the payment is confirmed with the server, as on Billing.
  const sync = useMutation({
    mutationFn: (code: number) => syncDepositStatus(code),
    onSettled: () =>
      void cache.invalidateQueries({ queryKey: walletKeys.all(user.id) }),
  });
  const { mutate: syncPayment } = sync;
  const onPaymentReturn = useCallback(
    ({ orderCode }: PaymentReturnEvent) => syncPayment(orderCode),
    [syncPayment],
  );

  const resume = unfinished.data?.get(roleProfileId);
  const returnPath = setupHref(routes.interviews.new.preflight, {
    roleProfileId,
    focusItemId,
    avatarId,
    voiceId,
  });
  const header = (
    <SetupHeader
      context={t("context")}
      title={
        resume
          ? t("readiness.practice.resumeTitle")
          : t("readiness.practice.startTitle")
      }
      description={
        resume
          ? t("readiness.practice.resumeDescription")
          : t("readiness.practice.startDescription")
      }
    />
  );

  if (
    profile.isError ||
    (profile.data !== undefined && profile.data.status !== "confirmed")
  )
    return (
      <section className="space-y-6">
        {header}
        <ErrorState
          description={
            profile.data
              ? t("readiness.practice.confirm")
              : isAxiosError(profile.error) &&
                  profile.error.response?.status === 404
                ? t("readiness.practice.notFound")
                : apiErrorMessage(
                    errors,
                    profile.error,
                    t("roleProfile.loadFailed"),
                  )
          }
        >
          <Button
            asChild
            variant="outline"
            className="h-auto min-h-11 whitespace-normal py-2 bg-background"
          >
            <Link
              href={
                profile.data
                  ? routes.roleProfiles.detail(roleProfileId)
                  : routes.interviews.new.setup
              }
            >
              {profile.data
                ? t("readiness.practice.openProfile")
                : t("readiness.practice.chooseProfile")}
            </Link>
          </Button>
        </ErrorState>
      </section>
    );

  const editLink = (href: string, label: string) => (
    <Link
      href={href}
      className="inline-flex min-h-11 items-center text-sm font-medium underline underline-offset-4"
      aria-label={label}
    >
      {t("readiness.summary.edit")}
    </Link>
  );

  const summary = (
    <section
      aria-label={t("readiness.summary.title")}
      className="rounded-xl border bg-card p-5"
    >
      <h2 className="font-semibold">{t("readiness.summary.title")}</h2>
      <dl className="mt-3 divide-y text-sm">
        <SummaryRow
          label={t("readiness.summary.roleProfile")}
          action={editLink(
            setupHref(routes.interviews.new.setup, {
              roleProfileId,
              focusItemId,
            }),
            t("readiness.summary.changeProfile"),
          )}
        >
          {profile.data ? (
            profile.data.title
          ) : (
            <Skeleton className="h-5 w-40" />
          )}
        </SummaryRow>
        {resume ? (
          <SummaryRow label={t("readiness.summary.keptLabel")}>
            {t("readiness.summary.kept")}
          </SummaryRow>
        ) : (
          <>
            <SummaryRow
              label={t("readiness.summary.interviewer")}
              action={editLink(
                setupHref(routes.interviews.new.interviewer, {
                  roleProfileId,
                  focusItemId,
                  avatarId,
                  voiceId,
                }),
                t("readiness.summary.changeInterviewer"),
              )}
            >
              {!view ? (
                setup.isError ? (
                  t("readiness.summary.avatarError")
                ) : (
                  <Skeleton className="h-10 w-32" />
                )
              ) : !view.avatar ? (
                t("readiness.summary.notChosen")
              ) : avatar.data ? (
                <span className="flex items-center gap-3">
                  <AvatarPortrait
                    avatar={avatar.data}
                    userId={user.id}
                    compact
                    className="w-10 shrink-0 rounded-md"
                  />
                  <span className="[overflow-wrap:anywhere]">
                    {avatar.data.name}
                  </span>
                </span>
              ) : avatar.isError ? (
                view.avatar.name
              ) : (
                <Skeleton className="h-10 w-32" />
              )}
            </SummaryRow>
            <SummaryRow label={t("readiness.summary.voice")}>
              {!view ? (
                setup.isError ? (
                  t("readiness.summary.voiceError")
                ) : (
                  <Skeleton className="h-5 w-28" />
                )
              ) : (
                (view.voice?.name ?? t("readiness.summary.notChosen"))
              )}
            </SummaryRow>
            <SummaryRow label={t("readiness.summary.environment")}>
              {!view ? (
                <Skeleton className="h-5 w-24" />
              ) : view.environment ? (
                <RoomName id={view.environment} />
              ) : (
                t("readiness.summary.notChosen")
              )}
            </SummaryRow>
          </>
        )}
      </dl>
    </section>
  );

  const focusReady =
    !focusItemId ||
    (roadmap.data?.generation_status === "ready" &&
      roadmap.data.items.some((item) => item.id === focusItemId));
  const side =
    !resume && !focusReady ? (
      <div className="space-y-3 rounded-xl border p-5">
        <p role="status" className="text-sm">
          {roadmapT(roadmap.isPending ? "loadingFocus" : "unknownFocus")}
        </p>
        <Button asChild variant="outline" className="min-h-11">
          <Link href={routes.roleProfiles.roadmap(roleProfileId)}>
            {roadmapT("chooseAnotherTopic")}
          </Link>
        </Button>
      </div>
    ) : unfinished.isPending || (!resume && setup.isPending) ? (
      <div role="status" className="space-y-3 rounded-xl border bg-card p-5">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-11 w-full" />
        <span className="sr-only">{t("readiness.checking")}</span>
      </div>
    ) : !resume && !view ? (
      <ErrorState
        description={t("blockers.error")}
        onRetry={() => void setup.refetch()}
        retryLabel={t("tryAgain")}
        isRetrying={setup.isFetching}
      ></ErrorState>
    ) : (
      <>
        {!resume && view && (
          <SetupBlockers
            blockers={view.blockers}
            target={target}
            params={{ roleProfileId, focusItemId, avatarId, voiceId }}
          />
        )}
        <StartPanel
          readiness={readiness}
          setupReady={resume ? undefined : view?.ready}
          links={{ roleProfileId }}
          returnPath={returnPath}
          plan={
            resume
              ? {
                  kind: "resume",
                  type: "practice",
                  sessionId: resume.id,
                  callout: (
                    <ResumeCallout
                      title={t("resume.calloutTitle")}
                      details={<PracticeProgress session={resume} />}
                      note={t("resume.noCharge")}
                    />
                  ),
                }
              : {
                  kind: "new",
                  request: {
                    type: "practice",
                    roleProfileId,
                    focusItemId,
                    avatarId: view?.avatar?.id,
                    voiceId: view?.voice?.id,
                  },
                }
          }
        />
      </>
    );

  return (
    <section className="space-y-8">
      {header}
      <Suspense fallback={null}>
        <PaymentStatusBanner
          verification={sync}
          onPaymentReturn={onPaymentReturn}
        />
      </Suspense>
      <PracticeFocus
        profileId={roleProfileId}
        focusItemId={resume ? resume.focusItemId : focusItemId}
        requestedFocusId={focusItemId}
        resumed={Boolean(resume)}
      />
      <Layout summary={summary} microphone={microphone} side={side} />
    </section>
  );
}

function RecruitmentReadiness({
  applicationId,
  readiness,
  cameraReady,
  consentGiven,
  microphone,
}: {
  applicationId: string;
  readiness: MicrophoneReadiness;
  cameraReady: boolean;
  consentGiven: boolean;
  microphone: ReactNode;
}) {
  const t = useTranslations("candidate.setup");
  const fmt = useFormatters();
  const now = useNow();
  const ti = useTranslations("candidate.applications.interviewer");
  const user = useSessionUser();
  const application = useQuery({
    queryKey: candidateApplicationKeys.detail(user.id, applicationId),
    queryFn: ({ signal }) => getCandidateApplication(applicationId, signal),
    // Eligibility and the deadline move on the recruiter's side, so always ask again.
    refetchOnMount: "always",
    refetchInterval: 30_000,
    retry: false,
  });
  const setup = useSavedSetup(user.id, { mode: "recruitment", applicationId });
  const data = application.data;
  const inProgress = data?.interview?.status === "in_progress";
  const back = (
    <Link
      href={routes.application(applicationId)}
      className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <ArrowLeft className="size-4" aria-hidden />
      {t("readiness.recruitment.back")}
    </Link>
  );
  const header = (
    <SetupHeader
      context={data?.job_posting_title ?? t("readiness.recruitment.context")}
      title={
        inProgress
          ? t("readiness.recruitment.resumeTitle")
          : t("readiness.recruitment.startTitle")
      }
      description={t("readiness.recruitment.description")}
    />
  );

  if (application.isPending)
    return (
      <section className="space-y-8" aria-busy="true">
        {back}
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <span className="sr-only">{t("readiness.recruitment.loading")}</span>
        </div>
      </section>
    );

  if (application.isError || !data)
    return (
      <section className="space-y-6">
        {back}
        <ErrorState
          description={
            isAxiosError(application.error) &&
            application.error.response?.status === 404
              ? t("readiness.recruitment.notFound")
              : t("readiness.recruitment.loadFailed")
          }
          onRetry={() => void application.refetch()}
          retryLabel={t("tryAgain")}
          isRetrying={application.isFetching}
        ></ErrorState>
      </section>
    );

  const summary = (
    <section
      aria-label={t("readiness.summary.title")}
      className="rounded-xl border bg-card p-5"
    >
      <h2 className="font-semibold">{t("readiness.summary.title")}</h2>
      <dl className="mt-3 divide-y text-sm">
        <SummaryRow label={t("readiness.summary.jobPosting")}>
          {data.job_posting_title}
        </SummaryRow>
        <SummaryRow label={t("readiness.summary.keptLabel")}>
          {data.can_start_interview &&
          (!data.interview_deadline ||
            now < Date.parse(data.interview_deadline)) ? (
            <CompanyInterviewer
              userId={user.id}
              applicationId={applicationId}
            />
          ) : (
            t("readiness.summary.companyInterviewer")
          )}
          <CandidateApplicationStatusBadge status={data.status} />
        </SummaryRow>
      </dl>
    </section>
  );

  const deadline = data.interview_deadline;
  const blocked =
    !data.can_start_interview ||
    (deadline !== null && now >= Date.parse(deadline));
  const side = blocked ? (
    <div className="space-y-3 rounded-xl border bg-card p-5 text-sm">
      <h2 className="font-semibold">
        {t(`readiness.recruitment.status.${data.status}`)}
      </h2>
      <p role="status" className="text-muted-foreground">
        {data.blocker
          ? ti(`blockers.${data.blocker}`)
          : t("readiness.recruitment.notEligible")}
      </p>
      {deadline && (
        <dl className="space-y-2">
          <div>
            <dt className="text-muted-foreground">
              {t("readiness.recruitment.completeBy")}
            </dt>
            <dd>
              <time dateTime={deadline}>{formatDeadline(deadline, fmt)}</time>
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t("readiness.recruitment.timeLeft")}
            </dt>
            <dd className="tabular-nums">
              <TimeLeft deadline={deadline} />
            </dd>
          </div>
        </dl>
      )}
      <Button
        asChild
        variant="outline"
        className="h-auto min-h-11 whitespace-normal py-2 w-full"
      >
        <Link href={routes.application(applicationId)}>
          {t("readiness.recruitment.back")}
        </Link>
      </Button>
    </div>
  ) : (
    <StartPanel
      readiness={readiness}
      setupReady={inProgress ? undefined : (setup.data?.ready ?? false)}
      cameraReady={cameraReady}
      consentGiven={consentGiven}
      links={{ applicationId }}
      returnPath={setupHref(routes.interviews.new.preflight, {
        applicationId,
      })}
      plan={
        inProgress && data.interview
          ? {
              kind: "resume",
              type: "recruitment",
              sessionId: data.interview.id,
              callout: (
                <ResumeCallout
                  title={t("resume.calloutTitle")}
                  details={
                    deadline
                      ? t.rich("readiness.recruitment.completeIt", {
                          deadline: formatDeadline(deadline, fmt),
                          time: (chunks) => (
                            <time dateTime={deadline}>{chunks}</time>
                          ),
                        })
                      : undefined
                  }
                  note={t("readiness.recruitment.resumeNote")}
                />
              ),
            }
          : { kind: "new", request: { type: "recruitment", applicationId } }
      }
    >
      {deadline && (
        <dl className="divide-y rounded-lg border text-sm">
          <div className="flex items-center justify-between gap-4 px-4 py-3">
            <dt className="text-muted-foreground">
              {t("readiness.recruitment.completeBy")}
            </dt>
            <dd className="text-right font-semibold">
              <time dateTime={deadline}>{formatDeadline(deadline, fmt)}</time>
            </dd>
          </div>
          <div className="flex items-center justify-between gap-4 px-4 py-3">
            <dt className="text-muted-foreground">
              {t("readiness.recruitment.timeLeft")}
            </dt>
            <dd className="font-semibold tabular-nums">
              <TimeLeft deadline={deadline} />
            </dd>
          </div>
        </dl>
      )}
    </StartPanel>
  );

  return (
    <section className="space-y-8">
      {back}
      {header}
      {!inProgress && !blocked && (
        <RecruitmentEnvironment applicationId={applicationId} setup={setup} />
      )}
      <Layout summary={summary} microphone={microphone} side={side} />
    </section>
  );
}

function SummaryRow({
  label,
  action,
  children,
}: {
  label: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
      <div className="min-w-0 space-y-1">
        <dt className="text-xs text-muted-foreground">{label}</dt>
        <dd className="font-medium [overflow-wrap:anywhere]">{children}</dd>
      </div>
      {action}
    </div>
  );
}

// The countdown is re-read once a minute; it stops with the component.
function TimeLeft({ deadline }: { deadline: string }) {
  const fmt = useFormatters();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60_000);
    return () => window.clearInterval(timer);
  }, []);
  return timeLeftText(deadline, now, fmt);
}

// The room name, from the room picker's own catalog.
function RoomName({ id }: { id: string }) {
  const t = useTranslations("candidate.interviewRoom.background");
  return roomBackgrounds.some((room) => room.id === id)
    ? t(`options.${id as RoomBackgroundId}`)
    : id;
}
