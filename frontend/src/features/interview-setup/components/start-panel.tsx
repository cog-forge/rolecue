"use client";

import { useId, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { AlertCircle, ArrowRight, LoaderCircle } from "lucide-react";
import { CoinIcon } from "@/components/icons/coin-icon";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getWallet, walletKeys } from "@/features/billing/api/wallet";
import { AddCoinsDialog } from "@/features/billing/components/wallet-purchase";
import { candidateApplicationKeys } from "@/features/candidate/applications/api/applications";
import { historyKeys } from "@/features/history/api/history";
import { interviewSetupKeys } from "../api/setup-catalog";
import {
  createInterviewSession,
  PRACTICE_SESSION_COINS,
  resumeInterviewSession,
  startFailure,
  type StartRequest,
} from "../api/start-session";
import { setupHref } from "../lib/setup-params";
import { describeStartFailure, type StartAction } from "../lib/start-errors";
import type { MicrophoneReadiness } from "./preflight-check";

// What pressing the button does. A resume names the session the server already has; a new start
// names what to create. The plan, not the button, decides the price shown.
export type StartPlan =
  | { kind: "new"; request: StartRequest }
  | {
      kind: "resume";
      type: StartRequest["type"];
      sessionId: string;
      callout: ReactNode;
    };

const micHints: Record<Exclude<MicrophoneReadiness, "ready">, string> = {
  "not-ready": "Turn on your microphone to start.",
  muted: "Unmute your microphone to start.",
  interrupted: "Your microphone needs attention. Retry the check to start.",
  error: "Your microphone needs attention. Retry the check to start.",
};

const deniedCodes = new Set(["FORBIDDEN", "INVALID_TOKEN", "USER_INACTIVE"]);

export function StartPanel({
  plan,
  readiness,
  links,
  returnPath,
  children,
}: {
  plan: StartPlan;
  readiness: MicrophoneReadiness;
  /** Where the error actions lead: the Role Profile (practice) or the Application (recruitment). */
  links: { roleProfileId?: string; applicationId?: string };
  /** The preflight URL, so PayOS brings the Candidate back here after buying coins. */
  returnPath: string;
  /** Extra facts for a new start (the recruitment deadline). */
  children?: ReactNode;
}) {
  const user = useSessionUser();
  const router = useRouter();
  const cache = useQueryClient();
  const ids = useId();
  const [addingCoins, setAddingCoins] = useState(false);
  // One Idempotency-Key per attempt: kept while a retry follows an ambiguous failure, replaced when
  // the request changes or a failure proves nothing was created. Memory only, never storage.
  const attempt = useRef<{ key: string; signature: string } | null>(null);
  const type = plan.kind === "new" ? plan.request.type : plan.type;
  const paid = plan.kind === "new" && type === "practice";
  const wallet = useQuery({
    queryKey: walletKeys.wallet(user.id),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: paid,
    // The balance decides whether Start is offered, so it is always read fresh.
    refetchOnMount: "always",
    staleTime: 0,
  });

  const start = useMutation({
    mutationFn: async (current: StartPlan) => {
      if (current.kind === "resume")
        return {
          session: await resumeInterviewSession(current.sessionId),
          existing: false,
        };
      const signature = JSON.stringify(current.request);
      if (attempt.current?.signature !== signature)
        attempt.current = { key: crypto.randomUUID(), signature };
      const { session, created } = await createInterviewSession(
        current.request,
        attempt.current.key,
      );
      return { session, existing: !created };
    },
    onSuccess: ({ session, existing }) => {
      attempt.current = null;
      void cache.invalidateQueries({ queryKey: walletKeys.all(user.id) });
      void cache.invalidateQueries({ queryKey: historyKeys.all(user.id) });
      void cache.invalidateQueries({
        queryKey: interviewSetupKeys.unfinished(user.id),
      });
      void cache.invalidateQueries({
        queryKey: candidateApplicationKeys.all(user.id),
      });
      // A 200 to a new start is a session that already existed. It may be a replay of a start
      // that was charged, so nothing is said about coins.
      if (existing)
        gooeyToast.info("Opening your existing interview", {
          description: "You already had this interview in progress.",
        });
      router.push(routes.interviews.room(session.id));
    },
    onError: (error) => {
      const failure = startFailure(error);
      if (!failure.confirmed) return;
      attempt.current = null;
      if (failure.code === "INSUFFICIENT_BALANCE")
        void cache.invalidateQueries({ queryKey: walletKeys.all(user.id) });
      if (
        failure.code === "INTERVIEW_COMPLETED" ||
        failure.code === "INTERVIEW_NOT_FOUND"
      )
        void cache.invalidateQueries({
          queryKey: interviewSetupKeys.unfinished(user.id),
        });
      if (type === "recruitment")
        void cache.invalidateQueries({
          queryKey: candidateApplicationKeys.all(user.id),
        });
      if (deniedCodes.has(failure.code))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });

  const balance = wallet.data?.balance;
  const short =
    paid && balance !== undefined && balance < PRACTICE_SESSION_COINS
      ? PRACTICE_SESSION_COINS - balance
      : 0;
  const walletBlocks =
    paid && (wallet.isPending || wallet.isError || short > 0);
  const busy = start.isPending || start.isSuccess;
  const micReady = readiness === "ready";
  const disabled = busy || !micReady || walletBlocks;
  const problem = start.isError
    ? describeStartFailure(startFailure(start.error), {
        type,
        resume: plan.kind === "resume",
        balance,
      })
    : null;
  const resume = plan.kind === "resume";
  const label = start.isSuccess
    ? "Opening the interview room…"
    : start.isPending
      ? resume
        ? "Resuming…"
        : "Starting…"
      : resume
        ? "Resume interview"
        : paid
          ? `Start interview · ${PRACTICE_SESSION_COINS} coins`
          : "Start interview";
  const hint = !micReady
    ? micHints[readiness]
    : paid && wallet.isError
      ? "Start is unavailable until your balance loads."
      : null;

  const action = (
    <div className="space-y-3">
      {problem && (
        <Problem
          message={problem.message}
          action={problem.action}
          links={links}
          onAddCoins={() => setAddingCoins(true)}
          onStartNew={() => start.reset()}
        />
      )}
      <Button
        type="button"
        className="h-11 w-full"
        disabled={disabled}
        aria-describedby={hint ? `${ids}-hint` : undefined}
        onClick={() => start.mutate(plan)}
      >
        {busy && (
          <LoaderCircle
            aria-hidden="true"
            className="motion-safe:animate-spin"
          />
        )}
        {label}
        {!busy && <ArrowRight aria-hidden="true" />}
      </Button>
      <p
        id={`${ids}-hint`}
        role="status"
        aria-live="polite"
        className="min-h-4 text-xs text-muted-foreground"
      >
        {busy ? label : hint}
      </p>
    </div>
  );

  if (plan.kind === "resume")
    return (
      <div className="space-y-3">
        {plan.callout}
        {action}
      </div>
    );

  return (
    <section
      aria-labelledby={`${ids}-title`}
      className="space-y-5 rounded-xl border bg-card p-5"
    >
      <div className="space-y-1">
        <h2 id={`${ids}-title`} className="font-semibold">
          {paid ? "Practice interview" : "Recruitment interview"}
        </h2>
        <p className="text-sm text-muted-foreground">
          {paid
            ? `${PRACTICE_SESSION_COINS} coins are charged once, when the interview starts. Resuming it later is free.`
            : "Free for you. The company covers this interview."}
        </p>
      </div>

      {paid && (
        <dl className="divide-y rounded-lg border text-sm">
          <div className="flex items-center justify-between gap-4 px-4 py-3">
            <dt className="text-muted-foreground">Price</dt>
            <dd className="flex items-center gap-1.5 font-semibold tabular-nums">
              <CoinIcon className="size-4" />
              {PRACTICE_SESSION_COINS} coins
            </dd>
          </div>
          <div className="flex items-center justify-between gap-4 px-4 py-3">
            <dt className="text-muted-foreground">Your balance</dt>
            <dd className="font-semibold tabular-nums">
              {wallet.isPending ? (
                <Skeleton className="h-5 w-16" />
              ) : wallet.isError ? (
                "—"
              ) : (
                `${balance?.toLocaleString("en-US")} coins`
              )}
            </dd>
          </div>
        </dl>
      )}
      {children}

      {paid && wallet.isError && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-3 text-sm"
        >
          <p>We couldn&apos;t load your balance.</p>
          <Button
            type="button"
            variant="outline"
            className="h-11"
            disabled={wallet.isFetching}
            onClick={() => void wallet.refetch()}
          >
            Try again
          </Button>
        </div>
      )}
      {short > 0 && !problem && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/60 p-3 text-sm">
          <p>
            You need{" "}
            <span className="font-semibold tabular-nums">
              {short} more {short === 1 ? "coin" : "coins"}
            </span>{" "}
            to start.
          </p>
          <Button
            type="button"
            variant="outline"
            className="h-11 bg-background"
            onClick={() => setAddingCoins(true)}
          >
            <CoinIcon className="size-4" /> Add coins
          </Button>
        </div>
      )}

      {action}

      {paid && (
        <AddCoinsDialog
          open={addingCoins}
          onOpenChange={setAddingCoins}
          userId={user.id}
          role="candidate"
          returnPath={returnPath}
        />
      )}
    </section>
  );
}

