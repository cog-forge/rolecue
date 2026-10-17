import {
  DataTable,
  type DataTableColumn,
} from "@/components/data-table/data-table";
import { UserAvatar } from "@/components/account/user-avatar";
import type { AdminAccount } from "../api/accounts";
import { AccountRoleBadge, AccountLockBadge } from "./account-badges";
import { AccountRowActions } from "./account-row-actions";
import { isAccountLocked } from "../schemas/account-state";

export function accountName(user: AdminAccount) {
  return user.name?.trim() || user.email;
}
export function accountDate(date: Date) {
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZoneName: "short",
  }).format(date);
}

export function AccountsTable({
  users,
  checkedAt,
  disabled,
  onAction,
  onView,
}: {
  users: AdminAccount[];
  checkedAt: number;
  disabled: boolean;
  onAction: (user: AdminAccount, trigger: HTMLButtonElement | null) => void;
  onView: (user: AdminAccount, trigger: HTMLButtonElement | null) => void;
}) {
  const columns: DataTableColumn<AdminAccount>[] = [
    {
      id: "account",
      header: "Account",
      cell: (user) => (
        <div className="flex items-center gap-3">
          <UserAvatar
            id={user.id}
            name={accountName(user)}
            image={user.image}
          />
          <div className="min-w-0 max-w-64">
            <p className="truncate font-medium" title={accountName(user)}>
              {accountName(user)}
            </p>
            <p className="mt-1 break-all text-xs text-muted-foreground">
              {user.email}
            </p>
          </div>
        </div>
      ),
    },
    {
      id: "role",
      header: "Role",
      cell: (user) => <AccountRoleBadge role={user.role} />,
    },
    {
      id: "lock",
      header: "Lock state",
      className: "max-w-72 px-5 py-4",
      cell: (user) => (
        <>
          <AccountLockBadge user={user} checkedAt={checkedAt} />
          {isAccountLocked(user, checkedAt) && (
            <>
              <p className="mt-2 break-words text-xs text-muted-foreground">
                {user.banReason || "No reason recorded"}
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                {user.banExpires
                  ? `Until ${accountDate(user.banExpires)}`
                  : "Indefinitely"}
              </p>
            </>
          )}
        </>
      ),
    },
    {
      id: "created",
      header: "Created",
      className: "whitespace-nowrap px-5 py-4 text-xs text-muted-foreground",
      cell: (user) => (
        <time dateTime={user.createdAt.toISOString()}>
          {accountDate(user.createdAt)}
        </time>
      ),
    },
    {
      id: "actions",
      header: "Actions",
      className: "px-5 py-2",
      cell: (user) => (
        <AccountRowActions
          user={user}
          checkedAt={checkedAt}
          disabled={disabled}
          onAction={onAction}
          onView={onView}
        />
      ),
    },
  ];
  return (
    <DataTable
      rows={users}
      columns={columns}
      getRowId={(user) => user.id}
      label="Platform accounts"
    />
  );
}
