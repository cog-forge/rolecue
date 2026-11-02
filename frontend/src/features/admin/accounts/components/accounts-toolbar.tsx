import { Search } from "lucide-react";
import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { accountFilters, type AccountQuery } from "../api/accounts";

export function AccountsToolbar({
  query,
  search,
  onSearchChange,
  disabled,
  onChange,
}: {
  query: AccountQuery;
  search: string;
  onSearchChange: (value: string) => void;
  disabled: boolean;
  onChange: (query: AccountQuery) => void;
}) {
  const t = useTranslations("admin.accounts.toolbar");
  const roles = useTranslations("common.shell.roles");
  return (
    <div
      role="search"
      aria-label={t("label")}
      className="grid items-end gap-3 sm:grid-cols-[minmax(0,1fr)_13rem]"
    >
      <div className="flex flex-col gap-2">
        <label htmlFor="account-search" className="text-xs font-medium">
          {t("searchLabel")}
        </label>
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <Input
            id="account-search"
            type="search"
            className="h-11 bg-background pl-9 shadow-none"
            placeholder={t("searchPlaceholder")}
            maxLength={500}
            disabled={disabled}
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
          />
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <label htmlFor="account-filter" className="text-xs font-medium">
          {t("filterLabel")}
        </label>
        <Select
          value={query.filter}
          disabled={disabled}
          onValueChange={(value) =>
            onChange({
              ...query,
              offset: 0,
              searchValue: search.trim(),
              filter: accountFilters.parse(value),
            })
          }
        >
          <SelectTrigger
            id="account-filter"
            className="w-full bg-background shadow-none data-[size=default]:h-11"
            aria-describedby={
              query.filter === "locked" ? "locked-filter-hint" : undefined
            }
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            <SelectItem value="all">{t("filterAll")}</SelectItem>
            <SelectItem value="locked">{t("filterLocked")}</SelectItem>
            <SelectItem value="candidate">{roles("candidate")}</SelectItem>
            <SelectItem value="recruiter">{roles("recruiter")}</SelectItem>
            <SelectItem value="admin">{roles("admin")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {query.filter === "locked" && (
        <p
          id="locked-filter-hint"
          className="text-xs text-muted-foreground sm:col-span-2"
        >
          {t("lockedHint")}
        </p>
      )}
    </div>
  );
}
