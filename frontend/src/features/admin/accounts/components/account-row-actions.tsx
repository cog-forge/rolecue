import { useRef } from "react";
import { Eye, LockKeyhole, MoreHorizontal, UnlockKeyhole } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { AdminAccount } from "../api/accounts";
import { isAccountLocked } from "../schemas/account-state";

export function AccountRowActions({
  user,
  checkedAt,
  disabled,
  onAction,
  onView,
}: {
  user: AdminAccount;
  checkedAt: number;
  disabled: boolean;
  onAction: (user: AdminAccount, trigger: HTMLButtonElement | null) => void;
  onView: (user: AdminAccount, trigger: HTMLButtonElement | null) => void;
}) {
  const trigger = useRef<HTMLButtonElement>(null);
  const openingDialog = useRef(false);
  const managed = user.role === "candidate" || user.role === "recruiter";
  const name = user.name?.trim() || user.email;
  const locked = isAccountLocked(user, checkedAt);
  const Icon = locked ? UnlockKeyhole : LockKeyhole;
  return (
    <DropdownMenu
      onOpenChange={(open) => {
        if (open) openingDialog.current = false;
      }}
    >
      <DropdownMenuTrigger asChild>
        <Button
          ref={trigger}
          variant="ghost"
          className="size-11"
          disabled={disabled}
          aria-label={`Actions for ${name}`}
        >
          <MoreHorizontal aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="min-w-44"
        onCloseAutoFocus={(event) => {
          if (openingDialog.current) event.preventDefault();
        }}
      >
        <DropdownMenuItem
          className="min-h-11"
          onSelect={() => {
            openingDialog.current = true;
            onView(user, trigger.current);
          }}
        >
          <Eye aria-hidden />
          View details
        </DropdownMenuItem>
        <DropdownMenuItem
          className="min-h-11"
          disabled={!managed || disabled}
          variant={locked ? "default" : "destructive"}
          onSelect={() => {
            openingDialog.current = true;
            onAction(user, trigger.current);
          }}
        >
          <Icon aria-hidden />
          {managed ? (locked ? "Unlock account" : "Lock account") : "Read only"}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
