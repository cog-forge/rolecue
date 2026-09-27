"use client";

import Image from "next/image";
import Link from "next/link";
import { Menu, X } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
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
import {
  landingContainer,
  landingFocus,
  primaryButton,
} from "./rolecue-landing-styles";

const navigation = [
  { href: "#practice", label: "Practice" },
  { href: "#method", label: "Method" },
  { href: "#questions", label: "Questions" },
] as const;

const navLink = cn(
  landingFocus,
  "grid min-h-11 place-items-center text-(length:--rolecue-type-body-sm) font-semibold text-(--rolecue-ink-muted) no-underline transition-colors duration-(--duration-quick) ease-(--ease-smooth-out) hover:text-(--rolecue-accent-hover) motion-reduce:transition-none!",
);

export function MarketingHeader() {
  const [isCampaignVisible, setIsCampaignVisible] = useState(true);

  return (
    <>
      <a
        className={cn(
          landingFocus,
          "fixed top-3 left-3 z-100 translate-y-[-160%] rounded-lg bg-(--rolecue-ink) px-4 py-3 text-(--rolecue-on-dark) transition-transform duration-(--duration-quick) ease-(--ease-smooth-out) focus:translate-y-0 motion-reduce:transition-none!",
        )}
        href="#main-content"
      >
        Skip to content
      </a>

      {isCampaignVisible ? (
        <div
          className="relative z-70 flex min-h-11 items-center justify-center gap-[0.65rem] overflow-hidden border-b border-[rgb(var(--rolecue-accent-rgb)/30%)] bg-(--rolecue-accent-soft) px-15 py-2 text-center text-[0.78rem] text-(--rolecue-ink) max-[760px]:items-start max-[760px]:gap-[0.42rem] max-[760px]:px-[3.2rem] max-[760px]:py-[0.55rem] max-[760px]:text-left max-[760px]:text-[0.68rem] max-[760px]:leading-[1.3]"
          role="status"
        >
          <span
            aria-hidden="true"
            className="size-[0.42rem] shrink-0 rounded-full bg-(--rolecue-accent) max-[760px]:mt-[0.22rem]"
          />
          <strong className="font-[780] max-[760px]:whitespace-nowrap">
            RoleCue / Practice
          </strong>
          <span>Bring a clearer point of view to the next conversation.</span>
          <Button
            aria-label="Dismiss announcement"
            className="absolute top-1/2 right-[0.85rem] size-8 -translate-y-1/2 rounded-full p-0 text-(--rolecue-ink) hover:bg-[rgb(var(--rolecue-ink-rgb)/8%)]"
            onClick={() => setIsCampaignVisible(false)}
            size="icon"
            type="button"
            variant="ghost"
          >
            <X aria-hidden="true" size={15} strokeWidth={1.8} />
          </Button>
        </div>
      ) : null}

      <header className="sticky top-0 z-60 h-[5.3rem] pointer-events-none max-[760px]:h-19">
        <nav
          aria-label="Main navigation"
          className={cn(landingContainer, "h-full pointer-events-auto")}
        >
          <div className="relative grid h-full grid-cols-[1fr_auto_1fr] items-center gap-6 max-[760px]:grid-cols-[1fr_auto]">
            <Link
              aria-label="RoleCue home"
              className={cn(
                landingFocus,
                "inline-flex w-[8.55rem] justify-self-start max-[760px]:w-[8.1rem]",
              )}
              href={routes.home}
            >
              <Image
                alt="RoleCue"
                className="h-auto w-full"
                height={48}
                priority
                src="/rolecue-logo.svg"
                width={314}
              />
            </Link>

            <div className="flex items-center gap-[2.1rem] max-[760px]:hidden">
              {navigation.map((item) => (
                <a className={navLink} href={item.href} key={item.href}>
                  {item.label}
                </a>
              ))}
            </div>

            <div className="flex items-center justify-self-end gap-3">
              <span className="font-mono text-[0.65rem] font-bold tracking-wider text-(--rolecue-ink-subtle) max-[1080px]:hidden">
                ROLE / RESPONSE / REVIEW
              </span>
              <Button
                asChild
                className={cn(
                  primaryButton,
                  landingFocus,
                  "min-h-[2.55rem] px-4 py-[0.55rem] text-[0.78rem] max-[760px]:hidden",
                )}
              >
                <Link href={routes.interviews.new.jobDescription}>
                  Start practice
                </Link>
              </Button>

              <Sheet>
                <SheetTrigger asChild>
                  <Button
                    aria-label="Open navigation"
                    className={cn(
                      landingFocus,
                      "hidden size-11 rounded-full border-(--rolecue-border) bg-(--rolecue-button-secondary) p-0 text-(--rolecue-ink) shadow-none hover:bg-(--rolecue-surface) max-[760px]:inline-flex",
                    )}
                    size="icon"
                    type="button"
                    variant="outline"
                  >
                    <Menu aria-hidden="true" size={20} strokeWidth={1.8} />
                  </Button>
                </SheetTrigger>
                <SheetContent
                  className="inset-x-4! top-4! h-auto rounded-2xl border border-(--rolecue-border) bg-[color-mix(in_srgb,var(--rolecue-surface)_94%,transparent)] p-[0.65rem] shadow-(--rolecue-shadow-media) backdrop-blur-[14px] motion-reduce:animate-none! motion-reduce:transition-none!"
                  side="top"
                  showCloseButton={false}
                >
                  <SheetTitle className="sr-only">
                    RoleCue navigation
                  </SheetTitle>
                  <SheetDescription className="sr-only">
                    Site navigation and practice links.
                  </SheetDescription>
                  <nav aria-label="Mobile navigation" className="grid gap-1">
                    {navigation.map((item) => (
                      <SheetClose asChild key={item.href}>
                        <a
                          className={cn(
                            landingFocus,
                            "flex min-h-[2.8rem] items-center justify-between rounded-3xl px-[0.65rem] py-[0.42rem] text-[0.9rem] text-(--rolecue-ink) no-underline [font-variation-settings:'wght'_700] hover:bg-(--rolecue-accent-soft)",
                          )}
                          href={item.href}
                        >
                          {item.label}
                          <span aria-hidden="true">↗</span>
                        </a>
                      </SheetClose>
                    ))}
                    <SheetClose asChild>
                      <Link
                        className={cn(
                          landingFocus,
                          "flex min-h-[2.8rem] items-center justify-between rounded-3xl px-[0.65rem] py-[0.42rem] text-[0.9rem] text-(--rolecue-ink) no-underline [font-variation-settings:'wght'_700] hover:bg-(--rolecue-accent-soft)",
                        )}
                        href={routes.interviews.new.jobDescription}
                      >
                        Start practice
                        <span aria-hidden="true">↗</span>
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
