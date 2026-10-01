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
import { useAuthStore } from "@/stores/auth-store";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { Eye, EyeOff } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { login } from "../api/login";
import { loginSchema, type LoginFormValues } from "../api/login-schema";

const providers = [
  {
    name: "Google",
    href: routes.oauth.google,
    Icon: GoogleIcon,
    iconClassName: "",
  },
  {
    name: "GitHub",
    href: routes.oauth.github,
    Icon: GitHubIcon,
    iconClassName: "",
  },
  {
    name: "Facebook",
    href: routes.oauth.facebook,
    Icon: FacebookIcon,
    iconClassName: "text-[#1877f2]",
  },
] as const;

function getLoginErrorMessage(error: unknown) {
  if (isAxiosError(error)) {
    const data: unknown = error.response?.data;
    if (
      data &&
      typeof data === "object" &&
      "error" in data &&
      data.error &&
      typeof data.error === "object" &&
      "message" in data.error &&
      typeof data.error.message === "string"
    ) {
      return data.error.message;
    }
  }
  return "Unable to sign in. Please check your connection and try again.";
}

export function LoginForm({ defaultEmail = "" }: { defaultEmail?: string }) {
  const [showPassword, setShowPassword] = useState(false);
  const router = useRouter();
  const setAccessToken = useAuthStore((state) => state.setAccessToken);
  const loginMutation = useMutation({ mutationFn: login });
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues, unknown, LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: defaultEmail, password: "" },
    mode: "onTouched",
  });
  const isPending = isSubmitting || loginMutation.isPending;

  const submit = async (values: LoginFormValues) => {
    try {
      const result = await loginMutation.mutateAsync(values);
      setAccessToken(result.accessToken);
      router.replace(routes.dashboard);
    } catch (error) {
      toast.error(getLoginErrorMessage(error));
    }
  };

  return (
    <div className="w-full">
      <header className="mb-7 text-center motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-500">
        <RoleCueMark className="mx-auto size-32 object-contain" priority />
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-foreground sm:text-[28px]">
          Sign in to RoleCue
        </h1>
        <p className="mx-auto mt-1.5 max-w-[320px] text-sm leading-normal text-muted-foreground">
          Welcome back. Select an authentication method to continue.
        </p>
      </header>

      <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-3">
        {providers.map(({ name, href, Icon, iconClassName = "" }) => (
          <Button
            key={name}
            asChild
            variant="outline"
            className="group h-12 w-full rounded-xl border-border bg-card px-2 text-sm font-medium text-card-foreground shadow-xs transition-all duration-200 hover:border-(--rolecue-border-strong) hover:bg-muted hover:shadow-(--rolecue-shadow-low) active:scale-[0.99]"
          >
            <Link href={href} aria-label={`Continue with ${name}`}>
              <Icon
                className={`size-4 shrink-0 transition-transform duration-200 group-hover:scale-110 ${iconClassName}`}
              />
              <span>{name}</span>
            </Link>
          </Button>
        ))}
      </div>

      <div className="my-5 flex items-center gap-3 text-xs uppercase tracking-wide text-muted-foreground">
        <Separator className="flex-1 bg-border" />
        <span className="select-none text-[11px] font-medium tracking-[0.12em]">
          or continue with email
        </span>
        <Separator className="flex-1 bg-border" />
      </div>

      <form
        onSubmit={handleSubmit(submit)}
        noValidate
        aria-label="Sign in credentials form"
        className="space-y-4"
      >
        <div className="space-y-1.5">
          <label
            htmlFor="login-email"
            className="block text-xs font-semibold uppercase tracking-wider text-foreground"
          >
            Email address
          </label>
          <Input
            id="login-email"
            type="email"
            autoComplete="email"
            placeholder="name@example.com"
            disabled={isPending}
            aria-invalid={!!errors.email}
            aria-describedby={errors.email ? "login-email-error" : undefined}
            className="h-11 rounded-xl border-input bg-card px-3.5 text-sm text-card-foreground shadow-2xs transition-all placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/20 disabled:bg-muted sm:h-12"
            {...register("email")}
          />
          {errors.email && (
            <p
              id="login-email-error"
              role="alert"
              className="text-xs font-medium text-destructive"
            >
              {errors.email.message}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between gap-3">
            <label
              htmlFor="login-password"
              className="block text-xs font-semibold uppercase tracking-wider text-foreground"
            >
              Password
            </label>
            <Link
              href={routes.forgotPassword}
              className="rounded text-xs font-medium text-rolecue-brand transition hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rolecue-brand/40"
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
              disabled={isPending}
              aria-invalid={!!errors.password}
              aria-describedby={
                errors.password ? "login-password-error" : undefined
              }
              className="h-11 rounded-xl border-input bg-card px-3.5 pr-12 text-sm text-card-foreground shadow-2xs transition-all placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/20 disabled:bg-muted sm:h-12"
              {...register("password")}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => setShowPassword((visible) => !visible)}
              aria-label={showPassword ? "Hide password" : "Show password"}
              className="absolute top-1/2 right-2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              {showPassword ? <EyeOff /> : <Eye />}
            </Button>
          </div>
          {errors.password && (
            <p
              id="login-password-error"
              role="alert"
              className="text-xs font-medium text-destructive"
            >
              {errors.password.message}
            </p>
          )}
        </div>

        <Button
          type="submit"
          disabled={isPending}
          className="mt-1 h-11 w-full rounded-xl bg-primary px-4 text-sm font-medium text-primary-foreground shadow-(--rolecue-shadow-low) transition-all hover:bg-primary/90 hover:shadow-(--rolecue-shadow-float) active:scale-[0.99] sm:h-12"
        >
          {isPending ? "Signing in..." : "Sign in"}
        </Button>
      </form>

      <p className="mt-5 text-center text-xs text-muted-foreground sm:text-sm">
        No account?{" "}
        <Link
          href={routes.register}
          className="rounded-sm font-semibold text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rolecue-brand/40"
        >
          Register
        </Link>
      </p>
      <p className="mt-3.5 text-center text-[11px] leading-relaxed text-muted-foreground sm:text-xs">
        By continuing, you agree to the{" "}
        <Link
          href={routes.terms}
          className="font-medium text-foreground hover:text-rolecue-brand hover:underline"
        >
          Terms of Service
        </Link>{" "}
        and acknowledge that you have read the{" "}
        <Link
          href={routes.privacy}
          className="font-medium text-foreground hover:text-rolecue-brand hover:underline"
        >
          Privacy Policy
        </Link>
        .
      </p>
    </div>
  );
}
