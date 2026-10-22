"use client";

import { useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { LoaderCircle } from "lucide-react";
import QRCode from "react-qr-code";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { authClient } from "@/lib/auth/client";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import {
  currentPasswordSchema,
  totpSetupSchema,
  totpVerificationSchema,
  type CurrentPasswordValues,
  type TotpSetup,
  type VerificationValues,
} from "../schemas/credentials-schema";
import { securityData } from "../utils/security-errors";
import { PasswordField } from "./password-field";
import { SecurityRequestError } from "./security-request-error";
import { BackupCodes } from "./backup-codes";
import { CopyButton } from "./copy-button";

const steps = [
  {
    title: "Confirm it's you",
    description:
      "Enter your current password to start setting up an authenticator app.",
  },
  {
    title: "Connect your authenticator",
    description:
      "Scan the code with an app such as Google Authenticator or 1Password, then enter the code it shows.",
  },
  {
    title: "Save your backup codes",
    description:
      "Each code signs you in once if you lose your phone. They are shown only now.",
  },
];

// Pasted codes often arrive grouped as "012 345".
const withoutSpaces = (value: string) => value.replace(/\s/g, "");

function SetupProgress({ step }: { step: number }) {
  return (
    <div className="flex gap-1.5" aria-hidden>
      {steps.map((item, index) => (
        <span
          key={item.title}
          className={`h-1 flex-1 rounded-full transition-colors duration-(--duration-fast) ${index < step ? "bg-foreground" : "bg-muted"}`}
        />
      ))}
    </div>
  );
}

export function TotpEnrollmentDialog({
  onChanged,
  onAccessError,
  enabled = false,
}: {
  onChanged: () => Promise<void>;
  onAccessError: (error: unknown) => void;
  enabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [setup, setSetup] = useState<TotpSetup | null>(null);
  const passwordFormRef = useRef<HTMLFormElement>(null);
  const codeFormRef = useRef<HTMLFormElement>(null);
  const [verified, setVerified] = useState(false);
  const password = useForm<CurrentPasswordValues>({
    resolver: zodResolver(currentPasswordSchema),
    defaultValues: { password: "" },
  });
  const code = useForm<VerificationValues>({
    resolver: zodResolver(totpVerificationSchema),
    defaultValues: { code: "", trustDevice: false },
  });
  useShakeInvalidFields(passwordFormRef, password.formState.submitCount);
  useShakeInvalidFields(codeFormRef, code.formState.submitCount);
  const resetPassword = password.reset;
  const resetCode = code.reset;
  useEffect(
    () => () => {
      resetPassword();
      resetCode();
    },
    [resetPassword, resetCode],
  );
  const enrollment = useMutation({
    gcTime: 0,
    retry: false,
    mutationFn: async () => {
      const result = totpSetupSchema.parse(
        securityData(
          await authClient.twoFactor.enable({
            password: password.getValues("password"),
            method: "totp",
          }),
        ),
      );
      if (!new URL(result.totpURI).searchParams.get("secret"))
        throw new Error("Missing authenticator key");
      password.reset();
      setSetup(result);
    },
    onSuccess: (): void => {
      enrollment.reset();
    },
    onError: onAccessError,
  });
  const verification = useMutation({
    gcTime: 0,
    retry: false,
    mutationFn: async () => {
      securityData(
        await authClient.twoFactor.verifyTotp({
          code: code.getValues("code"),
          trustDevice: false,
        }),
      );
      code.reset();
      setVerified(true);
      gooeyToast.success("Authenticator enabled", {
        description: "Password sign-in now asks for a code.",
      });
    },
    onSuccess: (): void => {
      verification.reset();
    },
    onError: onAccessError,
  });
  const pending = enrollment.isPending || verification.isPending;
  function close() {
    const refreshSession = setup !== null;
    password.reset();
    code.reset();
    setSetup(null);
    setVerified(false);
    enrollment.reset();
    verification.reset();
    setOpen(false);
    if (refreshSession) void onChanged();
  }
  const error = setup ? verification.error : enrollment.error;
  const step = verified ? 3 : setup ? 2 : 1;
  const secret = setup
    ? (new URL(setup.totpURI).searchParams.get("secret") ?? "")
    : "";
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (pending) return;
        if (next) setOpen(true);
        else close();
      }}
    >
      <DialogTrigger asChild>
        <Button className={enabled ? "hidden" : "min-h-11"} disabled={enabled}>
          Enable authenticator
        </Button>
      </DialogTrigger>
      <DialogContent
        showCloseButton={false}
        className="max-h-[90dvh] overflow-y-auto sm:max-w-lg"
        onInteractOutside={(event) => event.preventDefault()}
      >
        <DialogHeader className="gap-3">
          <SetupProgress step={step} />
          <DialogTitle>{steps[step - 1].title}</DialogTitle>
          <DialogDescription>{steps[step - 1].description}</DialogDescription>
        </DialogHeader>
        {verified && setup ? (
          <div className="space-y-4">
            <div className="flex items-center gap-3 rounded-xl bg-(--rolecue-status-on-bg) px-4 py-3 text-(--rolecue-status-on-ink)">
              <span className="t-success-check" data-state="in" aria-hidden>
                <svg
                  viewBox="0 0 24 24"
                  className="size-5"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2.5}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M5 12l4 4L19 6" />
                </svg>
              </span>
              <p className="text-sm font-medium">Authenticator enabled</p>
            </div>
            <BackupCodes codes={setup.backupCodes} onDone={close} />
            <Button variant="ghost" className="min-h-11 w-full" onClick={close}>
              Close without saving
            </Button>
          </div>
        ) : setup ? (
          <div className="space-y-6">
            <div className="grid gap-5 sm:grid-cols-[auto_minmax(0,1fr)] sm:items-center">
              <div className="mx-auto rounded-xl border bg-white p-3">
                <QRCode
                  value={setup.totpURI}
                  size={148}
                  title="Scan this QR code with your authenticator"
                />
              </div>
              <div className="min-w-0 space-y-3">
                <p className="text-sm leading-6 text-muted-foreground">
                  Can&apos;t scan it? Add the account in your app with this
                  setup key instead.
                </p>
                <div className="flex items-center gap-2 rounded-xl bg-muted py-1.5 pr-1.5 pl-3">
                  <code
                    aria-label="Setup key"
                    className="min-w-0 flex-1 font-mono text-sm tracking-wider wrap-anywhere select-all"
                  >
                    {secret}
                  </code>
                  <CopyButton value={secret} label="Copy setup key" />
                </div>
              </div>
            </div>
            <form
              ref={codeFormRef}
              className="space-y-4"
              noValidate
              onSubmit={code.handleSubmit(() => verification.mutate())}
            >
              <div
                className={`t-input-wrap space-y-1.5 ${code.formState.errors.code ? "is-error" : ""}`}
              >
                <label
                  htmlFor="enrollment-code"
                  className="block text-sm font-medium"
                >
                  Authenticator code
                </label>
                <Input
                  id="enrollment-code"
                  className={`t-input h-12 text-center text-xl tracking-[0.35em] tabular-nums md:text-xl ${code.formState.errors.code ? "is-error" : ""}`}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={7}
                  disabled={pending}
                  aria-invalid={!!code.formState.errors.code}
                  aria-describedby={
                    code.formState.errors.code
                      ? "enrollment-code-error"
                      : "enrollment-code-help"
                  }
                  {...code.register("code", { setValueAs: withoutSpaces })}
                />
                {code.formState.errors.code ? (
                  <p
                    id="enrollment-code-error"
                    role="alert"
                    className="t-error-msg text-xs text-destructive"
                  >
                    {code.formState.errors.code.message}
                  </p>
                ) : (
                  <p
                    id="enrollment-code-help"
                    className="text-xs text-muted-foreground"
                  >
                    The 6-digit code your app shows for RoleCue. It changes
                    every 30 seconds.
                  </p>
                )}
              </div>
              {Boolean(error) && <SecurityRequestError error={error} />}
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  className="min-h-11"
                  disabled={pending}
                  onClick={close}
                >
                  Cancel setup
                </Button>
                <Button
                  type="submit"
                  className="min-h-11"
                  disabled={pending}
                  aria-busy={pending}
                >
                  {pending && (
                    <LoaderCircle className="animate-spin" aria-hidden />
                  )}
                  {pending ? "Verifying…" : "Verify authenticator"}
                </Button>
              </DialogFooter>
            </form>
          </div>
        ) : (
          <form
            ref={passwordFormRef}
            className="space-y-4"
            noValidate
            onSubmit={password.handleSubmit(() => enrollment.mutate())}
          >
            <PasswordField
              id="enrollment-password"
              label="Current password"
              autoComplete="current-password"
              disabled={pending}
              error={password.formState.errors.password}
              registration={password.register("password")}
            />
            {Boolean(error) && <SecurityRequestError error={error} />}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                className="min-h-11"
                disabled={pending}
                onClick={close}
              >
                Cancel setup
              </Button>
              <Button
                type="submit"
                className="min-h-11"
                disabled={pending}
                aria-busy={pending}
              >
                {pending && (
                  <LoaderCircle className="animate-spin" aria-hidden />
                )}
                {pending ? "Preparing…" : "Continue"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
