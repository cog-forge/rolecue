"use client";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  ArrowRight,
  Building2,
  Globe2,
  ImagePlus,
  Link2,
  LoaderCircle,
  UserRound,
} from "lucide-react";
import { UserAvatar } from "@/components/account/user-avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  onSave: (patch: ProfilePatch) => Promise<void>;
}) {
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
    setFocus,
    setValue,
    formState: { errors },
  } = useForm<ProfileValues>({
    resolver: zodResolver(createOnboardingSchema(profile)),
    defaultValues: values,
    mode: "onBlur",
  });
  const [name, image, website] = useWatch({
    control,
    name: ["full_name", "image", "company_website"],
  });
  const recruiter = profile.role === "recruiter";
  const fields = [
    {
      key: "full_name" as const,
      label: "Display name",
      type: "text",
      required: true,
      placeholder: "How should we call you?",
      Icon: UserRound,
    },
    {
      key: "image" as const,
      label: "Avatar URL",
      type: "url",
      required: false,
      placeholder: "https://example.com/your-photo.jpg",
      Icon: Link2,
    },
    ...(recruiter
      ? [
          {
            key: "company_name" as const,
            label: "Company name",
            type: "text",
            required: true,
            placeholder: "Your company’s name",
            Icon: Building2,
          },
          {
            key: "company_website" as const,
            label: "Company website",
            type: "url",
            required: true,
            placeholder: "your-company.com",
            Icon: Globe2,
          },
        ]
      : []),
  ];
  return (
    <form
      noValidate
      aria-label="Your onboarding details"
      onSubmit={handleSubmit(async (v) => {
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
        await onSave(patch);
      })}
      className="space-y-5"
    >
      <div className="flex flex-col items-stretch gap-3 rounded-2xl border border-[#e8e3e7] bg-white/70 dark:border-[#3c4962] dark:bg-[#202b40] p-3.5 shadow-[0_3px_14px_-10px_rgba(20,33,60,0.2)] sm:flex-row sm:items-center sm:justify-between sm:p-4">
        <div className="flex min-w-0 items-center gap-3">
          <UserAvatar
            id={profile.id}
            name={name || profile.full_name}
            image={image || null}
            className="size-12 rounded-full ring-2 ring-white dark:ring-[#45516a] sm:size-13"
          />
          <div className="min-w-0 text-left">
            <p className="text-sm font-semibold text-[#202438] dark:text-[#eef2ff]">
              Your avatar
            </p>
            <p className="mt-0.5 text-xs leading-relaxed text-[#727b8d] dark:text-[#aab7cf]">
              {image ? "Your photo is ready" : "Your Peek avatar is ready"}
            </p>
          </div>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => setFocus("image")}
          disabled={saving}
          className="h-9 self-end rounded-lg border-[#dedfe7] bg-white px-3 text-xs text-[#283047] hover:bg-[#f5f6fa] dark:border-[#45516a] dark:bg-[#243049] dark:text-[#eef2ff] dark:hover:bg-[#2f4364]"
        >
          <ImagePlus className="size-3.5" aria-hidden />
          {image ? "Change photo" : "Add photo URL"}
        </Button>
      </div>
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
                {!field.required && (
                  <span className="ml-2 text-xs font-normal text-[#8b91a0] dark:text-[#aab7cf]">
                    Optional
                  </span>
                )}
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
                  required={field.required}
                  disabled={saving}
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
                      : field.key === "image"
                        ? "avatar-hint"
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
              {field.key === "image" && (
                <p
                  id="avatar-hint"
                  className="text-xs text-[#80899a] dark:text-[#aab7cf]"
                >
                  Leave blank to use your Peek avatar.
                </p>
              )}
              {field.key === "company_website" && (
                <p
                  id="website-hint"
                  className="text-xs text-[#80899a] dark:text-[#aab7cf]"
                >
                  We’ll add https:// automatically if you enter a domain.
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
          disabled={saving}
          className="h-12 w-full rounded-xl bg-[#202438] text-sm text-white shadow-[0_8px_22px_-12px_rgba(32,36,56,0.7)] hover:bg-[#323950] dark:bg-[#9abbff] dark:text-[#12213a] dark:hover:bg-[#b9d0ff]"
        >
          {saving && (
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
          )}
          {saving ? "Saving your profile…" : "Open my workspace"}
          {!saving && <ArrowRight className="ml-1 size-4" aria-hidden />}
        </Button>
        <p className="text-center text-xs leading-relaxed text-[#858d9c] dark:text-[#aab7cf]">
          You can edit these details later in your profile.
        </p>
      </div>
    </form>
  );
}
