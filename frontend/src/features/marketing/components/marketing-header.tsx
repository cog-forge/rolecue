"use client";

import Link from "next/link";
import { useState } from "react";
import { useMotionValueEvent, useScroll } from "motion/react";
import { ArrowRight, Menu, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { RoleCueMark } from "@/components/brand/rolecue-mark";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { landingFocus, primaryButton } from "./rolecue-landing-styles";

const navigation = [
  { href: "#about", label: "Features" },
  { href: "#practice", label: "How it works" },
  { href: "#questions", label: "Questions" },
] as const;

const compactHeaderThreshold = 48;

const navLink = cn(
  landingFocus,
  "grid min-h-11 place-items-center text-base font-medium text-(--rolecue-ink-muted) no-underline transition-colors duration-150 hover:text-(--rolecue-brand-blue) motion-reduce:transition-none",
);

const mobileNavLink = cn(
  landingFocus,
  "flex min-h-11 items-center justify-between border-b border-(--rolecue-border-soft) px-3 text-[0.95rem] font-medium text-(--rolecue-ink) no-underline transition-colors duration-150 hover:text-(--rolecue-brand-blue) motion-reduce:transition-none",
);

export function MarketingHeader() {
  const [isCompact, setIsCompact] = useState(false);
  const { scrollY } = useScroll();

  useMotionValueEvent(scrollY, "change", (current) => {
    const shouldBeCompact = current > compactHeaderThreshold;
    setIsCompact((wasCompact) =>
      wasCompact === shouldBeCompact ? wasCompact : shouldBeCompact,
    );
  });

  return (
    <>
      <a
        className={cn(
          landingFocus,
          "fixed top-3 left-3 z-100 translate-y-[-160%] rounded-lg bg-(--rolecue-ink) px-4 py-3 text-(--rolecue-on-primary) transition-transform duration-150 focus:translate-y-0 motion-reduce:transition-none",
        )}
        href="#main-content"
      >
        Skip to content
      </a>

      <header className="pointer-events-none sticky top-0 z-60 flex h-[5.3rem] justify-center -mb-[5.3rem] max-[760px]:h-19 max-[760px]:-mb-19">
        <nav
          aria-label="Main navigation"
          className={cn(
            "pointer-events-auto min-w-0 h-full transition-[width,height,margin,padding,border-radius,background-color,border-color,box-shadow] duration-400 ease-[cubic-bezier(0.22,0.61,0.36,1)] motion-reduce:transition-none! max-[760px]:px-3",
            isCompact
              ? "mt-3 h-14 w-[min(80rem,calc(100%-2rem))] rounded-full border border-(--rolecue-border-soft) bg-[color-mix(in_srgb,var(--rolecue-surface)_94%,transparent)] px-3 shadow-(--rolecue-shadow-low) backdrop-blur-md"
              : "mt-2 sm:mt-3 lg:mt-4 h-[4.2rem] w-full max-w-[1440px] border-transparent bg-transparent shadow-none px-4 sm:px-8 lg:px-12 max-[760px]:w-[calc(100%-1.5rem)] sm:max-[760px]:w-[calc(100%-2.5rem)]",
          )}
        >
          <div className="grid h-full grid-cols-[auto_1fr] items-center gap-6 max-[760px]:grid-cols-[minmax(0,1fr)_auto] max-[760px]:gap-3">
            <Link
              aria-label="RoleCue home"
              className={cn(
                landingFocus,
                "inline-flex items-center gap-2.5 justify-self-start transition-transform duration-300",
              )}
              href={routes.home}
            >
              <div
                className={cn(
                  "transition-[width,height] duration-400 ease-[cubic-bezier(0.22,0.61,0.36,1)] motion-reduce:transition-none!",
                  isCompact
                    ? "size-9 max-[760px]:size-8"
                    : "size-10 sm:size-11 max-[760px]:size-9",
                )}
              >
                <RoleCueMark
                  alt=""
                  className="size-full object-contain"
                  priority
                />
              </div>
              <span
                className={cn(
                  "font-bold tracking-tight text-[#1E2229] transition-[opacity,font-size] duration-300",
                  isCompact
                    ? "text-lg max-[600px]:hidden"
                    : "text-xl sm:text-2xl",
                )}
              >
                RoleCue
              </span>
            </Link>

            <div className="flex items-center justify-self-end gap-9 max-[920px]:gap-6 max-[760px]:gap-2">
              <div className="flex items-center gap-9 max-[920px]:gap-6 max-[760px]:hidden">
                {navigation.map((item) => (
                  <a className={navLink} href={item.href} key={item.href}>
                    {item.label}
                  </a>
                ))}
              </div>
              <Button
                asChild
                className={cn(
                  primaryButton,
                  landingFocus,
                  "min-h-11 px-4 py-2 text-[0.8rem] max-[760px]:px-3 max-[760px]:text-xs",
                )}
              >
                <Link href={routes.login}>Sign in</Link>
              </Button>

              <Sheet>
                <SheetTrigger asChild>
                  <Button
                    aria-label="Open navigation"
                    className={cn(
                      landingFocus,
                      "hidden size-11 rounded-lg border-(--rolecue-border) bg-(--rolecue-surface) p-0 text-(--rolecue-ink) shadow-none hover:border-(--rolecue-border-strong) hover:bg-(--rolecue-surface) max-[760px]:inline-flex",
                    )}
                    size="icon"
                    type="button"
                    variant="outline"
                  >
                    <Menu aria-hidden="true" size={19} strokeWidth={1.8} />
                  </Button>
                </SheetTrigger>
                <SheetContent
                  className="inset-x-3! top-3! h-auto gap-2 rounded-xl border border-(--rolecue-border) bg-(--rolecue-surface) p-3 shadow-(--rolecue-shadow-media) motion-reduce:animate-none! motion-reduce:transition-none!"
                  side="top"
                  showCloseButton={false}
                >
                  <div className="flex min-h-11 items-center justify-between px-2">
                    <SheetTitle className="text-sm font-semibold text-(--rolecue-ink)">
                      Explore RoleCue
                    </SheetTitle>
                    <SheetDescription className="sr-only">
                      Site navigation and account access.
                    </SheetDescription>
                    <SheetClose asChild>
                      <Button
                        aria-label="Close navigation"
                        className={cn(
                          landingFocus,
                          "size-11 rounded-lg text-(--rolecue-ink)",
                        )}
                        size="icon"
                        type="button"
                        variant="ghost"
                      >
                        <X aria-hidden="true" size={19} strokeWidth={1.8} />
                      </Button>
                    </SheetClose>
                  </div>
                  <nav aria-label="Mobile navigation" className="grid">
                    {navigation.map((item) => (
                      <SheetClose asChild key={item.href}>
                        <a className={mobileNavLink} href={item.href}>
                          {item.label}
                        </a>
                      </SheetClose>
                    ))}
                    <SheetClose asChild>
                      <Link
                        className={cn(
                          primaryButton,
                          landingFocus,
                          "group/button mt-3 min-h-11 w-full justify-between px-4 text-sm",
                        )}
                        href={routes.login}
                      >
                        Sign in
                        <ArrowRight
                          aria-hidden="true"
                          className="transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover/button:translate-x-1 motion-reduce:transform-none! motion-reduce:transition-none!"
                          size={16}
                        />
                      </Link>
                    </SheetClose>
                  </nav>
                </SheetContent>
              </Sheet>
            </div>
          </div>
        </nav>
      </header>
    </>
  );
}
