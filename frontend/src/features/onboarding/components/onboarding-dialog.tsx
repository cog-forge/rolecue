"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { isAxiosError } from "axios";
import {
  BriefcaseBusiness,
  Check,
  LoaderCircle,
  LogOut,
  UserRound,
  ArrowRight,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { OnboardingForm as OnboardingFormFrame } from "@/components/ui/onboarding-form";
import {
  getMyProfile,
  updateMyProfile,
  profileKeys,
} from "@/features/profile/api/profile";
import type { SessionUser } from "@/features/auth/schemas/session-schema";
import { useSignOut } from "@/features/auth/hooks/use-sign-out";
import { workspaceHome } from "@/config/navigation";
import { cn } from "@/lib/utils";
import { selectOnboardingRole, onboardingError } from "../api/onboarding";
import type { OnboardingRole } from "../schemas/onboarding-schema";
import { OnboardingForm } from "./onboarding-form";

export function OnboardingDialog({ user }: { user: SessionUser }) {
  const cache = useQueryClient();
  const router = useRouter();
  const signOut = useSignOut();
  const [role, setRole] = useState<OnboardingRole | null>(null);
  const selected = user.onboarding_role_selected;
  const headingRef = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (selected) headingRef.current?.focus({ preventScroll: true });
  }, [selected]);
  const profile = useQuery({
    queryKey: profileKeys.me(user.id),
    queryFn: ({ signal }) => getMyProfile(signal),
    enabled: selected,
    staleTime: 0,
    retry: false,
  });
  const reconcile = useCallback(
    async (error: unknown) => {
      if (
        isAxiosError(error) &&
        [401, 403].includes(error.response?.status ?? 0)
      )
        await cache.invalidateQueries({ queryKey: ["auth", "me"] });
    },
    [cache],
  );
  useEffect(() => {
    if (profile.error) void reconcile(profile.error);
  }, [profile.error, reconcile]);
  const choose = useMutation({
    mutationFn: selectOnboardingRole,
    onSuccess: (updated) => {
      cache.setQueryData(["auth", "me"], updated);
      void cache.invalidateQueries({ queryKey: profileKeys.me(user.id) });
    },
    onError: reconcile,
  });
  const save = useMutation({
    mutationFn: updateMyProfile,
    onSuccess: async (updated) => {
      cache.setQueryData(profileKeys.me(user.id), updated);
      await cache.invalidateQueries({ queryKey: ["auth", "me"] });
      router.replace(workspaceHome(updated.role));
    },
    onError: reconcile,
  });
  return (
    <main className="min-h-svh bg-transparent">
      <Dialog open>
        <DialogContent
          showCloseButton={false}
          onEscapeKeyDown={(e) => e.preventDefault()}
          onPointerDownOutside={(e) => e.preventDefault()}
          onInteractOutside={(e) => e.preventDefault()}
          overlayClassName="bg-[#172342]/20 dark:bg-black/45 backdrop-blur-[2px] supports-backdrop-filter:backdrop-blur-[2px]"
          className="max-h-[calc(100dvh-1.5rem)] max-w-[calc(100%_-_1.5rem)] gap-0 overflow-y-auto rounded-[27px] border-0 bg-transparent p-0 shadow-none sm:max-w-[640px]"
        >
          <OnboardingFormFrame
            imageSrc="/images/onboarding-art-blue.webp"
            topBar={
              <Button
                variant="outline"
                size="sm"
                onClick={() => void signOut.signOut()}
                disabled={signOut.pending}
                className="h-8 rounded-full border-white/70 bg-white/85 px-3 text-[#1c2848] shadow-sm backdrop-blur-md hover:bg-white dark:border-white/20 dark:bg-[#223451]/90 dark:text-[#eef2ff] dark:hover:bg-[#2f4364]"
              >
                <LogOut className="size-3.5" aria-hidden />
                {signOut.pending ? "Signing out…" : "Sign out"}
              </Button>
            }
            progress={
              <ol
                aria-label="Onboarding progress"
                className="mx-auto mb-5 flex max-w-[350px] items-center gap-2 text-[11px] font-medium text-[#536079] dark:text-[#aab7cf] sm:mb-6"
              >
                <li
                  aria-current={!selected ? "step" : undefined}
                  className="flex shrink-0 items-center gap-1.5"
                >
                  <span className="grid size-5 place-items-center rounded-full bg-[#1c2848] text-[10px] text-white dark:bg-[#9abbff] dark:text-[#12213a]">
                    {selected ? <Check className="size-3" aria-hidden /> : "1"}
                  </span>
                  Choose your role
                </li>
                <li
                  role="presentation"
                  className="h-px flex-1 bg-[#b9c0ce] dark:bg-[#45516a]"
                  aria-hidden
                />
                <li
                  aria-current={selected ? "step" : undefined}
                  className={cn(
                    "flex shrink-0 items-center gap-1.5",
                    !selected && "text-[#8790a2] dark:text-[#93a2be]",
                  )}
                >
                  <span
                    className={cn(
                      "grid size-5 place-items-center rounded-full text-[10px]",
                      selected
                        ? "bg-[#1c2848] text-white dark:bg-[#9abbff] dark:text-[#12213a]"
                        : "bg-[#e7e9ef] text-[#8790a2] dark:bg-[#2a354b] dark:text-[#aab7cf]",
                    )}
                  >
                    2
                  </span>
                  Your details
                </li>
              </ol>
            }
            heading={
              <DialogTitle
                ref={headingRef}
                tabIndex={-1}
                className="text-[25px] leading-tight font-bold tracking-tight text-[#202438] dark:text-[#eef2ff] outline-none sm:text-[30px]"
              >
                {selected
                  ? "Make this space yours"
                  : "Welcome to your next chapter"}
              </DialogTitle>
            }
            description={
              <DialogDescription className="mx-auto max-w-[410px] text-sm leading-relaxed text-[#677084] dark:text-[#aab7cf]">
                {selected
                  ? `Add a few details to set up your ${user.role} profile. Your email is already saved.`
                  : "Tell us what brings you here. You’ll choose your role once, then make your profile your own."}
              </DialogDescription>
            }
          >
            {!selected ? (
              <form
                aria-label="Choose your role"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (role) choose.mutate(role);
                }}
                className="space-y-5"
              >
                <fieldset
                  disabled={choose.isPending}
                  className="grid gap-3 sm:grid-cols-2"
                >
                  <legend className="sr-only">Your role</legend>
                  {(["candidate", "recruiter"] as const).map((value) => {
                    const Icon =
                      value === "candidate" ? UserRound : BriefcaseBusiness;
                    return (
                      <label key={value} className="relative cursor-pointer">
                        <input
                          type="radio"
                          className="peer sr-only"
                          name="role"
                          value={value}
                          checked={role === value}
                          onChange={() => setRole(value)}
                        />
                        <span className="flex h-full flex-col rounded-2xl border border-[#e6e2e8] bg-white/65 p-5 text-left shadow-[0_3px_14px_-10px_rgba(20,33,60,0.25)] transition-all hover:border-[#aeb9d3] hover:bg-white/90 peer-checked:border-[#526fa9] peer-checked:bg-[#edf2fc] peer-checked:ring-1 peer-checked:ring-[#526fa9] peer-focus-visible:ring-2 peer-focus-visible:ring-[#526fa9] peer-focus-visible:ring-offset-2 dark:border-[#3c4962] dark:bg-[#202b40] dark:hover:border-[#6d8dbf] dark:hover:bg-[#273650] dark:peer-checked:border-[#9abbff] dark:peer-checked:bg-[#223451] dark:peer-checked:ring-[#9abbff] dark:peer-focus-visible:ring-[#9abbff] dark:peer-focus-visible:ring-offset-[#182033]">
                          <Icon
                            className="mb-4 size-6 text-[#536da3] dark:text-[#9abbff]"
                            aria-hidden
                          />
                          <span className="font-semibold capitalize">
                            {value}
                          </span>
                          <span className="mt-2 text-[13px] leading-relaxed text-[#667084] dark:text-[#aab7cf]">
                            {value === "candidate"
                              ? "Practice interviews, build confidence, and find your next opportunity."
                              : "Create job postings and discover candidates for your company."}
                          </span>
                        </span>
                      </label>
                    );
                  })}
                </fieldset>
                {choose.isError && (
                  <p role="alert" className="text-sm text-destructive">
                    {onboardingError(choose.error)}
                  </p>
                )}
                <div>
                  <Button
                    type="submit"
                    size="lg"
                    disabled={!role || choose.isPending}
                    className="h-12 w-full rounded-xl bg-[#202438] text-sm text-white shadow-[0_8px_22px_-12px_rgba(32,36,56,0.7)] hover:bg-[#323950] dark:bg-[#9abbff] dark:text-[#12213a] dark:hover:bg-[#b9d0ff]"
                  >
                    {choose.isPending ? (
                      <LoaderCircle
                        className="size-4 animate-spin"
                        aria-hidden
                      />
                    ) : (
                      <ArrowRight className="size-4" aria-hidden />
                    )}
                    Continue{role && ` as ${role}`}
                  </Button>
                </div>
              </form>
            ) : profile.isPending ? (
              <p role="status">Loading your profile…</p>
            ) : profile.error ? (
              <div role="alert" className="space-y-3">
                <p>Unable to load your profile.</p>
                <Button
                  variant="outline"
                  onClick={() => void profile.refetch()}
                >
                  Try again
                </Button>
              </div>
            ) : (
              <OnboardingForm
                key={user.id + user.role}
                profile={profile.data}
                saving={save.isPending}
                error={save.error ? onboardingError(save.error) : null}
                onSave={async (patch) => {
                  try {
                    await save.mutateAsync(patch);
                  } catch {
                    /* Mutation owns the error and keeps the form draft. */
                  }
                }}
              />
            )}
            {signOut.error && (
              <p role="alert" className="mt-4 text-sm text-destructive">
                Sign-out failed. Please try again.
              </p>
            )}
          </OnboardingFormFrame>
        </DialogContent>
      </Dialog>
    </main>
  );
}
