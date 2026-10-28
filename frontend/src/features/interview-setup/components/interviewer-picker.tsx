"use client";

import { useId, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { Controller, useForm } from "react-hook-form";
import { ArrowLeft, ArrowRight, AudioLines } from "lucide-react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  avatarKeys,
  avatarPageSize,
  listAvatars,
} from "@/features/avatar/api/inventory";
import {
  interviewSetupKeys,
  listActiveVoices,
  useUnfinishedPractice,
} from "../api/setup-catalog";
import { setupHref } from "../lib/setup-params";
import { AvatarPortrait } from "./avatar-portrait";
import { PracticeProgress, ResumeCallout } from "./resume-callout";
import { SetupHeader } from "./setup-header";

// "2d" and "default" are the absence of a choice: no avatar_id or voice_id is sent.
const NO_AVATAR = "2d";
const DEFAULT_VOICE = "default";
export const interviewerFormSchema = z.object({
  avatarId: z.union([z.literal(NO_AVATAR), z.uuid()]),
  voiceId: z.union([z.literal(DEFAULT_VOICE), z.uuid()]),
});
type InterviewerForm = z.infer<typeof interviewerFormSchema>;

const cardClass =
  "group relative flex min-w-0 cursor-pointer flex-col overflow-hidden rounded-xl border bg-card transition-[border-color,box-shadow,translate] duration-(--duration-fast) ease-(--ease-smooth-out) hover:-translate-y-0.5 hover:border-foreground/30 has-focus-visible:ring-3 has-focus-visible:ring-ring/50 has-data-[state=checked]:border-foreground has-data-[state=checked]:shadow-[inset_0_0_0_1px_var(--foreground)] motion-reduce:transition-none motion-reduce:hover:translate-y-0";

