"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Skeleton } from "@/components/ui/skeleton";
import { roleLabels, getActiveNavigation } from "@/config/navigation";
import { useWorkspacePreferences } from "@/providers/workspace-provider";
import {
  WorkspaceSidebar,
  type WorkspaceAccountActions,
  type WorkspaceUser,
} from "./workspace-sidebar";
import { WorkspaceSearch } from "./workspace-search";
import { cn } from "@/lib/utils";

export function ApplicationShell({
  children,
  user,
  ...actions
}: {
  children: React.ReactNode;
  user: WorkspaceUser;
} & WorkspaceAccountActions) {
  const preferences = useWorkspacePreferences();
  const pathname = usePathname();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const searchOrigin = useRef<HTMLElement | null>(null);
  const mobileTrigger = useRef<HTMLButtonElement>(null);
  const openSearch = () => {
    searchOrigin.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    setSearchOpen(true);
  };
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || event.altKey || event.shiftKey)
        return;
      const target = event.target;
      if (event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (searchOpen) setSearchOpen(false);
        else openSearch();
      }
      if (
        event.key.toLowerCase() === "b" &&
        !(
          target instanceof HTMLElement &&
          (target.isContentEditable ||
            /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))
        )
      ) {
        event.preventDefault();
        if (preferences.mobile) setDrawerOpen((open) => !open);
        else preferences.toggleCollapsed();
      }
    };
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [preferences, searchOpen]);
  useEffect(() => {
    const breakpoint = window.matchMedia("(min-width: 768px)");
    const closeDrawer = () => {
      if (breakpoint.matches) setDrawerOpen(false);
    };
    breakpoint.addEventListener("change", closeDrawer);
    return () => breakpoint.removeEventListener("change", closeDrawer);
  }, []);
  const closeNavigation = () => {
    setDrawerOpen(false);
    setSearchOpen(false);
  };
  if (!preferences.ready)
    return (
      <div role="status" className="p-6">
        <span className="sr-only">Preparing your workspace…</span>
        <Skeleton className="h-24 w-full" />
      </div>
    );
  // Runtime keeps session protection, but leaves workspace chrome behind.
  if (/^\/interviews\/[^/]+\/room$/.test(pathname))
    return (
      <main id="main-content" className="min-h-dvh p-6">
        {children}
      </main>
    );
  const pageLabel =
    getActiveNavigation(user.role, pathname)?.label ??
    (pathname === "/profile"
      ? "Your Profile"
      : pathname === "/settings"
        ? "Account Security"
        : "Workspace");
  return (
    <TooltipProvider delayDuration={150}>
      <a
        href="#main-content"
        className="sr-only z-100 rounded-lg bg-primary px-4 py-3 text-primary-foreground focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        Skip to content
      </a>
      <div
        data-slot="workspace-shell"
        className="flex min-h-dvh bg-white dark:bg-background"
      >
        {!preferences.mobile && (
          <aside
            aria-label="Workspace sidebar"
            className={cn(
              "sticky top-3 m-3 mr-0 h-[calc(100dvh-24px)] shrink-0 rounded-3xl border bg-sidebar text-sidebar-foreground shadow-sm transition-[width] duration-550 ease-[cubic-bezier(.32,.72,0,1)] motion-reduce:transition-none",
              preferences.collapsed ? "w-[72px]" : "w-[264px]",
            )}
          >
            <WorkspaceSidebar
              user={user}
              collapsed={preferences.collapsed}
              onToggle={preferences.toggleCollapsed}
              onSearch={openSearch}
              onNavigate={closeNavigation}
              {...actions}
            />
          </aside>
        )}
        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex min-h-20 items-center gap-3 border-b border-border/60 px-5 md:px-8">
            {preferences.mobile && (
              <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
                <SheetTrigger asChild>
                  <Button
                    ref={mobileTrigger}
                    variant="ghost"
                    className="size-11"
                    aria-label="Open workspace navigation"
                  >
                    <Menu className="size-5" aria-hidden />
                  </Button>
                </SheetTrigger>
                <SheetContent
                  side="left"
                  className="z-90 w-[min(320px,calc(100vw-32px))]! gap-0 p-0 [&>[data-slot=sheet-close]]:size-11"
                >
                  <SheetTitle className="sr-only">
                    Workspace navigation
                  </SheetTitle>
                  <SheetDescription className="sr-only">
                    {roleLabels[user.role]} pages and account actions.
                  </SheetDescription>
                  <WorkspaceSidebar
                    user={user}
                    collapsed={false}
                    onSearch={openSearch}
                    onNavigate={closeNavigation}
                    {...actions}
                  />
                </SheetContent>
              </Sheet>
            )}
            <div className="min-w-0">
              <p className="text-[10px] font-semibold tracking-[.14em] text-muted-foreground uppercase">
                {roleLabels[user.role]} workspace
              </p>
              <p className="mt-1 truncate text-sm font-semibold">{pageLabel}</p>
            </div>
            <span className="ml-auto hidden text-xs text-muted-foreground sm:block">
              A steady place to prepare.
            </span>
          </header>
          <main
            id="main-content"
            tabIndex={-1}
            className="mx-auto w-full max-w-7xl flex-1 space-y-6 px-5 py-8 outline-none md:px-8 md:py-10 lg:px-10"
          >
            {children}
          </main>
        </div>
      </div>
      <WorkspaceSearch
        role={user.role}
        open={searchOpen}
        onOpenChange={setSearchOpen}
        onNavigate={closeNavigation}
        returnFocus={() => {
          if (searchOrigin.current?.isConnected) searchOrigin.current.focus();
          else mobileTrigger.current?.focus();
        }}
      />
    </TooltipProvider>
  );
}
