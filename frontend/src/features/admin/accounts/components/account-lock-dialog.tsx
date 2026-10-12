import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { AccountAction, AdminAccount } from "../api/accounts";
import {
  createLockFormSchema,
  lockRequest,
  type LockFormValues,
} from "../schemas/account-lock";
import { AccountLockFields } from "./account-lock-fields";
import { accountName } from "./accounts-table";

export function AccountLockDialog({
  user,
  pending,
  blocked,
  error,
  onCancel,
  onConfirm,
  onRestoreFocus,
}: {
  user: AdminAccount;
  pending: boolean;
  blocked: boolean;
  error: string | null;
  onCancel: () => void;
  onConfirm: (action: AccountAction) => Promise<void>;
  onRestoreFocus: () => void;
}) {
  const locked = user.banned === true;
  const form = useForm<LockFormValues>({
    resolver: zodResolver(createLockFormSchema()),
    defaultValues: {
      reason: "",
      indefinite: false,
      duration: "",
      expiresAt: "",
    },
  });
  const submit = async (values: LockFormValues) => {
    if (pending || blocked) return;
    let request;
    try {
      request = lockRequest(user.id, values);
    } catch {
      form.setError("expiresAt", {
        message: "Choose a valid date and time in the future.",
      });
      return;
    }
    await onConfirm({ kind: "lock", request });
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onCancel();
      }}
    >
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto p-6 sm:max-w-lg"
        showCloseButton={false}
        onEscapeKeyDown={(event) => {
          if (pending) event.preventDefault();
        }}
        onPointerDownOutside={(event) => {
          if (pending) event.preventDefault();
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          onRestoreFocus();
        }}
      >
        <DialogHeader>
          <DialogTitle>
            {locked ? "Unlock account" : "Lock account"}
          </DialogTitle>
          <DialogDescription>
            {locked
              ? "Allow this user to sign in again. Previously revoked sessions will stay invalid. A notification email will be sent after the change."
              : "Prevent this user from signing in and revoke all of their active sessions. A notification email will be sent after the change."}
          </DialogDescription>
        </DialogHeader>
        <div className="min-w-0 rounded-lg border bg-muted/40 p-3">
          <p className="break-words text-sm font-medium">{accountName(user)}</p>
          <p className="mt-1 break-all text-xs text-muted-foreground">
            {user.email}
          </p>
        </div>
        <form
          noValidate
          aria-label={
            locked ? "Unlock account confirmation" : "Lock account confirmation"
          }
          className="space-y-5"
          onSubmit={
            locked
              ? (event) => {
                  event.preventDefault();
                  if (!pending && !blocked)
                    void onConfirm({
                      kind: "unlock",
                      request: { userId: user.id },
                    });
                }
              : (event) => {
                  void form.handleSubmit(submit)(event);
                }
          }
        >
          {!locked && (
            <AccountLockFields form={form} disabled={pending || blocked} />
          )}
          {error && (
            <p
              role="alert"
              className="rounded-lg border border-destructive/20 bg-destructive/5 p-3 text-sm text-destructive"
            >
              {error}
            </p>
          )}
          <DialogFooter className="m-0 rounded-none border-t bg-transparent p-0 pt-4">
            <Button
              type="button"
              className="h-11"
              variant="outline"
              disabled={pending}
              onClick={onCancel}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              className="h-11"
              variant={locked ? "default" : "destructive"}
              disabled={pending || blocked}
              aria-busy={pending}
            >
              {pending && (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden
                />
              )}
              {pending
                ? "Updating…"
                : locked
                  ? "Confirm unlock"
                  : "Confirm lock"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
