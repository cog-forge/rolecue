"use client";
import { useEffect, useRef } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import { ArrowUpRight, ChevronDown } from "lucide-react";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from "@/components/ui/accordion";
import { AiLoader } from "@/components/ui/ai-loader";
import { LoadingMorph } from "@/components/ui/loading-morph";
import { ErrorState } from "@/components/ui/error-state";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import type { RoleSummary } from "@/features/role-profile/schemas/role-profile-schema";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Roadmap } from "../api/roadmap";
import { RoadmapUpdateNotice } from "./roadmap-screen";
import { RoadmapPreview } from "./roadmap-preview";

export function RoadmapOverviewRow({
  profile,
  query,
  stalled,
  onRetry,
  open,
  onOpenChange,
}: {
  profile: RoleSummary;
  query?: UseQueryResult<Roadmap>;
  stalled: boolean;
  onRetry: () => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("candidate.roadmapOverview");
  const errorMessage = useApiErrorMessage();
  const fmt = useFormatters();
  const timers = useRef<{
    enter?: ReturnType<typeof setTimeout>;
    leave?: ReturnType<typeof setTimeout>;
  }>({});
  const hovered = useRef(false);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const current = timers.current;
    return () => {
      clearTimeout(current.enter);
      clearTimeout(current.leave);
    };
  }, []);
  const data = query?.data;
  const ready = data?.generation_status === "ready";
  const canPreview = ready && data.items.length > 0;
  const state = stalled
    ? "stalled"
    : data?.generation_status !== "ready"
      ? data?.generation_status
      : data.learning_status;
  return (
    <Accordion
      type="single"
      collapsible
      value={open ? "preview" : ""}
      onValueChange={(value) => {
        clearTimeout(timers.current.enter);
        clearTimeout(timers.current.leave);
        onOpenChange(value === "preview");
      }}
    >
      <AccordionItem
        value="preview"
        className="t-acc border-0"
        data-open={open}
      >
        <article
          data-roadmap-preview
          onPointerEnter={(event) => {
            if (event.pointerType !== "mouse" || !canPreview) return;
            hovered.current = true;
            clearTimeout(timers.current.leave);
            const focusedRow = document.activeElement?.closest(
              "[data-roadmap-preview]",
            );
            if (focusedRow && focusedRow !== event.currentTarget) return;
            clearTimeout(timers.current.enter);
            timers.current.enter = setTimeout(() => onOpenChange(true), 120);
          }}
          onPointerLeave={(event) => {
            if (event.pointerType !== "mouse") return;
            hovered.current = false;
            clearTimeout(timers.current.enter);
            if (event.currentTarget.contains(document.activeElement)) return;
            timers.current.leave = setTimeout(() => onOpenChange(false), 180);
          }}
          onBlur={(event) => {
            if (
              !event.currentTarget.contains(event.relatedTarget) &&
              !hovered.current
            )
              onOpenChange(false);
          }}
          onKeyDown={(event) => {
            if (event.key === "Escape" && open) {
              event.preventDefault();
              event.stopPropagation();
              clearTimeout(timers.current.enter);
              clearTimeout(timers.current.leave);
              onOpenChange(false);
              trigger.current?.focus();
            }
          }}
        >
          <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_auto] lg:items-center">
            <div className="min-w-0 space-y-2">
              <h3 className="text-base font-medium [overflow-wrap:anywhere]">
                {profile.title}
              </h3>
              <p className="text-sm text-muted-foreground">
                {profile.status !== "confirmed"
                  ? t("needsConfirmation")
                  : query?.isError && !data
                    ? t("unavailableProgress")
                    : state
                      ? t(`status.${state}`)
                      : t("loading")}
              </p>
            </div>
            <div className="min-w-0">
              {query?.isPending ? (
                <LoadingMorph variant="compact" label={t("loading")} />
              ) : query?.isError && !data ? (
                <ErrorState
                  variant="compact"
                  description={errorMessage(query.error, t("loadFailed"))}
                  onRetry={onRetry}
                  isRetrying={query.isFetching}
                />
              ) : ready ? (
                <div className="space-y-2">
                  <p className="flex flex-wrap justify-between gap-2 text-sm tabular-nums">
                    <span>
                      {t("progress", {
                        count: data.completed_items,
                        total: data.total_items,
                      })}
                    </span>
                    <span>
                      {data.progress_percent === null
                        ? t("unavailableProgress")
                        : fmt.percent(data.progress_percent, 0)}
                    </span>
                  </p>
                  {data.progress_percent !== null && (
                    <div
                      role="progressbar"
                      aria-label={t("progressLabel", { title: profile.title })}
                      aria-valuemin={0}
                      aria-valuemax={100}
                      aria-valuenow={data.progress_percent}
                      className="h-1.5 overflow-hidden rounded-full bg-muted"
                    >
                      <div
                        className="h-full rounded-full bg-rolecue-brand transition-[width] duration-(--duration-slow) motion-reduce:transition-none"
                        style={{ width: `${data.progress_percent}%` }}
                      />
                    </div>
                  )}
                </div>
              ) : data?.generation_status === "generating" && !stalled ? (
                <AiLoader
                  stage="generating"
                  variant="compact"
                  title={t("status.generating")}
                />
              ) : null}
              {query?.isError && data && (
                <ErrorState
                  variant="compact"
                  description={t("refreshFailed")}
                  onRetry={onRetry}
                  isRetrying={query.isFetching}
                />
              )}
              {data && <RoadmapUpdateNotice roadmap={data} />}
            </div>
            <div className="flex flex-wrap items-center gap-2 justify-self-start">
              <Button
                asChild
                variant="outline"
                className="h-auto min-h-11 justify-self-start whitespace-normal py-2"
              >
                <Link
                  href={
                    profile.status === "confirmed"
                      ? routes.roleProfiles.roadmap(profile.id)
                      : routes.roleProfiles.detail(profile.id)
                  }
                >
                  {profile.status !== "confirmed"
                    ? t("reviewProfile")
                    : ready
                      ? t("viewRoadmap")
                      : t("viewStatus")}
                  <ArrowUpRight aria-hidden="true" />
                </Link>
              </Button>
              {canPreview && (
                <AccordionTrigger
                  ref={trigger}
                  showIndicator={false}
                  aria-label={t("preview.toggle", { title: profile.title })}
                  className="min-h-11 items-center gap-2 px-3 hover:bg-muted/60 hover:no-underline"
                >
                  <span>{t("preview.action")}</span>
                  <ChevronDown
                    aria-hidden="true"
                    className="t-acc-chevron size-4 text-muted-foreground"
                  />
                </AccordionTrigger>
              )}
            </div>
          </div>
          {canPreview && (
            <AccordionContent
              motion="smooth"
              forceMount
              inert={!open}
              aria-hidden={!open}
            >
              <RoadmapPreview roadmap={data} open={open} />
            </AccordionContent>
          )}
        </article>
      </AccordionItem>
    </Accordion>
  );
}
