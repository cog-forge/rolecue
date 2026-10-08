"use client";

import { useId, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutGroup, motion } from "motion/react";
import {
  ChevronsUpDown,
  LogOut,
  Moon,
  Monitor,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  Shield,
  Sun,
  UserRound,
} from "lucide-react";
import { UserAvatar } from "@/components/account/user-avatar";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { RoleCueMark } from "@/components/brand/rolecue-mark";
import {
  getActiveNavigation,
  roleLabels,
  workspaceHome,
} from "@/config/navigation";
import { routes } from "@/config/routes";
import type { ProductRole } from "@/config/permissions";
import { useWorkspacePreferences } from "@/providers/workspace-provider";
import { cn } from "@/lib/utils";
import {
  WorkspaceNavigationIcon,
  workspaceNavigationWithMotion,
} from "./workspace-navigation-icon";

export type WorkspaceUser = {
  id: string;
  full_name: string;
  image?: string | null;
  email: string;
  role: ProductRole;
};
export type WorkspaceAccountActions = {
  onSignOut: () => Promise<void>;
  signingOut: boolean;
  signOutError: boolean;
};

function Hint({
  label,
  enabled,
  children,
}: {
  label: string;
  enabled: boolean;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      {enabled && (
        <TooltipContent side="right" sideOffset={12} className="z-100">
          {label}
        </TooltipContent>
      )}
    </Tooltip>
  );
}

