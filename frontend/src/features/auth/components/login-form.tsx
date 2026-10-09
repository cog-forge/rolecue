"use client";

import { RoleCueMark } from "@/components/brand/rolecue-mark";
import {
  FacebookIcon,
  GitHubIcon,
  GoogleIcon,
} from "@/components/icons/social-icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { routes } from "@/config/routes";
import {
  getAuthErrorMessage,
  isEmailVerificationError,
} from "@/features/auth/utils/get-auth-error-message";
import { authClient } from "@/lib/auth/client";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { PasswordVisibilityIcon } from "@/components/icons/password-visibility-icon";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { gooeyToast as toast } from "goey-toast";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import { loginSchema, type LoginFormValues } from "../schemas/login-schema";

const providerOptions = [
  { name: "Google", provider: "google", Icon: GoogleIcon, className: "" },
  { name: "GitHub", provider: "github", Icon: GitHubIcon, className: "" },
  {
    name: "Facebook",
    provider: "facebook",
    Icon: FacebookIcon,
    className: "text-[#1877f2]",
  },
] as const;
export type SocialProvider = (typeof providerOptions)[number]["provider"];

export function LoginForm({
  defaultEmail = "",
  oauthFailed = false,
  oauthError,
  enabledProviders = [],
}: {
  defaultEmail?: string;
  oauthFailed?: boolean;
  oauthError?: string;
  enabledProviders?: readonly SocialProvider[];
}) {
  const [showPassword, setShowPassword] = useState(false);
  const [socialPendingProvider, setSocialPendingProvider] =
    useState<SocialProvider | null>(null);
  const [verificationEmail, setVerificationEmail] = useState("");
  const [verificationMessage, setVerificationMessage] = useState("");
  const router = useRouter();
  const formRef = useRef<HTMLFormElement>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting, submitCount },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: defaultEmail, password: "" },
    mode: "onTouched",
  });
  useShakeInvalidFields(formRef, submitCount);
  const pending = isSubmitting || socialPendingProvider !== null;

  const submit = async (values: LoginFormValues) => {
    try {
      const { error } = await authClient.signIn.email({
        ...values,
        callbackURL: `${routes.dashboard}?auth=signed-in`,
      });
      if (error) throw error;
      router.replace(`${routes.dashboard}?auth=signed-in`);
    } catch (error) {
      toast.error("Sign-in failed", {
        description: getAuthErrorMessage(
          error,
          "Unable to sign in. Please check your connection and try again.",
        ),
      });
      if (isEmailVerificationError(error)) setVerificationEmail(values.email);
    }
  };

  const resendVerification = async () => {
    try {
      const { error } = await authClient.sendVerificationEmail({
        email: verificationEmail,
        callbackURL: routes.afterEmailVerification,
      });
      if (error) throw error;
      setVerificationMessage("Verification email sent.");
      toast.success("Verification email sent", {
        description: "Check your inbox and spam folder for the link.",
      });
    } catch {
      setVerificationMessage(
        "Unable to resend the verification email right now.",
      );
      toast.error("Email could not be sent", {
        description: "Please try again in a moment.",
      });
    }
  };

  return (
    <div className="w-full">
      <header className="mb-7 text-center">
        <RoleCueMark className="mx-auto size-32 object-contain" priority />
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-[#121814] sm:text-[28px]">
          Sign in to RoleCue
        </h1>
        <p className="mx-auto mt-1.5 max-w-[320px] text-sm leading-normal text-[#5c5c5c]">
          Welcome back. Select an authentication method to continue.
        </p>
      </header>

      {oauthFailed && (
        <p
          role="alert"
          className="mb-4 rounded-xl border border-[#e5e5e5] bg-white p-3 text-sm text-[#5c5c5c]"
        >
          {oauthError === "account_not_linked"
            ? "This social account is not linked to RoleCue. Sign in with your original method first."
            : "Social sign-in could not be completed. Please try again."}
        </p>
      )}
      {verificationEmail && (
        <div className="mb-4 rounded-xl border border-[#e5e5e5] bg-white p-3 text-sm text-[#5c5c5c]">
          <p>
            Please verify your email before signing in. You can request a new
            verification link below.
          </p>
          <Button
            type="button"
            variant="link"
            onClick={resendVerification}
            className="mt-1 h-auto p-0 text-sm font-semibold text-[#121814]"
          >
            Resend verification email
          </Button>
          {verificationMessage && (
            <p role="status" className="mt-1">
              {verificationMessage}
            </p>
          )}
        </div>
      )}

      <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-3">
        {providerOptions
          .filter(({ provider }) => enabledProviders.includes(provider))
          .map(({ name, provider, Icon, className = "" }) => (
            <Button
              key={name}
              type="button"
              variant="outline"
              disabled={pending}
              aria-label={`Continue with ${name}`}
              onClick={async () => {
                setSocialPendingProvider(provider);
                try {
                  const { error } = await authClient.signIn.social({
                    provider,
                    callbackURL: `${routes.dashboard}?auth=signed-in`,
                    errorCallbackURL: `${routes.login}?oauth=failed`,
                  });
                  if (error) throw error;
                } catch (error) {
                  toast.error("Social sign-in failed", {
                    description: getAuthErrorMessage(
                      error,
                      `Unable to continue with ${name}. Please try again.`,
                    ),
                  });
                  setSocialPendingProvider(null);
                }
              }}
              aria-busy={socialPendingProvider === provider}
              className="group h-12 w-full rounded-xl border-[#dbdbdb] bg-white px-2 text-sm font-medium text-[#262626] shadow-[0_1px_2px_rgba(0,0,0,0.03)] transition-all duration-200 hover:border-[#b5b5b5] hover:bg-[#fafafa] hover:shadow-[0_2px_8px_rgba(0,0,0,0.06)] active:scale-[0.99]"
            >
              {socialPendingProvider === provider ? (
                <LoaderCircle className="size-4 animate-spin" aria-hidden />
              ) : (
                <Icon
                  className={`size-4 shrink-0 transition-transform duration-200 group-hover:scale-110 ${className}`}
                />
              )}
              <span>
                {socialPendingProvider === provider ? "Connecting…" : name}
              </span>
            </Button>
          ))}
      </div>

      <div className="my-5 flex items-center gap-3 text-xs uppercase tracking-wide text-[#858585]">
        <Separator className="flex-1 bg-[#e5e5e5]" />
        <span className="select-none text-[11px] font-medium tracking-[0.12em]">
          or continue with email
        </span>
        <Separator className="flex-1 bg-[#e5e5e5]" />
      </div>

      <form
        ref={formRef}
        onSubmit={handleSubmit(submit)}
        noValidate
        aria-label="Sign in credentials form"
        className="space-y-4"
      >
        <div
          className={`t-input-wrap space-y-1.5 ${errors.email ? "is-error" : ""}`}
        >
          <label
            htmlFor="login-email"
            className="block text-xs font-semibold uppercase tracking-wider text-[#494949]"
          >
            Email address
          </label>
          <Input
            id="login-email"
            type="email"
            autoComplete="email"
            placeholder="name@example.com"
            disabled={pending}
            aria-invalid={!!errors.email}
            aria-describedby={errors.email ? "login-email-error" : undefined}
            data-auth-input
            className={`t-input h-11 rounded-xl border-[#dbdbdb] bg-white px-3.5 text-sm text-[#262626] shadow-2xs placeholder:text-[#8c8c8c] focus-visible:border-rolecue-brand focus-visible:ring-rolecue-brand/15 sm:h-12 ${errors.email ? "is-error" : ""}`}
            {...register("email")}
          />
          {errors.email && (
            <p
              id="login-email-error"
              role="alert"
              className="t-error-msg text-xs font-medium text-destructive"
            >
              {errors.email.message}
            </p>
          )}
        </div>
        <div
          className={`t-input-wrap space-y-1.5 ${errors.password ? "is-error" : ""}`}
        >
          <div className="flex items-center justify-between gap-3">
            <label
              htmlFor="login-password"
              className="block text-xs font-semibold uppercase tracking-wider text-[#494949]"
            >
              Password
            </label>
            <Link
              href={routes.forgotPassword}
              className="rounded text-xs font-medium text-rolecue-brand hover:underline"
            >
              Forgot password?
            </Link>
          </div>
          <div className="relative">
            <Input
              id="login-password"
              type={showPassword ? "text" : "password"}
              autoComplete="current-password"
              placeholder="••••••••"
              disabled={pending}
              aria-invalid={!!errors.password}
              aria-describedby={
                errors.password ? "login-password-error" : undefined
              }
              data-auth-input
              className={`t-input h-11 rounded-xl border-[#dbdbdb] bg-white px-3.5 pr-12 text-sm text-[#262626] shadow-2xs placeholder:text-[#8c8c8c] focus-visible:border-rolecue-brand focus-visible:ring-rolecue-brand/15 sm:h-12 ${errors.password ? "is-error" : ""}`}
              {...register("password")}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => setShowPassword((value) => !value)}
              aria-label={showPassword ? "Hide password" : "Show password"}
              aria-controls="login-password"
              aria-pressed={showPassword}
              disabled={pending}
              className="absolute top-1/2 right-1 size-11 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-[#595959] hover:text-[#262626]"
            >
              <PasswordVisibilityIcon visible={showPassword} />
            </Button>
          </div>
          {errors.password && (
            <p
              id="login-password-error"
              role="alert"
              className="t-error-msg text-xs font-medium text-destructive"
            >
              {errors.password.message}
            </p>
          )}
        </div>
        <Button
          type="submit"
          disabled={pending}
          aria-busy={isSubmitting}
          className="mt-1 h-11 w-full rounded-xl bg-[#262626] px-4 text-sm font-medium text-white transition-colors duration-150 hover:bg-[#1f1f1f] sm:h-12"
        >
          {isSubmitting && (
            <LoaderCircle className="animate-spin" aria-hidden />
          )}
          {isSubmitting ? "Signing in…" : "Sign in"}
        </Button>
      </form>

      <p className="mt-5 text-center text-xs text-[#5c5c5c] sm:text-sm">
        No account?{" "}
        <Link
          href={routes.register}
          className="rounded-sm font-semibold text-[#121814] hover:underline"
        >
          Register
        </Link>
      </p>
      <p className="mt-3.5 text-center text-[11px] leading-relaxed text-[#737373] sm:text-xs">
        By continuing, you agree to the{" "}
        <Link
          href={routes.terms}
          className="font-medium text-[#494949] hover:text-[#121814] hover:underline"
        >
          Terms of Service
        </Link>{" "}
        and acknowledge that you have read the{" "}
        <Link
          href={routes.privacy}
          className="font-medium text-[#494949] hover:text-[#121814] hover:underline"
        >
          Privacy Policy
        </Link>
        .
      </p>
    </div>
  );
}
