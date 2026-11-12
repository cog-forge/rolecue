"use client";

import { useContext, useEffect, useLayoutEffect, useRef } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import type { PostingStatus } from "../schemas/job-posting-schema";
import { postingInboxHref, takesApplications } from "../utils/posting-guidance";
import { PostingNavigationContext } from "./posting-navigation";

export type PostingTab =
  "overview" | "applications" | "questions" | "evaluation";

const tabClass =
  "inline-flex min-h-11 items-center border-b-2 border-transparent text-sm whitespace-nowrap transition-colors duration-[var(--duration-quick)] outline-none focus-visible:ring-3 focus-visible:ring-ring/50";

// The sections of one posting, as real links so each has its own address. Applications opens the
// review inbox for this posting and stays off until the posting can take applications.
export function PostingTabs({
  postingId,
  active,
  status,
}: {
  postingId: string;
  active: PostingTab;
  status: PostingStatus;
}) {
  const t = useTranslations("recruiter.postings.tabs");
  const navigation = useContext(PostingNavigationContext);
  const bar = useRef<HTMLUListElement>(null);
  const pill = useRef<HTMLSpanElement>(null);
  const positioned = useRef(false);
  const measureRef = useRef<(animate: boolean) => void>(() => {});
  const tabs: { id: PostingTab; label: string; href: string }[] = [
    {
      id: "overview",
      label: t("overview"),
      href: routes.recruiter.jobPosting(postingId),
    },
    {
      id: "applications",
      label: t("applications"),
      href: postingInboxHref(postingId),
    },
    {
      id: "questions",
      label: t("questions"),
      href: routes.recruiter.jobPostingQuestions(postingId),
    },
    {
      id: "evaluation",
      label: t("evaluation"),
      href: routes.recruiter.jobPostingEvaluation(postingId),
    },
  ];
  const selectedHref =
    navigation?.pendingHref ?? tabs.find((tab) => tab.id === active)?.href;
  const getIndicator = navigation?.getIndicator;
  const rememberIndicator = navigation?.rememberIndicator;
  useLayoutEffect(() => {
    const list = bar.current;
    const indicator = pill.current;
    if (!list || !indicator) return;
    const measure = (animate: boolean) => {
      const target = list.querySelector<HTMLElement>('[data-active="true"]');
      if (!target) return;
      const bounds = { left: target.offsetLeft, width: target.offsetWidth };
      const write = (position: { left: number; width: number }) => {
        indicator.style.transform = `translateX(${position.left}px)`;
        indicator.style.width = `${position.width}px`;
      };
      if (!animate || !positioned.current) {
        indicator.style.transition = "none";
        write(animate ? (getIndicator?.() ?? bounds) : bounds);
        void indicator.offsetWidth;
        indicator.style.transition = "";
      }
      write(bounds);
      positioned.current = true;
      rememberIndicator?.(bounds);
    };
    measureRef.current = measure;
    measure(true);
  }, [selectedHref, getIndicator, rememberIndicator, t]);
  useEffect(() => {
    if (!bar.current) return;
    let first = true;
    const resize = new ResizeObserver(() => {
      if (first) first = false;
      else measureRef.current(false);
    });
    resize.observe(bar.current);
    return () => resize.disconnect();
  }, []);
  return (
    <div className="border-b" data-posting-tabs>
      <nav
        aria-label={t("label")}
        aria-busy={navigation?.pendingHref ? true : undefined}
        className="-mb-px max-w-full overflow-x-auto"
      >
        <ul ref={bar} className="relative flex gap-6">
          <span
            ref={pill}
            aria-hidden
            className="t-tabs-pill top-auto! bottom-0 h-0.5! rounded-none! bg-foreground!"
            data-posting-indicator
          />
          {tabs.map((tab) => {
            const selected = tab.id === active;
            if (tab.id === "applications" && !takesApplications(status))
              return (
                <li key={tab.id}>
                  {/* A link with no href is the standard disabled link; the reason is read aloud. */}
                  <a
                    role="link"
                    aria-disabled="true"
                    title={t("unavailable")}
                    className={cn(
                      tabClass,
                      "cursor-not-allowed text-muted-foreground/60",
                    )}
                  >
                    {tab.label}
                    <span className="sr-only"> {t("unavailableHint")}</span>
                  </a>
                </li>
              );
            return (
              <li key={tab.id}>
                <Link
                  href={tab.href}
                  prefetch={true}
                  data-active={tab.href === selectedHref}
                  onMouseEnter={() => {
                    if (navigation) void navigation.prefetch(tab.href);
                  }}
                  onFocus={() => {
                    if (navigation) void navigation.prefetch(tab.href);
                  }}
                  onTouchStart={() => {
                    if (navigation) void navigation.prefetch(tab.href);
                  }}
                  onNavigate={(event) => {
                    if (navigation && tab.id !== "applications") {
                      event.preventDefault();
                      navigation.navigate(tab.href);
                    }
                  }}
                  aria-current={selected ? "page" : undefined}
                  className={cn(
                    tabClass,
                    selected
                      ? "font-medium text-foreground"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  {tab.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </div>
  );
}