function Problem({
  message,
  action,
  links: { roleProfileId, applicationId },
  onAddCoins,
  onStartNew,
}: {
  message: string;
  action: StartAction | null;
  links: { roleProfileId?: string; applicationId?: string };
  onAddCoins: () => void;
  onStartNew: () => void;
}) {
  const link = (href: string, text: string) => (
    <Button asChild variant="outline" className="h-11 bg-background">
      <Link href={href}>{text}</Link>
    </Button>
  );
  let control: ReactNode = null;
  if (action === "add-coins")
    control = (
      <Button
        type="button"
        variant="outline"
        className="h-11 bg-background"
        onClick={onAddCoins}
      >
        <CoinIcon className="size-4" /> Add coins
      </Button>
    );
  else if (action === "start-new")
    control = (
      <Button
        type="button"
        variant="outline"
        className="h-11 bg-background"
        onClick={onStartNew}
      >
        Set up a new interview
      </Button>
    );
  else if (action === "role-profile" && roleProfileId)
    control = link(
      routes.roleProfiles.detail(roleProfileId),
      "Open Role Profile",
    );
  else if (action === "setup")
    control = link(routes.interviews.new.setup, "Choose a Role Profile");
  else if (action === "interviewer" && roleProfileId)
    control = link(
      setupHref(routes.interviews.new.interviewer, { roleProfileId }),
      "Change interviewer",
    );
  else if (action === "application" && applicationId)
    control = link(routes.application(applicationId), "Back to application");
  else if (action === "applications")
    control = link(routes.applications, "Back to applications");

  return (
    <div
      role="alert"
      className="space-y-3 rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm duration-(--duration-fast) ease-(--ease-smooth-out) animate-in fade-in slide-in-from-top-1 motion-reduce:animate-none"
    >
      <p className="flex items-start gap-2">
        <AlertCircle
          aria-hidden="true"
          className="mt-0.5 size-4 shrink-0 text-destructive"
        />
        {message}
      </p>
      {control}
    </div>
  );
}