// Step 4: who interviews the Candidate and how they sound. Both are optional.
export function InterviewerPicker({
  roleProfileId,
  avatarId,
  voiceId,
}: {
  roleProfileId?: string;
  avatarId?: string;
  voiceId?: string;
}) {
  const user = useSessionUser();
  const router = useRouter();
  const ids = useId();
  const [page, setPage] = useState(1);
  const unfinished = useUnfinishedPractice(user.id);
  const avatars = useQuery({
    queryKey: avatarKeys.list(user.id, page),
    queryFn: ({ signal }) => listAvatars(page, signal),
    // Avatars are made in the Avatar Studio, often in another tab: refresh on return.
    refetchOnWindowFocus: true,
    retry: false,
  });
  const voices = useQuery({
    queryKey: interviewSetupKeys.voices(user.id),
    queryFn: ({ signal }) => listActiveVoices(signal),
    retry: false,
  });
  const form = useForm<InterviewerForm>({
    resolver: zodResolver(interviewerFormSchema),
    defaultValues: {
      avatarId: avatarId ?? NO_AVATAR,
      voiceId: voiceId ?? DEFAULT_VOICE,
    },
  });

  if (!roleProfileId)
    return (
      <section className="space-y-6">
        <SetupHeader
          context="Interview setup"
          title="Choose your Role Profile first."
          description="Pick the confirmed role you want to practise for, then choose your interviewer."
        />
        <Button asChild className="h-11">
          <Link href={routes.interviews.new.setup}>Choose a Role Profile</Link>
        </Button>
      </section>
    );

  const resume = unfinished.data?.get(roleProfileId);
  if (resume)
    return (
      <section className="space-y-8">
        <SetupHeader
          context="Interview setup"
          title="Your interviewer is already set"
          description="This role has an unfinished interview. It keeps the interviewer and voice it started with."
        />
        <ResumeCallout
          title="You have an unfinished interview for this role"
          details={<PracticeProgress session={resume} />}
          note="Resuming is free and keeps its interviewer and voice."
        >
          <Button asChild className="h-11 w-full sm:w-auto">
            <Link
              href={setupHref(routes.interviews.new.preflight, {
                roleProfileId,
              })}
            >
              Resume interview <ArrowRight aria-hidden="true" />
            </Link>
          </Button>
        </ResumeCallout>
      </section>
    );

  const avatarPage = avatars.data;
  const allAvatarsLoaded =
    avatarPage !== undefined && avatarPage.pagination.total <= avatarPageSize;
  const pageCount = avatarPage
    ? Math.max(1, Math.ceil(avatarPage.pagination.total / avatarPageSize))
    : 1;
  // A chosen avatar or voice that is gone (deleted, deactivated) falls back to no choice, so the
  // start never sends a reference the API would refuse.
  const avatarGone = (value: string) =>
    value !== NO_AVATAR &&
    allAvatarsLoaded &&
    !avatarPage?.items.some((avatar) => avatar.id === value);
  const voiceGone = (value: string) =>
    value !== DEFAULT_VOICE &&
    voices.isSuccess &&
    !voices.data.some((voice) => voice.id === value);

  const submit = form.handleSubmit((values) => {
    const avatar =
      values.avatarId === NO_AVATAR || avatarGone(values.avatarId)
        ? undefined
        : values.avatarId;
    const voice =
      values.voiceId === DEFAULT_VOICE || voiceGone(values.voiceId)
        ? undefined
        : values.voiceId;
    router.push(
      setupHref(routes.interviews.new.preflight, {
        roleProfileId,
        avatarId: avatar,
        voiceId: voice,
      }),
    );
  });

  return (
    <form className="space-y-10" onSubmit={(event) => void submit(event)}>
      <SetupHeader
        context="Interview setup"
        title="Choose your interviewer"
        description="Pick who interviews you and how they sound. Both are optional: without an avatar you get the 2D interviewer, which works on every device."
      />

      <fieldset className="space-y-4">
        <legend className="font-semibold">Interviewer</legend>
        <p className="-mt-2 text-sm text-muted-foreground">
          Avatars come from your own inventory.{" "}
          <Link
            href={routes.avatarStudio}
            className="font-medium text-foreground underline underline-offset-4"
          >
            Open Avatar Studio
          </Link>
        </p>

        <Controller
          control={form.control}
          name="avatarId"
          render={({ field }) => (
            <>
              <RadioGroup
                aria-label="Interviewer"
                value={avatarGone(field.value) ? NO_AVATAR : field.value}
                onValueChange={field.onChange}
                className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4"
              >
                <label htmlFor={`${ids}-2d`} className={cardClass}>
                  <span className="relative grid aspect-[4/5] place-items-center bg-muted/60">
                    <AudioLines
                      aria-hidden="true"
                      className="size-10 text-muted-foreground"
                      strokeWidth={1.5}
                    />
                  </span>
                  <CardText
                    title="2D interviewer"
                    detail="An animated voice waveform."
                  />
                  <RadioGroupItem
                    id={`${ids}-2d`}
                    value={NO_AVATAR}
                    className="absolute top-2.5 left-2.5 bg-background"
                  />
                </label>
                {avatars.isPending &&
                  [0, 1].map((slot) => (
                    <div
                      key={slot}
                      role="presentation"
                      className="overflow-hidden rounded-xl border"
                    >
                      <Skeleton className="aspect-[4/5] rounded-none" />
                      <div className="space-y-2 p-3">
                        <Skeleton className="h-4 w-2/3" />
                        <Skeleton className="h-3 w-full" />
                      </div>
                    </div>
                  ))}
                {avatarPage?.items.map((avatar) => (
                  <label
                    key={avatar.id}
                    htmlFor={`${ids}-${avatar.id}`}
                    className={cardClass}
                  >
                    <AvatarPortrait avatar={avatar} userId={user.id} />
                    <CardText title={avatar.name} detail={avatar.personality} />
                    <RadioGroupItem
                      id={`${ids}-${avatar.id}`}
                      value={avatar.id}
                      className="absolute top-2.5 left-2.5 bg-background"
                    />
                  </label>
                ))}
              </RadioGroup>
              {avatarGone(field.value) && (
                <p role="status" className="text-sm text-muted-foreground">
                  The avatar you chose is no longer in your inventory, so the 2D
                  interviewer is selected.
                </p>
              )}
            </>
          )}
        />

        {pageCount > 1 && (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Button
              type="button"
              variant="outline"
              className="h-11"
              disabled={page <= 1}
              onClick={() => setPage((value) => value - 1)}
            >
              Previous
            </Button>
            <span className="text-muted-foreground tabular-nums">
              Page {page} of {pageCount}
            </span>
            <Button
              type="button"
              variant="outline"
              className="h-11"
              disabled={page >= pageCount}
              onClick={() => setPage((value) => value + 1)}
            >
              Next
            </Button>
          </div>
        )}
        {avatars.isError && (
          <div
            role="alert"
            className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm"
          >
            <p>
              We couldn&apos;t load your avatars. You can still use the 2D
              interviewer.
            </p>
            <Button
              type="button"
              variant="outline"
              className="h-11 bg-background"
              disabled={avatars.isFetching}
              onClick={() => void avatars.refetch()}
            >
              Try again
            </Button>
          </div>
        )}
        {avatarPage && avatarPage.pagination.total === 0 && (
          <p className="text-sm text-muted-foreground">
            You don&apos;t have an avatar yet.{" "}
            <Link
              href={routes.avatarStudioNew}
              className="font-medium text-foreground underline underline-offset-4"
            >
              Create one in Avatar Studio
            </Link>{" "}
            to see it here.
          </p>
        )}
      </fieldset>

      <div className="max-w-md space-y-2">
        <h2 id={`${ids}-voice-label`} className="font-semibold">
          Voice
        </h2>
        <Controller
          control={form.control}
          name="voiceId"
          render={({ field }) => (
            <Select
              value={voiceGone(field.value) ? DEFAULT_VOICE : field.value}
              onValueChange={field.onChange}
              disabled={voices.isPending}
            >
              <SelectTrigger
                aria-labelledby={`${ids}-voice-label`}
                aria-describedby={`${ids}-voice-help`}
                className="h-11 w-full min-w-0"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={DEFAULT_VOICE}>Default voice</SelectItem>
                {voices.data?.map((voice) => (
                  <SelectItem key={voice.id} value={voice.id}>
                    {voice.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <p
          id={`${ids}-voice-help`}
          className="text-xs text-muted-foreground"
          role={voices.isError ? "alert" : undefined}
        >
          {voices.isPending
            ? "Loading voices…"
            : voices.isError
              ? "We couldn't load the voices. The default voice is used."
              : voices.data.length === 0
                ? "No voices are available yet. The default voice is used."
                : voiceGone(form.getValues("voiceId"))
                  ? "The voice you chose is no longer available, so the default voice is selected."
                  : "Only active voices are listed."}
        </p>
        {voices.isError && (
          <Button
            type="button"
            variant="outline"
            className="h-11"
            onClick={() => void voices.refetch()}
          >
            Try again
          </Button>
        )}
      </div>

      <div className="flex flex-col-reverse gap-3 border-t pt-6 sm:flex-row sm:items-center sm:justify-between">
        <Button asChild variant="ghost" className="h-11">
          <Link
            href={setupHref(routes.interviews.new.setup, { roleProfileId })}
          >
            <ArrowLeft aria-hidden="true" /> Back to Role Profile
          </Link>
        </Button>
        <Button type="submit" className="h-11">
          Continue to preflight <ArrowRight aria-hidden="true" />
        </Button>
      </div>
    </form>
  );
}

function CardText({ title, detail }: { title: string; detail: string }) {
  return (
    <span className="flex flex-1 flex-col gap-1 p-3">
      <span className="truncate text-sm font-semibold" title={title}>
        {title}
      </span>
      <span className="line-clamp-2 text-xs leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">
        {detail}
      </span>
    </span>
  );
}
