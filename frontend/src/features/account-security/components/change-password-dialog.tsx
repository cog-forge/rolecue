"use client";

import { PasswordVisibilityIcon } from "@/components/icons/password-visibility-icon";
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
import { Input } from "@/components/ui/input";
import { useShakeInvalidFields } from "@/features/auth/hooks/use-shake-invalid-fields";
import { authClient } from "@/lib/auth/client";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { LoaderCircle } from "lucide-react";
import { useRef, useState } from "react";
import {
  Controller,
  useForm,
  type FieldError,
  type UseFormRegisterReturn,
} from "react-hook-form";
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
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Change password</DialogTitle>
          <DialogDescription>
            Enter your current password, then choose a new one with at least 8
            characters.
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
        help="Use at least 8 characters."
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
      <Controller
        control={control}
        name="revokeOtherSessions"
        render={({ field }) => (
          <div className="flex items-start gap-3">
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
          onClick={onDone}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
          {isSubmitting && (
            <LoaderCircle className="animate-spin" aria-hidden />
          )}
          {isSubmitting ? "Updating…" : "Update password"}
        </Button>
      </DialogFooter>
    </form>
  );
}

function PasswordField({
  id,
  label,
  autoComplete,
  help,
  error,
  registration,
}: {
  id: string;
  label: string;
  autoComplete: "current-password" | "new-password";
  help?: string;
  error?: FieldError;
  registration: UseFormRegisterReturn;
}) {
  const [visible, setVisible] = useState(false);
  const describedBy = error ? `${id}-error` : help ? `${id}-help` : undefined;
  return (
    <div className={`t-input-wrap space-y-1.5 ${error ? "is-error" : ""}`}>
      <label htmlFor={id} className="block text-sm font-medium">
        {label}
      </label>
      <div className="relative">
        <Input
          id={id}
          type={visible ? "text" : "password"}
          autoComplete={autoComplete}
          aria-invalid={!!error}
          aria-describedby={describedBy}
          className={`t-input h-10 pr-12 ${error ? "is-error" : ""}`}
          {...registration}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={() => setVisible((current) => !current)}
          aria-label={`${visible ? "Hide" : "Show"} ${label.toLowerCase()}`}
          aria-controls={id}
          aria-pressed={visible}
          className="absolute top-1/2 right-1 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-muted-foreground hover:text-foreground"
        >
          <PasswordVisibilityIcon visible={visible} />
        </Button>
      </div>
      {error ? (
        <p
          id={`${id}-error`}
          role="alert"
          className="t-error-msg text-xs text-destructive"
        >
          {error.message}
        </p>
      ) : (
        help && (
          <p id={`${id}-help`} className="text-xs text-muted-foreground">
            {help}
          </p>
        )
      )}
    </div>
  );
}
