"use client";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { credentialsKeys } from "@/features/auth/api/credentials";
import { PasswordField } from "@/features/auth/components/password-field";
import { useShakeInvalidFields } from "@/features/auth/hooks/use-shake-invalid-fields";
import { authClient } from "@/lib/auth/client";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { CircleCheck, CircleDashed, LoaderCircle } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import {
  createChangePasswordSchema,
  type ChangePasswordValues,
} from "../schemas/change-password-schema";
import { useTranslations } from "next-intl";

type ChangePasswordDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Called when Better Auth reports that the account has no password after all.
  onNoPassword: () => void;
};

// The form lives inside the dialog content, so closing the dialog unmounts it and drops every
// typed password. While a request is in flight, Esc, outside clicks and the close button are
// ignored so the result is never lost.
export function ChangePasswordDialog({
  open,
  onOpenChange,
  onNoPassword,
}: ChangePasswordDialogProps) {
  const t = useTranslations("auth.security.changePassword");
  const [busy, setBusy] = useState(false);
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && busy) return;
        onOpenChange(next);
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        <ChangePasswordForm
          onBusyChange={setBusy}
          onDone={() => {
            setBusy(false);
            onOpenChange(false);
          }}
          onNoPassword={onNoPassword}
        />
      </DialogContent>
    </Dialog>
  );
}

function ChangePasswordForm({
  onBusyChange,
  onDone,
  onNoPassword,
}: {
  onBusyChange: (busy: boolean) => void;
  onDone: () => void;
  onNoPassword: () => void;
}) {
  const t = useTranslations("auth.security.changePassword");
  const v = useTranslations("validation");
  const changePasswordSchema = useMemo(
    () => createChangePasswordSchema(v),
    [v],
  );
  const te = useTranslations("errors");
  const cache = useQueryClient();
  const formRef = useRef<HTMLFormElement>(null);
  const [message, setMessage] = useState("");
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting, submitCount },
  } = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: {
      currentPassword: "",
      newPassword: "",
      confirmPassword: "",
    },
  });
  useShakeInvalidFields(formRef, submitCount);
  const [currentPassword, newPassword, confirmPassword] = useWatch({
    control,
    name: ["currentPassword", "newPassword", "confirmPassword"],
  });
  // Mirrors changePasswordSchema so the rules are visible before submitting.
  const requirements = [
    { label: t("minLength"), met: newPassword.length >= 8 },
    {
      label: t("different"),
      met: newPassword.length > 0 && newPassword !== currentPassword,
    },
    {
      label: t("match"),
      met: confirmPassword.length > 0 && newPassword === confirmPassword,
    },
  ];

  const submit = async ({
    currentPassword,
    newPassword,
  }: ChangePasswordValues) => {
    setMessage("");
    onBusyChange(true);
    try {
      // The server always signs out other devices on password change (KAN-256).
      const result = await authClient.changePassword({
        currentPassword,
        newPassword,
        revokeOtherSessions: true,
      });
      if (result.error) {
        // Map known codes only; the server message is never shown.
        switch (result.error.code) {
          case "INVALID_PASSWORD":
            setError("currentPassword", {
              message: te("auth.changePasswordWrongCurrent"),
            });
            return;
          case "PASSWORD_TOO_SHORT":
            setError("newPassword", { message: v("passwordMin", { min: 8 }) });
            return;
          case "PASSWORD_TOO_LONG":
            setError("newPassword", { message: v("passwordTooLong") });
            return;
          case "CREDENTIAL_ACCOUNT_NOT_FOUND":
            onNoPassword();
            onDone();
            return;
        }
        if (
          result.error.status === 401 ||
          result.error.code === "UNAUTHORIZED"
        ) {
          setMessage(te("auth.changePasswordSessionExpired"));
          // SessionGate rechecks the session and sends the user to sign in.
          void cache.invalidateQueries({ queryKey: ["auth", "me"] });
        } else if (result.error.status === 429)
          setMessage(te("auth.changePasswordRateLimited"));
        else setMessage(te("auth.changePasswordFailed"));
        return;
      }
      gooeyToast.success(t("updatedTitle"), {
        description: t("updatedRevoked"),
      });
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      void cache.invalidateQueries({ queryKey: credentialsKeys.all });
      onDone();
    } catch {
      setMessage(te("auth.changePasswordFailed"));
    } finally {
      onBusyChange(false);
    }
  };

  return (
    <form
      ref={formRef}
      onSubmit={handleSubmit(submit)}
      noValidate
      className="space-y-4"
    >
      <PasswordField
        id="current-password"
        label={t("current")}
        autoComplete="current-password"
        error={errors.currentPassword}
        registration={register("currentPassword")}
      />
      <PasswordField
        id="new-password"
        label={t("new")}
        autoComplete="new-password"
        error={errors.newPassword}
        registration={register("newPassword")}
      />
      <PasswordField
        id="confirm-new-password"
        label={t("confirm")}
        autoComplete="new-password"
        error={errors.confirmPassword}
        registration={register("confirmPassword")}
      />
      <ul aria-label={t("requirements")} className="space-y-1.5">
        {requirements.map((requirement) => (
          <li
            key={requirement.label}
            className={`flex items-center gap-2 text-xs transition-colors duration-(--duration-quick) ${requirement.met ? "text-foreground" : "text-muted-foreground"}`}
          >
            {requirement.met ? (
              <CircleCheck
                className="size-3.5 text-rolecue-brand"
                aria-hidden
              />
            ) : (
              <CircleDashed className="size-3.5" aria-hidden />
            )}
            {requirement.label}
            <span className="sr-only">
              {requirement.met ? t("met") : t("notMet")}
            </span>
          </li>
        ))}
      </ul>
      <div className="rounded-xl bg-muted px-4 py-3 text-sm">
        <p>{t("revoke")}</p>
        <p className="text-xs text-muted-foreground">{t("revokeHint")}</p>
      </div>
      {message && (
        <p role="alert" className="text-sm text-destructive">
          {message}
        </p>
      )}
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          className="min-h-11"
          onClick={onDone}
          disabled={isSubmitting}
        >
          {t("cancel")}
        </Button>
        <Button
          type="submit"
          className="min-h-11"
          disabled={isSubmitting}
          aria-busy={isSubmitting}
        >
          {isSubmitting && (
            <LoaderCircle className="animate-spin" aria-hidden />
          )}
          {isSubmitting ? t("updating") : t("update")}
        </Button>
      </DialogFooter>
    </form>
  );
}
