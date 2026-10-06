"use client";

import { RoleCueMark } from "@/components/brand/rolecue-mark";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { routes } from "@/config/routes";
import { authClient } from "@/lib/auth/client";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import Link from "next/link";
import { useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { gooeyToast } from "goey-toast";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import {
  forgotPasswordSchema,
  type ForgotPasswordValues,
} from "../schemas/recovery-schema";

export function ForgotPasswordForm() {
  const [submitted, setSubmitted] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting, submitCount },
  } = useForm<ForgotPasswordValues>({
    resolver: zodResolver(forgotPasswordSchema),
  });
  useShakeInvalidFields(formRef, submitCount);
  const submit = async ({ email }: ForgotPasswordValues) => {
    try {
      await authClient.requestPasswordReset({
        email,
        redirectTo: routes.resetPassword,
      });
    } catch {
      // Keep the outcome generic if the request fails to avoid disclosing account state.
    } finally {
      // Keep the response generic so the UI does not reveal whether an account exists.
      setSubmitted(true);
      gooeyToast.info("Check your inbox", {
        description:
          "If an account exists for that email, you will receive a password reset link shortly.",
      });
    }
  };

  return (
    <div className="w-full">
      <header className="mb-6 text-center">
        <RoleCueMark className="mx-auto size-28 object-contain" priority />
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-[#121814]">
          Reset your password
        </h1>
        <p className="mt-1.5 text-sm leading-6 text-[#5c5c5c]">
          Enter your email and we’ll send a password reset link if an account
          exists.
        </p>
      </header>
      {submitted ? (
        <div
          role="status"
          aria-live="polite"
          className="rounded-xl border border-[#d9d9d9] bg-white p-4 text-center text-sm leading-6 text-[#595959] motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-1 motion-safe:duration-200"
        >
          If an account exists for that email, you’ll receive a password reset
          link shortly.
        </div>
      ) : (
        <form
          ref={formRef}
          onSubmit={handleSubmit(submit)}
          noValidate
          className="space-y-4"
        >
          <div
            className={`t-input-wrap space-y-1.5 ${errors.email ? "is-error" : ""}`}
          >
            <label
              htmlFor="forgot-email"
              className="block text-xs font-semibold uppercase tracking-wider text-[#494949]"
            >
              Email address
            </label>
            <Input
              id="forgot-email"
              type="email"
              autoComplete="email"
              aria-invalid={!!errors.email}
              aria-describedby={errors.email ? "forgot-email-error" : undefined}
              data-auth-input
              className={`t-input h-11 rounded-xl border-[#d9d9d9] bg-white px-3.5 text-sm text-[#262626] placeholder:text-[#8c8c8c] focus-visible:border-rolecue-brand focus-visible:ring-rolecue-brand/15 sm:h-12 ${errors.email ? "is-error" : ""}`}
              {...register("email")}
            />
            {errors.email && (
              <p
                id="forgot-email-error"
                role="alert"
                className="t-error-msg text-xs text-destructive"
              >
                {errors.email.message}
              </p>
            )}
          </div>
          <Button
            type="submit"
            disabled={isSubmitting}
            aria-busy={isSubmitting}
            className="h-11 w-full rounded-xl bg-[#262626] text-white transition-colors duration-150 hover:bg-[#1f1f1f] sm:h-12"
          >
            {isSubmitting && (
              <LoaderCircle className="animate-spin" aria-hidden />
            )}
            {isSubmitting ? "Sending…" : "Send reset link"}
          </Button>
        </form>
      )}
      <p className="mt-5 text-center text-sm text-[#595959]">
        <Link
          href={routes.login}
          className="rounded-sm font-semibold text-[#262626] underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-rolecue-brand"
        >
          Back to sign in
        </Link>
      </p>
    </div>
  );
}
