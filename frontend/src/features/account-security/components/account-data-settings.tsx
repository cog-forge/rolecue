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
import { Input } from "@/components/ui/input";
import {
  SecurityPanel,
  SecurityRow,
  SecuritySection,
} from "@/features/auth/components/security-section";
import { useEndLocalSession } from "@/features/auth/hooks/use-sign-out";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { authClient } from "@/lib/auth/client";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { gooeyToast } from "goey-toast";
import { Download, LoaderCircle, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useId, useState } from "react";
import {
  deleteMyAccount,
  ERASURE_CONFIRMATION,
  EXPORT_LINK_MINUTES,
  fetchMyDataExport,
} from "../api/account";

// Saves the export as a file. The object URL is revoked right after the click.
function saveJson(data: unknown) {
  const blob = new Blob([JSON.stringify(data, null, 2)], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "rolecue-data-export.json";
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function AccountDataSettings() {
  const t = useTranslations("auth.security.data");
  const message = useApiErrorMessage();
  const user = useSessionUser();
  const [pending, setPending] = useState(false);

  async function download() {
    if (pending) return;
    setPending(true);
    try {
      saveJson(await fetchMyDataExport());
      gooeyToast.success(t("readyTitle"), {
        description: t("readyDescription"),
      });
    } catch (error) {
      gooeyToast.error(t("failedTitle"), {
        description: message(error, t("failedDescription")),
      });
    } finally {
      setPending(false);
    }
  }

  // The API has no export for Administrators.
  if (user.role === "admin") return null;
  return (
    <SecuritySection
      id="account-data"
      title={t("title")}
      description={t("description")}
    >
      <SecurityPanel>
        <SecurityRow
          icon={<Download />}
          title={t("rowTitle")}
          description={t("rowDescription", { minutes: EXPORT_LINK_MINUTES })}
          action={
            <Button
              variant="outline"
              className="min-h-11"
              disabled={pending}
              onClick={() => void download()}
            >
              {pending && <LoaderCircle className="animate-spin" aria-hidden />}
              {pending ? t("preparing") : t("download")}
            </Button>
          }
        />
      </SecurityPanel>
    </SecuritySection>
  );
}

export function DeleteAccountSettings() {
  const t = useTranslations("auth.security.danger");
  const user = useSessionUser();
  const [open, setOpen] = useState(false);
  // Administrator accounts are refused by the API (ADMIN_CANNOT_SELF_DELETE).
  if (user.role === "admin") return null;
  return (
    <SecuritySection
      id="delete-account"
      title={t("title")}
      description={t("description")}
    >
      <SecurityPanel>
        <SecurityRow
          icon={<Trash2 />}
          title={t("rowTitle")}
          description={t("rowDescription")}
          action={
            <Button
              variant="destructive"
              className="min-h-11"
              onClick={() => setOpen(true)}
            >
              {t("open")}
            </Button>
          }
        />
      </SecurityPanel>
      <DeleteAccountDialog open={open} onOpenChange={setOpen} />
    </SecuritySection>
  );
}

// The typed word lives inside the dialog content, so closing the dialog drops it. While the request
// runs, Esc and outside clicks are ignored so the outcome is never lost.
function DeleteAccountDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
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
        <DeleteAccountForm
          onBusyChange={setBusy}
          onCancel={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function DeleteAccountForm({
  onBusyChange,
  onCancel,
}: {
  onBusyChange: (busy: boolean) => void;
  onCancel: () => void;
}) {
  const t = useTranslations("auth.security.danger");
  const message = useApiErrorMessage();
  const endLocalSession = useEndLocalSession();
  const inputId = useId();
  const [typed, setTyped] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const matches = typed.trim() === ERASURE_CONFIRMATION;

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!matches || pending) return;
    setPending(true);
    onBusyChange(true);
    setError(null);
    try {
      await deleteMyAccount();
    } catch (failure) {
      setError(message(failure));
      setPending(false);
      onBusyChange(false);
      return;
    }
    // The API already removed the session; this call only clears the cookie, so its failure is moot.
    try {
      await authClient.signOut();
    } catch {
      // nothing to recover: the account is gone
    }
    gooeyToast.success(t("deletedTitle"), {
      description: t("deletedDescription"),
    });
    onBusyChange(false);
    await endLocalSession();
  }

  return (
    <form onSubmit={(event) => void submit(event)} className="space-y-4">
      <DialogHeader>
        <DialogTitle>{t("dialogTitle")}</DialogTitle>
        <DialogDescription>{t("dialogDescription")}</DialogDescription>
      </DialogHeader>
      <div className="space-y-2">
        <label htmlFor={inputId} className="text-sm font-medium">
          {t("confirmLabel", { word: t("confirmWord") })}
        </label>
        <Input
          id={inputId}
          value={typed}
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          disabled={pending}
          aria-invalid={error ? true : undefined}
          onChange={(event) => setTyped(event.target.value)}
        />
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
      </div>
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          className="min-h-11"
          disabled={pending}
          onClick={onCancel}
        >
          {t("cancel")}
        </Button>
        <Button
          type="submit"
          variant="destructive"
          className="min-h-11"
          disabled={!matches || pending}
        >
          {pending && <LoaderCircle className="animate-spin" aria-hidden />}
          {pending ? t("deleting") : t("confirm")}
        </Button>
      </DialogFooter>
    </form>
  );
}
