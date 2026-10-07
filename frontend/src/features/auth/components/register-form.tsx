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
import { useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { gooeyToast } from "goey-toast";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import {
  registerSchema,
  type RegisterFormValues,
} from "../schemas/register-schema";

// Better Auth requires a name during sign-up; onboarding collects the real name.
const SIGN_UP_PLACEHOLDER_NAME = "New user";

export function RegisterForm() {
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
        setMessage(
          getAuthErrorMessage(
            result.error,
            "Unable to create your account. Please try again.",
          ),
        );
        gooeyToast.error("Account could not be created", {
          description: getAuthErrorMessage(
            result.error,
            "Please try again in a moment.",
          ),
        });
        return;
      }
      setSubmittedEmail(email);
      gooeyToast.success("Account created", {
        description:
          "Check your email to verify your account before signing in.",
      });
    } catch {
      setMessage(
        "Unable to create your account. Please check your connection and try again.",
      );
      gooeyToast.error("Account could not be created", {
        description: "Please check your connection and try again.",
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
      setMessage("Verification email sent.");
      gooeyToast.success("Verification email sent", {
        description: "Check your inbox and spam folder for the link.",
      });
    } catch {
      setMessage("Unable to resend the email right now.");
      gooeyToast.error("Email could not be sent", {
        description: "Please try again in a moment.",
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
          Check your email
        </h1>
        <p className="mt-2 text-sm leading-6 text-[#5c5c5c]">
          We sent a verification link to <strong>{submittedEmail}</strong>.
          Verify your email to finish creating your account.
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
          {isResending ? "Sending…" : "Resend verification email"}
        </Button>
        <p className="mt-5 text-sm text-[#5c5c5c]">
          Already verified?{" "}
          <Link
            href={routes.login}
            className="font-semibold text-[#121814] hover:underline"
          >
            Sign in
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
          Create your RoleCue account
        </h1>
        <p className="mt-1.5 text-sm text-[#5c5c5c]">
          Start practicing for your next interview.
        </p>
      </header>
      <form
        ref={formRef}
        onSubmit={handleSubmit(submit)}
        noValidate
        className="space-y-4"
      >
        <Field
          id="register-email"
          label="Email address"
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
          label="Password"
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
              aria-label={showPassword ? "Hide password" : "Show password"}
              aria-controls="register-password"
              aria-pressed={showPassword}
              className="absolute top-1/2 right-1 size-11 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-[#595959] hover:text-[#262626]"
            >
              <PasswordVisibilityIcon visible={showPassword} />
            </Button>
          </div>
          {!errors.password && (
            <p id="register-password-help" className="text-xs text-[#595959]">
              Use at least 8 characters.
            </p>
          )}
        </Field>
        <Field
          id="register-confirm-password"
          label="Confirm password"
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
                showConfirmPassword
                  ? "Hide confirmation password"
                  : "Show confirmation password"
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
          {isSubmitting ? "Creating account…" : "Create account"}
        </Button>
      </form>
      <p className="mt-4 text-center text-xs leading-5 text-[#595959]">
        By creating an account, you agree to the{" "}
        <Link
          href={routes.terms}
          className="font-medium text-[#262626] underline underline-offset-2"
        >
          Terms of Service
        </Link>{" "}
        and acknowledge that you have read our{" "}
        <Link
          href={routes.privacy}
          className="font-medium text-[#262626] underline underline-offset-2"
        >
          Privacy Policy
        </Link>
        .
      </p>
      <p className="mt-5 text-center text-sm text-[#5c5c5c]">
        Already have an account?{" "}
        <Link
          href={routes.login}
          className="font-semibold text-[#121814] hover:underline"
        >
          Sign in
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
