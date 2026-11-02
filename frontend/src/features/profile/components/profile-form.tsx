"use client";

import { useEffect, useMemo, useRef, type FormEvent } from "react";
import { useTranslations } from "next-intl";
import { useForm, type UseFormRegisterReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle, Building2, UserRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import {
  createProfileFormSchema,
  normalizeHttpsProfileUrlInput,
  profilePatch,
  profileValues,
  type Profile,
  type ProfilePatch,
  type ProfileValues,
} from "../schemas/profile-schema";

function ProfileField({
  id,
  label,
  hint,
  error,
  registration,
  disabled,
  type = "text",
}: {
  id: string;
  label: string;
  hint?: string;
  error?: string;
  registration: UseFormRegisterReturn;
  disabled: boolean;
  type?: string;
}) {
  return (
    <div className="space-y-2">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <Input
        id={id}
        type={type}
        disabled={disabled}
        className="h-11"
        aria-invalid={!!error}
        aria-describedby={
          [hint ? `${id}-hint` : "", error ? `${id}-error` : ""]
            .filter(Boolean)
            .join(" ") || undefined
        }
        {...registration}
      />
      {hint && (
        <p
          id={`${id}-hint`}
          className="text-xs leading-relaxed text-muted-foreground"
        >
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
export function ProfileForm({
  profile,
  onSave,
  saving,
  error,
  saved,
  savedProfile,
}: {
  profile: Profile;
  onSave: (patch: ProfilePatch) => Promise<void>;
  saving: boolean;
  error: string | null;
  saved: boolean;
  savedProfile?: Profile;
}) {
  const t = useTranslations("common.profile");
  const schema = useMemo(
    () =>
      createProfileFormSchema(profile, {
        name: t("validation.name"),
        image: t("validation.image"),
        companyLength: t("validation.companyLength"),
        website: t("validation.website"),
      }),
    [profile, t],
  );
  const baseline = useRef(profile);
  const lastSaved = useRef<Profile | undefined>(undefined);
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    formState: { errors, isDirty, isValid },
  } = useForm<ProfileValues>({
    resolver: zodResolver(schema),
    defaultValues: profileValues(profile),
    mode: "onChange",
  });
  const companyWebsiteRegistration = register("company_website");
  useEffect(() => {
    // New server data reconciles pristine forms; a dirty draft stays untouched.
    if (savedProfile && savedProfile !== lastSaved.current) {
      lastSaved.current = savedProfile;
      baseline.current = savedProfile;
      reset(profileValues(savedProfile));
    } else if (profile !== baseline.current && !isDirty) {
      // Query structural sharing can clone a saved response. Updating only the
      // object identity must not schedule another reset over the next draft.
      const values = profileValues(profile);
      const changed =
        Object.keys(profilePatch(values, baseline.current)).length > 0;
      baseline.current = profile;
      if (changed) reset(values);
    }
  }, [profile, savedProfile, isDirty, reset]);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    void handleSubmit(async (values) => {
      const patch = profilePatch(values, baseline.current);
      if (!Object.keys(patch).length) {
        reset(profileValues(baseline.current));
        return;
      }
      await onSave(patch);
    })(event);
  };
  return (
    <form
      aria-label={t("form.label")}
      noValidate
      onSubmit={submit}
      className="min-w-0 space-y-6"
    >
      <Card className="gap-5 py-6">
        <CardHeader className="px-6">
          <div className="mb-1 flex items-center gap-2 text-muted-foreground">
            <UserRound className="size-4" aria-hidden />
            <span className="text-xs font-semibold tracking-wider uppercase">
              {t("form.personal")}
            </span>
          </div>
          <CardTitle>
            <h2 className="text-lg font-semibold">{t("form.personalTitle")}</h2>
          </CardTitle>
          <CardDescription>{t("form.personalDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5 px-6">
          <ProfileField
            id="profile-name"
            label={t("form.displayName")}
            registration={register("full_name")}
            disabled={saving}
            error={errors.full_name?.message}
          />
          <ProfileField
            id="profile-image"
            label={t("form.avatarUrl")}
            type="url"
            hint={t("form.avatarHint")}
            registration={register("image")}
            disabled={saving}
            error={errors.image?.message}
          />
        </CardContent>
      </Card>
      {profile.role === "recruiter" && (
        <Card className="gap-5 py-6">
          <CardHeader className="px-6">
            <div className="mb-1 flex items-center gap-2 text-muted-foreground">
              <Building2 className="size-4" aria-hidden />
              <span className="text-xs font-semibold tracking-wider uppercase">
                {t("form.company")}
              </span>
            </div>
            <CardTitle>
              <h2 className="text-lg font-semibold">
                {t("form.companyTitle")}
              </h2>
            </CardTitle>
            <CardDescription>{t("form.companyDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5 px-6">
            <ProfileField
              id="profile-company"
              label={t("form.companyName")}
              registration={register("company_name")}
              disabled={saving}
              error={errors.company_name?.message}
            />
            <ProfileField
              id="profile-website"
              label={t("form.companyWebsite")}
              type="url"
              hint={t("form.websiteHint")}
              registration={{
                ...companyWebsiteRegistration,
                onBlur: async (event) => {
                  const normalized = normalizeHttpsProfileUrlInput(
                    (event.target as HTMLInputElement).value,
                  );
                  if (normalized !== (event.target as HTMLInputElement).value)
                    setValue("company_website", normalized, {
                      shouldDirty: true,
                      shouldTouch: true,
                      shouldValidate: true,
                    });
                  await companyWebsiteRegistration.onBlur(event);
                },
              }}
              disabled={saving}
              error={errors.company_website?.message}
            />
          </CardContent>
        </Card>
      )}
      {error && (
        <div
          role="alert"
          className="rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm text-destructive"
        >
          {error}
        </div>
      )}
      <div className="flex flex-col gap-4 border-t pt-5 sm:flex-row sm:items-center sm:justify-between">
        <p
          role="status"
          aria-live="polite"
          className="text-sm text-muted-foreground"
        >
          {saving
            ? t("form.saving")
            : isDirty
              ? t("form.unsaved")
              : saved
                ? t("form.saved")
                : t("form.idle")}
        </p>
        <div className="flex shrink-0 gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={!isDirty || saving}
            onClick={() => {
              baseline.current = profile;
              reset(profileValues(profile));
            }}
          >
            {t("form.discard")}
          </Button>
          <Button
            type="submit"
            className="bg-workspace-accent text-white hover:bg-workspace-accent/90"
            disabled={!isDirty || !isValid || saving}
            aria-busy={saving}
          >
            {saving && (
              <LoaderCircle
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden
              />
            )}
            {saving ? t("form.savingButton") : t("form.save")}
          </Button>
        </div>
      </div>
    </form>
  );
}