export function WorkspaceSidebar({
  user,
  collapsed,
  onToggle,
  onSearch,
  onNavigate,
  onSignOut,
  signingOut,
  signOutError,
}: {
  user: WorkspaceUser;
  collapsed: boolean;
  onToggle?: () => void;
  onSearch: () => void;
  onNavigate: () => void;
} & WorkspaceAccountActions) {
  const pathname = usePathname();
  const active = getActiveNavigation(user.role, pathname);
  const [hovered, setHovered] = useState<string | null>(null);
  const preferences = useWorkspacePreferences();
  const { reducedMotion } = preferences;
  const layoutId = useId();
  const accountTrigger = useRef<HTMLButtonElement>(null);
  const labelClass = cn(
    "min-w-0 truncate whitespace-nowrap transition-opacity duration-200 motion-reduce:transition-none",
    collapsed && "pointer-events-none absolute left-14 opacity-0",
  );
  return (
    <div className="flex h-full min-h-0 flex-col gap-3 p-3">
      <div
        className={cn(
          "flex min-h-12 shrink-0 items-center",
          collapsed ? "justify-center" : "gap-2",
        )}
      >
        {collapsed && onToggle ? (
          <Hint label="Expand sidebar" enabled>
            <Button
              variant="ghost"
              className="group relative size-12 rounded-xl"
              aria-label="Expand sidebar"
              aria-expanded={false}
              onClick={onToggle}
            >
              <RoleCueMark className="size-8 object-contain transition-opacity group-hover:opacity-0 group-focus-visible:opacity-0 motion-reduce:transition-none" />
              <PanelLeftOpen
                className="absolute size-5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100 motion-reduce:transition-none"
                aria-hidden
              />
            </Button>
          </Hint>
        ) : (
          <>
            <Link
              href={workspaceHome(user.role)}
              onClick={onNavigate}
              aria-label="RoleCue workspace"
              className="flex min-w-0 flex-1 items-center gap-2 rounded-xl focus-visible:outline-2 focus-visible:outline-ring"
            >
              <RoleCueMark className="size-10 shrink-0 object-contain" />
              <span className="min-w-0">
                <span className="block text-lg font-bold tracking-tight">
                  RoleCue
                </span>
                <span className="block truncate text-xs text-muted-foreground">
                  {roleLabels[user.role]} workspace
                </span>
              </span>
            </Link>
            {onToggle && (
              <Button
                variant="ghost"
                className="size-11 shrink-0 rounded-xl"
                onClick={onToggle}
                aria-label="Collapse sidebar"
                aria-expanded
              >
                <PanelLeftClose className="size-5" aria-hidden />
              </Button>
            )}
          </>
        )}
      </div>

      <Hint label="Search pages" enabled={collapsed}>
        <Button
          variant="secondary"
          onClick={onSearch}
          aria-label="Search pages"
          className={cn(
            "relative h-11 shrink-0 justify-start gap-3 overflow-hidden rounded-xl px-3 text-muted-foreground",
            collapsed && "justify-center px-0",
          )}
        >
          <Search className="size-5" aria-hidden />
          <span className={labelClass}>Search pages</span>
          {!collapsed && (
            <kbd className="ml-auto rounded border bg-background/70 px-1.5 text-[10px]">
              ⌘ / Ctrl K
            </kbd>
          )}
        </Button>
      </Hint>

      <LayoutGroup id={layoutId}>
        <motion.nav
          layoutScroll
          aria-label={`${roleLabels[user.role]} navigation`}
          className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-contain"
          onMouseLeave={() => setHovered(null)}
        >
          {workspaceNavigationWithMotion[user.role].map((group) => (
            <div key={group.label} className="mb-3 last:mb-0">
              <div className="flex h-8 items-center px-3">
                {collapsed ? (
                  <Separator />
                ) : (
                  <span className="text-[10px] font-semibold tracking-[0.14em] text-muted-foreground uppercase">
                    {group.label}
                  </span>
                )}
              </div>
              <ul className="space-y-1">
                {group.items.map((item) => {
                  const selected = active?.href === item.href;
                  return (
                    <li key={item.href}>
                      <Hint label={item.label} enabled={collapsed}>
                        <Link
                          href={item.href}
                          aria-label={item.label}
                          aria-current={selected ? "page" : undefined}
                          onClick={onNavigate}
                          onMouseEnter={() => setHovered(item.href)}
                          onFocus={() => setHovered(item.href)}
                          onBlur={() => setHovered(null)}
                          className={cn(
                            "group relative flex min-h-11 items-center gap-3 rounded-xl px-2 text-sm font-medium text-muted-foreground outline-offset-2 focus-visible:outline-2 focus-visible:outline-ring",
                            collapsed && "justify-center px-0",
                            selected && "font-semibold text-foreground",
                          )}
                        >
                          {hovered === item.href && !selected && (
                            <motion.span
                              aria-hidden
                              layoutId="hover"
                              initial={false}
                              transition={{
                                duration: reducedMotion ? 0 : 0.25,
                              }}
                              className="pointer-events-none absolute inset-0 rounded-xl bg-sidebar-accent/60"
                            />
                          )}
                          {selected && (
                            <motion.span
                              aria-hidden
                              layoutId="active"
                              initial={false}
                              transition={{
                                duration: reducedMotion ? 0 : 0.35,
                                ease: [0.32, 0.72, 0, 1],
                              }}
                              className={cn(
                                "pointer-events-none absolute inset-0 rounded-xl bg-sidebar-accent",
                                collapsed && "bg-transparent",
                              )}
                            />
                          )}
                          <span
                            className={cn(
                              "relative grid size-8 shrink-0 place-items-center rounded-[10px] transition-colors",
                              selected && "bg-workspace-accent text-white",
                            )}
                          >
                            <WorkspaceNavigationIcon
                              icon={item.icon}
                              animatedIcon={item.animatedIcon}
                              animate={hovered === item.href}
                            />
                          </span>
                          <span className={cn("relative", labelClass)}>
                            {item.label}
                          </span>
                        </Link>
                      </Hint>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </motion.nav>
      </LayoutGroup>

      <div className="shrink-0 space-y-1 border-t pt-2">
        <DropdownMenu>
          <Hint label={user.full_name || "Your account"} enabled={collapsed}>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                ref={accountTrigger}
                aria-label="Open account menu"
                className={cn(
                  "h-14 w-full justify-start gap-2 rounded-xl px-1",
                  collapsed && "justify-center px-0",
                )}
              >
                <UserAvatar
                  id={user.id}
                  name={user.full_name}
                  image={user.image}
                />
                {!collapsed && (
                  <>
                    <span className="min-w-0 flex-1 text-left">
                      <span className="block truncate text-sm font-semibold">
                        {user.full_name || "Your account"}
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        {roleLabels[user.role]}
                      </span>
                    </span>
                    <ChevronsUpDown
                      className="size-4 text-muted-foreground"
                      aria-hidden
                    />
                  </>
                )}
              </Button>
            </DropdownMenuTrigger>
          </Hint>
          <DropdownMenuContent
            side={collapsed ? "right" : "top"}
            align="start"
            sideOffset={12}
            className="z-100 w-64 rounded-2xl p-1.5 motion-reduce:animate-none"
          >
            <DropdownMenuLabel className="px-3 py-2">
              <span className="block truncate font-semibold">
                {user.full_name || "Your account"}
              </span>
              <span className="block truncate text-xs font-normal text-muted-foreground">
                {user.email}
              </span>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem asChild className="min-h-11 gap-3 px-3">
                <Link
                  href={routes.profile}
                  onClick={onNavigate}
                  aria-current={
                    pathname === routes.profile ? "page" : undefined
                  }
                >
                  <UserRound aria-hidden />
                  Your Profile
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem asChild className="min-h-11 gap-3 px-3">
                <Link
                  href={routes.settings}
                  onClick={onNavigate}
                  aria-current={
                    pathname === routes.settings ? "page" : undefined
                  }
                >
                  <Shield aria-hidden />
                  Account Security
                </Link>
              </DropdownMenuItem>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <div className="flex min-h-12 items-center justify-between gap-3 px-3 py-1.5">
              <DropdownMenuLabel className="p-0 text-xs text-muted-foreground">
                Appearance
              </DropdownMenuLabel>
              <DropdownMenuRadioGroup
                value={preferences.theme}
                aria-label="Color theme"
                className="flex items-center rounded-full border bg-muted/50 p-0.5"
              >
                {(
                  [
                    { value: "light", label: "Light", icon: Sun },
                    { value: "system", label: "System", icon: Monitor },
                    { value: "dark", label: "Dark", icon: Moon },
                  ] as const
                ).map(({ value, label, icon: Icon }) => (
                  <DropdownMenuRadioItem
                    key={value}
                    value={value}
                    aria-label={label}
                    title={label}
                    className="size-8 justify-center rounded-full p-0 text-muted-foreground transition-colors data-[state=checked]:bg-background data-[state=checked]:text-foreground data-[state=checked]:shadow-sm focus-visible:ring-2 focus-visible:ring-ring [&_[data-slot=dropdown-menu-radio-item-indicator]]:hidden"
                    onSelect={() => {
                      if (accountTrigger.current)
                        void preferences.setTheme(
                          value,
                          accountTrigger.current,
                        );
                    }}
                  >
                    <Icon aria-hidden />
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </div>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              className="min-h-11 gap-3 px-3"
              disabled={signingOut}
              onSelect={(event) => {
                event.preventDefault();
                void onSignOut();
              }}
            >
              <LogOut aria-hidden />
              {signingOut ? "Signing out…" : "Sign out"}
            </DropdownMenuItem>
            {signOutError && (
              <p role="alert" className="px-3 py-2 text-xs text-destructive">
                Sign out did not complete. Please try again.
              </p>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
