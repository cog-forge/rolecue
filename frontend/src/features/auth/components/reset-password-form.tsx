"use client";

import { RoleCueMark } from "@/components/brand/rolecue-mark";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { routes } from "@/config/routes";
import { authClient } from "@/lib/auth/client";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { PasswordVisibilityIcon } from "@/components/icons/password-visibility-icon";
import Link from "next/link";
import { useMemo, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { gooeyToast } from "goey-toast";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import {
  createResetPasswordSchema,
  type ResetPasswordValues,
} from "../schemas/recovery-schema";
import { useTranslations } from "next-intl";

export function ResetPasswordForm({ token }: { token?: string }) {
  const t = useTranslations("auth.resetPassword");
  const v = useTranslations("validation");
  const resetPasswordSchema = useMemo(() => createResetPasswordSchema(v), [v]);
  const [complete, setComplete] = useState(false);
  const [message, setMessage] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting, submitCount },
  } = useForm<ResetPasswordValues>({
    resolver: zodResolver(resetPasswordSchema),
  });
  useShakeInvalidFields(formRef, submitCount);
  const submit = async ({ password }: ResetPasswordValues) => {
    try {
      const result = await authClient.resetPassword({
        newPassword: password,
        token,
      });
      if (result.error) {
        setMessage(t("invalid"));
        gooeyToast.error(t("failed"), {
          description: t("invalidToast"),
        });
        return;
      }
      setComplete(true);
      gooeyToast.success(t("updated"), {
        description: t("signInNew"),
      });
    } catch {
      setMessage(t("error"));
      gooeyToast.error(t("failed"), {
        description: t("errorToast"),
      });
    }
  };

  if (!token) {
    return (
      <div className="w-full text-center">
        <header className="mb-6">
          <RoleCueMark className="mx-auto size-28 object-contain" priority />
          <h1 className="mt-2 text-2xl font-semibold tracking-tight text-[#262626]">
            {t("unavailable")}
          </h1>
          <p className="mt-1.5 text-sm leading-6 text-[#595959]">
            {t("missing")}
          </p>
        </header>
        <Button
          asChild
          className="h-11 w-full rounded-xl bg-[#262626] text-white transition-colors duration-150 hover:bg-[#1f1f1f] sm:h-12"
        >
          <Link href={routes.forgotPassword}>{t("request")}</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="w-full">
      <header className="mb-6 text-center">
        <RoleCueMark className="mx-auto size-28 object-contain" priority />
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-[#121814]">
          {t("heading")}
        </h1>
        <p className="mt-1.5 text-sm text-[#5c5c5c]">{t("help")}</p>
      </header>
      {complete ? (
        <div
          role="status"
          aria-live="polite"
          className="text-center text-sm leading-6 text-[#595959] motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-1 motion-safe:duration-200"
        >
          {t.rich("complete", {
            link: (chunks) => (
              <Link
                href={routes.login}
                className="font-semibold text-[#121814] hover:underline"
              >
                {chunks}
              </Link>
            ),
          })}
        </div>
      ) : (
        <form
          ref={formRef}
          onSubmit={handleSubmit(submit)}
          noValidate
          className="space-y-4"
        >
          <div
            className={`t-input-wrap space-y-1.5 ${errors.password ? "is-error" : ""}`}
          >
            <label
              htmlFor="reset-password"
              className="block text-xs font-semibold uppercase tracking-wider text-[#494949]"
            >
              {t("password")}
            </label>
            <div className="relative">
              <Input
                id="reset-password"
                type={showPassword ? "text" : "password"}
                autoComplete="new-password"
                aria-invalid={!!errors.password}
                aria-describedby={
                  errors.password
                    ? "reset-password-error"
                    : "reset-password-help"
                }
                data-auth-input
                className={`t-input h-11 rounded-xl border-[#d9d9d9] bg-white px-3.5 pr-12 text-sm text-[#262626] placeholder:text-[#8c8c8c] focus-visible:border-rolecue-brand focus-visible:ring-rolecue-brand/15 sm:h-12 ${errors.password ? "is-error" : ""}`}
                {...register("password")}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => setShowPassword((visible) => !visible)}
                aria-label={showPassword ? t("hide") : t("show")}
                aria-controls="reset-password"
                aria-pressed={showPassword}
                className="absolute top-1/2 right-1 size-11 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-[#595959] hover:text-[#262626]"
              >
                <PasswordVisibilityIcon visible={showPassword} />
              </Button>
            </div>
            {!errors.password && (
              <p id="reset-password-help" className="text-xs text-[#595959]">
                {t("help")}
              </p>
            )}
            {errors.password && (
              <p
                id="reset-password-error"
                role="alert"
                className="t-error-msg text-xs text-destructive"
              >
                {errors.password.message}
              </p>
            )}
          </div>
          <div
            className={`t-input-wrap space-y-1.5 ${errors.confirmPassword ? "is-error" : ""}`}
          >
            <label
              htmlFor="reset-confirm-password"
              className="block text-xs font-semibold uppercase tracking-wider text-[#494949]"
            >
              {t("confirm")}
            </label>
            <div className="relative">
              <Input
                id="reset-confirm-password"
                type={showConfirmPassword ? "text" : "password"}
                autoComplete="new-password"
                aria-invalid={!!errors.confirmPassword}
                aria-describedby={
                  errors.confirmPassword
                    ? "reset-confirm-password-error"
                    : undefined
                }
                data-auth-input
                className={`t-input h-11 rounded-xl border-[#d9d9d9] bg-white px-3.5 pr-12 text-sm text-[#262626] placeholder:text-[#8c8c8c] focus-visible:border-rolecue-brand focus-visible:ring-rolecue-brand/15 sm:h-12 ${errors.confirmPassword ? "is-error" : ""}`}
                {...register("confirmPassword")}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => setShowConfirmPassword((visible) => !visible)}
                aria-label={
                  showConfirmPassword ? t("hideConfirm") : t("showConfirm")
                }
                aria-controls="reset-confirm-password"
                aria-pressed={showConfirmPassword}
                className="absolute top-1/2 right-1 size-11 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-[#595959] hover:text-[#262626]"
              >
                <PasswordVisibilityIcon visible={showConfirmPassword} />
              </Button>
            </div>
            {errors.confirmPassword && (
              <p
                id="reset-confirm-password-error"
                role="alert"
                className="t-error-msg text-xs text-destructive"
              >
                {errors.confirmPassword.message}
              </p>
            )}
          </div>
          {message && (
            <p role="alert" className="text-sm text-destructive">
              {message}
            </p>
          )}
          <Button
            type="submit"
            disabled={isSubmitting}
            aria-busy={isSubmitting}
            className="h-11 w-full rounded-xl bg-[#262626] text-white transition-colors duration-150 hover:bg-[#1f1f1f] sm:h-12"
          >
            {isSubmitting && (
              <LoaderCircle className="animate-spin" aria-hidden />
            )}
            {isSubmitting ? t("updating") : t("update")}
          </Button>
        </form>
      )}
      {!complete && (
        <p className="mt-5 text-center text-sm text-[#595959]">
          <Link
            href={routes.forgotPassword}
            className="font-semibold text-[#121814] hover:underline"
          >
            {t("request")}
          </Link>
        </p>
      )}
    </div>
  );
}
