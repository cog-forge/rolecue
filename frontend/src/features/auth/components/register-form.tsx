"use client";

import { RoleCueMark } from "@/components/brand/rolecue-mark";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { routes } from "@/config/routes";
import { getAuthErrorMessage } from "@/features/auth/utils/get-auth-error-message";
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
  createRegisterSchema,
  type RegisterFormValues,
} from "../schemas/register-schema";
import { useTranslations } from "next-intl";

// Better Auth requires a name during sign-up; onboarding collects the real name.
const SIGN_UP_PLACEHOLDER_NAME = "New user";

export function RegisterForm() {
  const t = useTranslations("auth.register");
  const v = useTranslations("validation");
  const registerSchema = useMemo(() => createRegisterSchema(v), [v]);
  const te = useTranslations("errors");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [submittedEmail, setSubmittedEmail] = useState("");
  const [message, setMessage] = useState("");
  const [isResending, setIsResending] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting, submitCount },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      email: "",
      password: "",
      confirmPassword: "",
    },
  });
  useShakeInvalidFields(formRef, submitCount);

  const submit = async ({ email, password }: RegisterFormValues) => {
    setMessage("");
    try {
      const result = await authClient.signUp.email({
        name: SIGN_UP_PLACEHOLDER_NAME,
        email,
        password,
        callbackURL: routes.afterEmailVerification,
      });
      if (result.error) {
        setMessage(getAuthErrorMessage(result.error, t("createFailed"), te));
        gooeyToast.error(t("failedTitle"), {
          description: getAuthErrorMessage(result.error, t("later"), te),
        });
        return;
      }
      setSubmittedEmail(email);
      gooeyToast.success(t("created"), {
        description: t("verifyHint"),
      });
    } catch {
      setMessage(t("connectionError"));
      gooeyToast.error(t("failedTitle"), {
        description: t("connectionHint"),
      });
    }
  };

  const resend = async () => {
    setIsResending(true);
    try {
      const result = await authClient.sendVerificationEmail({
        email: submittedEmail,
        callbackURL: routes.afterEmailVerification,
      });
      if (result.error) throw new Error("Verification email failed");
      setMessage(t("sentMessage"));
      gooeyToast.success(t("sent"), {
        description: t("inboxHint"),
      });
    } catch {
      setMessage(t("resendFailed"));
      gooeyToast.error(t("emailFailed"), {
        description: t("later"),
      });
    } finally {
      setIsResending(false);
    }
  };

  if (submittedEmail) {
    return (
      <section className="w-full text-center motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-1 motion-safe:duration-200">
        <RoleCueMark className="mx-auto size-28 object-contain" priority />
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-[#121814]">
          {t("checkEmail")}
        </h1>
        <p className="mt-2 text-sm leading-6 text-[#5c5c5c]">
          {t.rich("verification", {
            email: submittedEmail,
            emailTag: (chunks) => <strong>{chunks}</strong>,
          })}
        </p>
        {message && (
          <p
            role="status"
            aria-live="polite"
            className="mt-4 text-sm text-[#595959]"
          >
            {message}
          </p>
        )}
        <Button
          type="button"
          variant="outline"
          onClick={resend}
          disabled={isResending}
          aria-busy={isResending}
          className="mt-5 h-11 w-full rounded-xl transition-colors duration-150 sm:h-12"
        >
          {isResending && <LoaderCircle className="animate-spin" aria-hidden />}
          {isResending ? t("sending") : t("resend")}
        </Button>
        <p className="mt-5 text-sm text-[#5c5c5c]">
          {t("alreadyVerified")}{" "}
          <Link
            href={routes.login}
            className="font-semibold text-[#121814] hover:underline"
          >
            {t("signIn")}
          </Link>
        </p>
      </section>
    );
  }

  return (
    <div className="w-full">
      <header className="mb-6 text-center">
        <RoleCueMark className="mx-auto size-28 object-contain" priority />
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-[#121814]">
          {t("heading")}
        </h1>
        <p className="mt-1.5 text-sm text-[#5c5c5c]">{t("description")}</p>
      </header>
      <form
        ref={formRef}
        onSubmit={handleSubmit(submit)}
        noValidate
        className="space-y-4"
      >
        <Field
          id="register-email"
          label={t("email")}
          error={errors.email?.message}
        >
          <Input
            id="register-email"
            type="email"
            autoComplete="email"
            aria-invalid={!!errors.email}
            aria-describedby={errors.email ? "register-email-error" : undefined}
            data-auth-input
            className={`t-input h-11 rounded-xl border-[#d9d9d9] bg-white px-3.5 text-sm text-[#262626] placeholder:text-[#8c8c8c] focus-visible:border-rolecue-brand focus-visible:ring-rolecue-brand/15 sm:h-12 ${errors.email ? "is-error" : ""}`}
            {...register("email")}
          />
        </Field>
        <Field
          id="register-password"
          label={t("password")}
          error={errors.password?.message}
        >
          <div className="relative">
            <Input
              id="register-password"
              type={showPassword ? "text" : "password"}
              autoComplete="new-password"
              aria-invalid={!!errors.password}
              aria-describedby={
                errors.password
                  ? "register-password-error"
                  : "register-password-help"
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
              aria-controls="register-password"
              aria-pressed={showPassword}
              className="absolute top-1/2 right-1 size-11 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-[#595959] hover:text-[#262626]"
            >
              <PasswordVisibilityIcon visible={showPassword} />
            </Button>
          </div>
          {!errors.password && (
            <p id="register-password-help" className="text-xs text-[#595959]">
              {t("passwordHint")}
            </p>
          )}
        </Field>
        <Field
          id="register-confirm-password"
          label={t("confirm")}
          error={errors.confirmPassword?.message}
        >
          <div className="relative">
            <Input
              id="register-confirm-password"
              type={showConfirmPassword ? "text" : "password"}
              autoComplete="new-password"
              aria-invalid={!!errors.confirmPassword}
              aria-describedby={
                errors.confirmPassword
                  ? "register-confirm-password-error"
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
              aria-controls="register-confirm-password"
              aria-pressed={showConfirmPassword}
              className="absolute top-1/2 right-1 size-11 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-[#595959] hover:text-[#262626]"
            >
              <PasswordVisibilityIcon visible={showConfirmPassword} />
            </Button>
          </div>
        </Field>
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
          {isSubmitting ? t("creating") : t("create")}
        </Button>
      </form>
      <p className="mt-4 text-center text-xs leading-5 text-[#595959]">
        {t.rich("consent", {
          terms: (chunks) => (
            <Link
              href={routes.terms}
              className="font-medium text-[#262626] underline underline-offset-2"
            >
              {chunks}
            </Link>
          ),
          privacy: (chunks) => (
            <Link
              href={routes.privacy}
              className="font-medium text-[#262626] underline underline-offset-2"
            >
              {chunks}
            </Link>
          ),
        })}
      </p>
      <p className="mt-5 text-center text-sm text-[#5c5c5c]">
        {t("alreadyAccount")}{" "}
        <Link
          href={routes.login}
          className="font-semibold text-[#121814] hover:underline"
        >
          {t("signIn")}
        </Link>
      </p>
    </div>
  );
}

function Field({
  id,
  label,
  error,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={`t-input-wrap space-y-1.5 ${error ? "is-error" : ""}`}>
      <label
        htmlFor={id}
        className="block text-xs font-semibold uppercase tracking-wider text-[#494949]"
      >
        {label}
      </label>
      {children}
      {error && (
        <p
          id={`${id}-error`}
          role="alert"
          className="t-error-msg text-xs font-medium text-destructive"
        >
          {error}
        </p>
      )}
    </div>
  );
}
