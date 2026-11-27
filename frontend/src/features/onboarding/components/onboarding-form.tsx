"use client";
import { useState } from "react";
import { useTranslations } from "next-intl";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  ArrowRight,
  Building2,
  Globe2,
  LoaderCircle,
  UserRound,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ProfilePhotoUpload } from "@/features/profile/api/profile";
import {
  ProfilePhotoField,
  type ProfilePhotoDraft,
} from "@/features/profile/components/profile-photo-field";
import {
  profileValues,
  profilePatch,
  type Profile,
  type ProfileValues,
  type ProfilePatch,
  normalizeHttpsProfileUrlInput,
} from "@/features/profile/schemas/profile-schema";
import { createOnboardingSchema } from "../schemas/onboarding-schema";
import { WebsitePreview } from "./website-preview";

export function OnboardingForm({
  profile,
  saving,
  error,
  onSave,
}: {
  profile: Profile;
  saving: boolean;
  error: string | null;
  onSave: (patch: ProfilePatch, photo?: ProfilePhotoUpload) => Promise<void>;
}) {
  const t = useTranslations("auth.onboarding");
  const [photo, setPhoto] = useState<ProfilePhotoDraft>(undefined);
  const [progress, setProgress] = useState<number | null>(null);
  const values = profileValues(profile);
  if (
    ["new user", "new candidate", "new recruiter"].includes(
      values.full_name.trim().toLowerCase(),
    )
  )
    values.full_name = "";
  const {
    register,
    handleSubmit,
    control,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<ProfileValues>({
    resolver: zodResolver(
      createOnboardingSchema(profile, {
        name: t("nameValidation"),
        companyLength: t("companyLength"),
        website: t("websiteValidation"),
        companyRequired: t("companyRequired"),
        websiteRequired: t("websiteRequired"),
        companyWebsiteInvalid: t("companyWebsiteInvalid"),
      }),
    ),
    defaultValues: values,
    mode: "onBlur",
  });
  const [name, website] = useWatch({
    control,
    name: ["full_name", "company_website"],
  });
  const busy = saving || isSubmitting;
  const recruiter = profile.role === "recruiter";
  const fields = [
    {
      key: "full_name" as const,
      label: t("displayName"),
      type: "text",
      placeholder: t("displayPlaceholder"),
      Icon: UserRound,
    },
    ...(recruiter
      ? [
          {
            key: "company_name" as const,
            label: t("companyName"),
            type: "text",
            placeholder: t("companyPlaceholder"),
            Icon: Building2,
          },
          {
            key: "company_website" as const,
            label: t("companyWebsite"),
            type: "url",
            placeholder: "your-company.com",
            Icon: Globe2,
          },
        ]
      : []),
  ];
  return (
    <form
      noValidate
      aria-label={t("form")}
      onSubmit={(event) => {
        if (busy) {
          event.preventDefault();
          return;
        }
        void handleSubmit(async (v) => {
          const patch: ProfilePatch = {
            ...profilePatch(v, profile),
            full_name: v.full_name.trim(),
          };
          if (recruiter) {
            patch.company_name = v.company_name.trim();
            patch.company_website = normalizeHttpsProfileUrlInput(
              v.company_website,
            );
          }
          if (photo === null) patch.image = null;
          if (!photo) {
            await onSave(patch);
            return;
          }
          try {
            await onSave(patch, { file: photo.file, onProgress: setProgress });
          } finally {
            setProgress(null);
          }
        })(event);
      }}
      className="space-y-5"
    >
      <ProfilePhotoField
        userId={profile.id}
        name={name || profile.full_name}
        image={profile.image}
        draft={photo}
        onChange={setPhoto}
        disabled={busy}
        progress={progress}
        showCurrent
        description={
          photo || (photo !== null && profile.image)
            ? t("photoReady")
            : t("peekReady")
        }
      />
      <div className="grid gap-x-4 gap-y-4 sm:grid-cols-2">
        {fields.map((field) => {
          const registration = register(field.key);
          return (
            <div className="space-y-2" key={field.key}>
              <label
                htmlFor={field.key}
                className="text-[13px] font-semibold text-[#30374b] dark:text-[#eef2ff]"
              >
                {field.label}
              </label>
              <div className="relative">
                <field.Icon
                  className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-[#8b94a7] dark:text-[#aab7cf]"
                  aria-hidden
                />
                <Input
                  id={field.key}
                  {...registration}
                  type={field.type}
                  placeholder={field.placeholder}
                  required
                  disabled={busy}
                  onBlur={(event) => {
                    if (field.key === "company_website") {
                      const normalized = normalizeHttpsProfileUrlInput(
                        event.currentTarget.value,
                      );
                      if (normalized !== event.currentTarget.value)
                        setValue("company_website", normalized, {
                          shouldDirty: true,
                          shouldTouch: true,
                          shouldValidate: true,
                        });
                    }
                    void registration.onBlur(event);
                  }}
                  aria-invalid={!!errors[field.key]}
                  aria-describedby={
                    errors[field.key]
                      ? `${field.key}-error`
                      : field.key === "company_website"
                        ? "website-hint"
                        : undefined
                  }
                  className="h-11 rounded-xl border-[#e2e2e9] bg-white/90 pl-10 text-[#202438] shadow-none placeholder:text-[#a0a6b3] focus-visible:border-[#7189bc] focus-visible:ring-[#a5b8dd]/40 dark:border-[#45516a] dark:bg-[#202b40] dark:text-[#eef2ff] dark:placeholder:text-[#93a2be] dark:focus-visible:border-[#9abbff] dark:focus-visible:ring-[#9abbff]/40"
                  autoComplete={
                    field.key === "full_name"
                      ? "name"
                      : field.key === "company_name"
                        ? "organization"
                        : "url"
                  }
                />
              </div>
              {field.key === "company_website" && (
                <p
                  id="website-hint"
                  className="text-xs text-[#80899a] dark:text-[#aab7cf]"
                >
                  {t("websiteHint")}
                </p>
              )}
              {errors[field.key] && (
                <p
                  role="alert"
                  id={`${field.key}-error`}
                  className="text-xs text-destructive"
                >
                  {errors[field.key]?.message}
                </p>
              )}
            </div>
          );
        })}
      </div>
      {recruiter && <WebsitePreview website={website} />}
      {error && (
        <p
          role="alert"
          className="rounded-lg border border-destructive/25 bg-destructive/5 p-3 text-sm text-destructive"
        >
          {error}
        </p>
      )}
      <div className="space-y-3 pt-1">
        <Button
          type="submit"
          size="lg"
          disabled={busy}
          className="h-12 w-full rounded-xl bg-[#202438] text-sm text-white shadow-[0_8px_22px_-12px_rgba(32,36,56,0.7)] hover:bg-[#323950] dark:bg-[#9abbff] dark:text-[#12213a] dark:hover:bg-[#b9d0ff]"
        >
          {busy && (
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
          )}
          {busy ? t("saving") : t("open")}
          {!busy && <ArrowRight className="ml-1 size-4" aria-hidden />}
        </Button>
        <p className="text-center text-xs leading-relaxed text-[#858d9c] dark:text-[#aab7cf]">
          {t("later")}
        </p>
      </div>
    </form>
  );
}
