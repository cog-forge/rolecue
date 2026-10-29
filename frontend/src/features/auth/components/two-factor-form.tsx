"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { ShieldCheck, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { authClient } from "@/lib/auth/client";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import {
  reauthenticationHref,
  signedInDestination,
} from "../utils/auth-navigation";
import {
  securityData,
  securityErrorCode,
  securityErrorMessage,
} from "../utils/security-errors";
import {
  createBackupVerificationSchema,
  createTotpVerificationSchema,
  type VerificationValues,
} from "../schemas/credentials-schema";
import { useTranslations } from "next-intl";

export function TwoFactorForm({ returnTo }: { returnTo?: string }) {
  const v = useTranslations("validation");
  const totpVerificationSchema = useMemo(
    () => createTotpVerificationSchema(v),
    [v],
  );
  const backupVerificationSchema = useMemo(
    () => createBackupVerificationSchema(v),
    [v],
  );
  const te = useTranslations("errors");
  const [method, setMethod] = useState<"totp" | "backup">("totp");
  const router = useRouter();
  const cache = useQueryClient();
  const formRef = useRef<HTMLFormElement>(null);
  const form = useForm<VerificationValues>({
    resolver: zodResolver(
      method === "totp" ? totpVerificationSchema : backupVerificationSchema,
    ),
    defaultValues: { code: "", trustDevice: false },
  });
  useShakeInvalidFields(formRef, form.formState.submitCount);
  const resetForm = form.reset;
  useEffect(() => () => resetForm(), [resetForm]);
  const verification = useMutation({
    // Sensitive form values are read inside the request, never retained as
    // mutation variables or returned as mutation data.
    mutationFn: async () => {
      const values = form.getValues();
      securityData(
        method === "totp"
          ? await authClient.twoFactor.verifyTotp(values)
          : await authClient.twoFactor.verifyBackupCode({
              ...values,
              disableSession: false,
            }),
      );
      const session = securityData(
        await authClient.getSession({ query: { disableCookieCache: true } }),
      );
      form.reset();
      cache.clear();
      router.replace(signedInDestination(session.user.role, returnTo));
    },
    retry: false,
    gcTime: 0,
  });
  const ended = [
    "INVALID_TWO_FACTOR_COOKIE",
    "TOO_MANY_ATTEMPTS_REQUEST_NEW_CODE",
    "BANNED_USER",
    "UNAUTHORIZED",
  ].includes(securityErrorCode(verification.error) ?? "");

  return (
    <div className="space-y-6">
      <header className="space-y-3">
        <ShieldCheck className="size-8 text-rolecue-brand" aria-hidden />
        <h1 className="text-3xl font-semibold tracking-tight">
          Verify your sign-in
        </h1>
        <p className="text-sm leading-6 text-muted-foreground">
          {method === "totp"
            ? "Enter the code from your authenticator app to finish signing in."
            : "Enter one of your saved backup codes. Each code can be used once."}
        </p>
      </header>
      <form
        ref={formRef}
        aria-label="Verify two-factor sign-in"
        className="space-y-5"
        noValidate
        onSubmit={form.handleSubmit(async () => {
          if (verification.isPending || ended) return;
          try {
            await verification.mutateAsync();
          } catch {
            /* The alert below owns request errors. */
          }
        })}
      >
        <div
          className={`t-input-wrap space-y-1.5 ${form.formState.errors.code ? "is-error" : ""}`}
        >
          <label htmlFor="challenge-code" className="block text-sm font-medium">
            {method === "totp" ? "Authenticator code" : "Backup code"}
          </label>
          <Input
            id="challenge-code"
            className={`t-input h-14 text-center tabular-nums ${method === "totp" ? "text-2xl tracking-[0.4em] md:text-2xl" : "text-lg tracking-wider md:text-lg"} ${form.formState.errors.code ? "is-error" : ""}`}
            type="text"
            inputMode={method === "totp" ? "numeric" : "text"}
            autoComplete="one-time-code"
            autoCapitalize="none"
            spellCheck={false}
            maxLength={method === "totp" ? 7 : 128}
            disabled={verification.isPending || ended}
            aria-invalid={Boolean(form.formState.errors.code)}
            aria-describedby={
              form.formState.errors.code ? "challenge-code-error" : undefined
            }
            {...form.register("code", {
              // Pasted codes often arrive grouped as "012 345".
              setValueAs: (value: string) =>
                method === "totp" ? value.replace(/\s/g, "") : value,
            })}
          />
          {form.formState.errors.code && (
            <p
              id="challenge-code-error"
              role="alert"
              className="t-error-msg text-xs text-destructive"
            >
              {form.formState.errors.code.message}
            </p>
          )}
        </div>
        <Controller
          control={form.control}
          name="trustDevice"
          render={({ field }) => (
            <div className="flex min-h-11 items-center gap-3">
              <Checkbox
                id="trust-device"
                checked={field.value}
                onCheckedChange={(checked) => field.onChange(checked === true)}
                disabled={verification.isPending || ended}
              />
              <label htmlFor="trust-device" className="text-sm">
                Remember this browser for 30 days
              </label>
            </div>
          )}
        />
        {verification.error && (
          <p role="alert" className="text-sm text-destructive">
            {securityErrorMessage(verification.error, te)}
          </p>
        )}
        <Button
          type="submit"
          className="h-11 w-full"
          disabled={verification.isPending || ended}
          aria-busy={verification.isPending}
        >
          {verification.isPending && (
            <LoaderCircle className="animate-spin" aria-hidden />
          )}
          {verification.isPending ? "Verifying…" : "Verify and sign in"}
        </Button>
      </form>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4">
        <Button
          variant="link"
          className="min-h-11 px-0"
          disabled={verification.isPending || ended}
          onClick={() => {
            setMethod(method === "totp" ? "backup" : "totp");
            form.reset({
              code: "",
              trustDevice: form.getValues("trustDevice"),
            });
            verification.reset();
          }}
        >
          {method === "totp" ? "Use a backup code" : "Use authenticator app"}
        </Button>
        <Link
          href={reauthenticationHref(returnTo)}
          className="inline-flex min-h-11 items-center text-sm underline underline-offset-4"
        >
          Back to sign in
        </Link>
      </div>
    </div>
  );
}
