"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { ListChecks, LoaderCircle, ShieldOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { authClient } from "@/lib/auth/client";
import { useShakeInvalidFields } from "../hooks/use-shake-invalid-fields";
import {
  createCurrentPasswordSchema,
  type CurrentPasswordValues,
} from "../schemas/credentials-schema";
import { securityData } from "../utils/security-errors";
import { PasswordField } from "./password-field";
import { SecurityRow } from "./security-section";
import { SecurityRequestError } from "./security-request-error";
import { BackupCodes } from "./backup-codes";
import { useTranslations } from "next-intl";

export function TotpManagement({
  enabled,
  onChanged,
  onAccessError,
}: {
  enabled: boolean;
  onChanged: () => Promise<void>;
  onAccessError: (error: unknown) => void;
}) {
  const t = useTranslations("auth.security");
  const v = useTranslations("validation");
  const currentPasswordSchema = useMemo(
    () => createCurrentPasswordSchema(v),
    [v],
  );
  const [action, setAction] = useState<"disable" | "regenerate" | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const formRef = useRef<HTMLFormElement>(null);
  const form = useForm<CurrentPasswordValues>({
    resolver: zodResolver(currentPasswordSchema),
    defaultValues: { password: "" },
  });
  useShakeInvalidFields(formRef, form.formState.submitCount);
  const resetForm = form.reset;
  useEffect(() => () => resetForm(), [resetForm]);
  const mutation = useMutation({
    gcTime: 0,
    retry: false,
    mutationFn: async () => {
      const password = form.getValues("password");
      if (action === "disable") {
        securityData(await authClient.twoFactor.disable({ password }));
        form.reset();
        setAction(null);
        gooeyToast.success(t("totp.disabledTitle"), {
          description: t("totp.disabledDescription"),
        });
        await onChanged();
      } else {
        const result = securityData(
          await authClient.twoFactor.generateBackupCodes({ password }),
        );
        form.reset();
        setCodes(result.backupCodes);
        gooeyToast.success(t("totp.regeneratedTitle"), {
          description: t("totp.regeneratedDescription"),
        });
      }
    },
    onSuccess: (): void => {
      mutation.reset();
    },
    onError: onAccessError,
  });
  function close() {
    if (mutation.isPending) return;
    form.reset();
    setCodes(null);
    setAction(null);
    mutation.reset();
  }
  return (
    <>
      {enabled && (
        <>
          <SecurityRow
            icon={<ListChecks />}
            title={t("totp.backupTitle")}
            description={t("totp.backupDescription")}
            action={
              <Button
                variant="outline"
                className="min-h-11"
                onClick={() => setAction("regenerate")}
              >
                {t("totp.regenerate")}
              </Button>
            }
          />
          <SecurityRow
            icon={<ShieldOff />}
            title={t("totp.offTitle")}
            description={t("totp.offDescription")}
            action={
              <Button
                variant="destructive"
                className="min-h-11"
                onClick={() => setAction("disable")}
              >
                {t("totp.disable")}
              </Button>
            }
          />
        </>
      )}
      <Dialog
        open={action !== null}
        onOpenChange={(open) => {
          if (!open) close();
        }}
      >
        <DialogContent
          showCloseButton={false}
          className="max-h-[90dvh] overflow-y-auto sm:max-w-md"
          onInteractOutside={(event) => event.preventDefault()}
        >
          <DialogHeader>
            <DialogTitle>
              {codes
                ? t("totp.saveNewTitle")
                : action === "disable"
                  ? t("totp.disableTitle")
                  : t("totp.regenerateTitle")}
            </DialogTitle>
            <DialogDescription>
              {codes
                ? t("totp.saveNewDescription")
                : action === "disable"
                  ? t("totp.disableDescription")
                  : t("totp.regenerateDescription")}
            </DialogDescription>
          </DialogHeader>
          {codes ? (
            <div className="space-y-3">
              <BackupCodes codes={codes} onDone={close} />
              <Button
                variant="ghost"
                className="min-h-11 w-full"
                onClick={close}
              >
                {t("totp.closeWithoutSaving")}
              </Button>
            </div>
          ) : (
            <form
              ref={formRef}
              className="space-y-4"
              noValidate
              onSubmit={form.handleSubmit(() => mutation.mutate())}
            >
              <PasswordField
                id="manage-totp-password"
                label={t("totp.currentPassword")}
                autoComplete="current-password"
                disabled={mutation.isPending}
                error={form.formState.errors.password}
                registration={form.register("password")}
              />
              {Boolean(mutation.error) && (
                <SecurityRequestError error={mutation.error} />
              )}
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  className="min-h-11"
                  disabled={mutation.isPending}
                  onClick={close}
                >
                  {t("cancel")}
                </Button>
                <Button
                  type="submit"
                  variant={action === "disable" ? "destructive" : "default"}
                  className="min-h-11"
                  disabled={mutation.isPending}
                  aria-busy={mutation.isPending}
                >
                  {mutation.isPending && (
                    <LoaderCircle className="animate-spin" aria-hidden />
                  )}
                  {mutation.isPending
                    ? t("totp.updating")
                    : action === "disable"
                      ? t("totp.confirmDisable")
                      : t("totp.generate")}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
