"use client";

import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowRight, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  avatarMetadataSchema,
  type AvatarMetadata,
} from "../schemas/avatar-schema";

const personalities = [
  {
    label: "Supportive",
    value:
      "Patient and encouraging. Ask clear questions and help me feel comfortable sharing my thought process.",
  },
  {
    label: "Direct",
    value:
      "Concise and professional. Ask focused questions and challenge me to explain my decisions clearly.",
  },
  {
    label: "Technical",
    value:
      "Curious and analytical. Explore technical tradeoffs and ask thoughtful follow-up questions.",
  },
];

export function AvatarDetailsForm({
  disabled,
  onStart,
  initialValues,
}: {
  disabled: boolean;
  onStart: (metadata: AvatarMetadata) => Promise<void>;
  initialValues?: AvatarMetadata;
}) {
  const {
    register,
    control,
    setValue,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<AvatarMetadata>({
    resolver: zodResolver(avatarMetadataSchema),
    defaultValues: initialValues ?? { name: "", personality: "" },
  });
  const personality = useWatch({ control, name: "personality" });
  return (
    <form
      aria-label="Avatar details"
      className="space-y-6"
      noValidate
      onSubmit={handleSubmit(onStart)}
    >
      <div className="space-y-2">
        <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
          Step 01 / Details
        </p>
        <h2 className="text-2xl font-semibold tracking-tight">
          Give your interviewer a character.
        </h2>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Start with a name and a personality. You’ll create the appearance
          next.
        </p>
      </div>
      <div className="space-y-2">
        <label htmlFor="avatar-name" className="text-sm font-medium">
          Avatar name
        </label>
        <Input
          id="avatar-name"
          className="h-11"
          placeholder="Give your interviewer a name"
          autoComplete="off"
          {...register("name")}
          aria-invalid={!!errors.name}
          aria-describedby={errors.name ? "avatar-name-error" : undefined}
          disabled={disabled || isSubmitting}
        />
        {errors.name && (
          <p
            id="avatar-name-error"
            role="alert"
            className="text-sm text-destructive"
          >
            {errors.name.message}
          </p>
        )}
      </div>
      <div className="space-y-2">
        <label htmlFor="avatar-personality" className="text-sm font-medium">
          Personality
        </label>
        <div
          role="group"
          aria-label="Personality starting points"
          className="flex flex-wrap gap-2 py-1"
        >
          {personalities.map((preset) => (
            <Button
              key={preset.label}
              type="button"
              variant="outline"
              className="min-h-11 px-3 text-xs aria-pressed:border-rolecue-brand aria-pressed:bg-rolecue-brand/10"
              aria-pressed={personality === preset.value}
              disabled={disabled || isSubmitting}
              onClick={() =>
                setValue("personality", preset.value, {
                  shouldDirty: true,
                  shouldValidate: true,
                })
              }
            >
              {preset.label}
            </Button>
          ))}
        </div>
        <Textarea
          id="avatar-personality"
          rows={4}
          placeholder="Describe your interviewer's style and personality"
          {...register("personality")}
          aria-invalid={!!errors.personality}
          aria-describedby={`avatar-personality-hint${errors.personality ? " avatar-personality-error" : ""}`}
          disabled={disabled || isSubmitting}
        />
        <p
          id="avatar-personality-hint"
          className="text-xs text-muted-foreground"
        >
          Choose a starting point or write your own. You can edit the text
          above.
        </p>
        {errors.personality && (
          <p
            id="avatar-personality-error"
            role="alert"
            className="text-sm text-destructive"
          >
            {errors.personality.message}
          </p>
        )}
      </div>
      <Button
        className="group min-h-12 w-full justify-between px-4 sm:w-auto sm:min-w-64"
        type="submit"
        disabled={disabled || isSubmitting}
      >
        {isSubmitting ? "Checking availability…" : "Open Avaturn creator"}
        {isSubmitting ? (
          <LoaderCircle
            aria-hidden
            className="size-4 animate-spin motion-reduce:animate-none"
          />
        ) : (
          <ArrowRight
            aria-hidden
            className="size-4 transition-transform duration-(--duration-fast) group-hover:translate-x-1 motion-reduce:transform-none motion-reduce:transition-none"
          />
        )}
      </Button>
      <p className="text-xs leading-relaxed text-muted-foreground">
        Nothing is saved until you finish in Avaturn and RoleCue confirms the
        save.
      </p>
    </form>
  );
}
