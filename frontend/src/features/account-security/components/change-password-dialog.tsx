"use client";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { useRef, useState } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import {
  changePasswordSchema,
  type ChangePasswordValues,
} from "../schemas/change-password-schema";

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
          <DialogTitle>Change password</DialogTitle>
          <DialogDescription>
            Confirm your current password, then choose a new one.
          </DialogDescription>
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
      revokeOtherSessions: true,
    },
  });
  useShakeInvalidFields(formRef, submitCount);
  const [currentPassword, newPassword, confirmPassword] = useWatch({
    control,
    name: ["currentPassword", "newPassword", "confirmPassword"],
  });
  // Mirrors changePasswordSchema so the rules are visible before submitting.
  const requirements = [
    { label: "At least 8 characters", met: newPassword.length >= 8 },
    {
      label: "Different from your current password",
      met: newPassword.length > 0 && newPassword !== currentPassword,
    },
    {
      label: "Both new passwords match",
      met: confirmPassword.length > 0 && newPassword === confirmPassword,
    },
  ];

  const submit = async ({
    currentPassword,
    newPassword,
    revokeOtherSessions,
  }: ChangePasswordValues) => {
    setMessage("");
    onBusyChange(true);
    try {
      const result = await authClient.changePassword({
        currentPassword,
        newPassword,
        revokeOtherSessions,
      });
      if (result.error) {
        // Map known codes only; the server message is never shown.
        switch (result.error.code) {
          case "INVALID_PASSWORD":
            setError("currentPassword", {
              message: "Current password is incorrect.",
            });
            return;
          case "PASSWORD_TOO_SHORT":
            setError("newPassword", { message: "Use at least 8 characters" });
            return;
          case "PASSWORD_TOO_LONG":
            setError("newPassword", { message: "Password is too long" });
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
          setMessage(
            "Your session expired. Sign in again to change your password.",
          );
          // SessionGate rechecks the session and sends the user to sign in.
          void cache.invalidateQueries({ queryKey: ["auth", "me"] });
        } else if (result.error.status === 429)
          setMessage("Too many attempts. Wait a few seconds and try again.");
        else
          setMessage("Your password could not be changed. Please try again.");
        return;
      }
      gooeyToast.success("Password updated", {
        description: revokeOtherSessions
          ? "Your other devices have been signed out."
          : "Use your new password the next time you sign in.",
      });
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      void cache.invalidateQueries({ queryKey: credentialsKeys.all });
      onDone();
    } catch {
      setMessage("Your password could not be changed. Please try again.");
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
        label="Current password"
        autoComplete="current-password"
        error={errors.currentPassword}
        registration={register("currentPassword")}
      />
      <PasswordField
        id="new-password"
        label="New password"
        autoComplete="new-password"
        error={errors.newPassword}
        registration={register("newPassword")}
      />
      <PasswordField
        id="confirm-new-password"
        label="Confirm new password"
        autoComplete="new-password"
        error={errors.confirmPassword}
        registration={register("confirmPassword")}
      />
      <ul aria-label="Password requirements" className="space-y-1.5">
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
              {requirement.met ? "(met)" : "(not met)"}
            </span>
          </li>
        ))}
      </ul>
      <Controller
        control={control}
        name="revokeOtherSessions"
        render={({ field }) => (
          <div className="flex items-start gap-3 rounded-xl bg-muted px-4 py-3">
            <Checkbox
              id="revoke-other-sessions"
              checked={field.value}
              onCheckedChange={(checked) => field.onChange(checked === true)}
              className="mt-0.5"
            />
            <label
              htmlFor="revoke-other-sessions"
              className="text-sm leading-snug"
            >
              Sign out of other devices
              <span className="block text-xs text-muted-foreground">
                This device stays signed in.
              </span>
            </label>
          </div>
        )}
      />
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
          Cancel
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
          {isSubmitting ? "Updating…" : "Update password"}
        </Button>
      </DialogFooter>
    </form>
  );
}
