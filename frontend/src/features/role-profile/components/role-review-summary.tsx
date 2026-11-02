"use client";

import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";
import { useTranslations } from "next-intl";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import {
  isCustomTarget,
  isKnownTarget,
  type RoleProfile,
} from "../api/contract";
import {
  proficiencyLabel,
  requirementLabel,
  seniorityLabel,
} from "../lib/labels";

export function RoleReviewSummary({ profile }: { profile: RoleProfile }) {
  const t = useTranslations("candidate.roleProfiles.summary");
  const labels = useTranslations("candidate.roleProfiles.labels");
  return (
    <div className="space-y-7">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold">{profile.title}</h2>
        {profile.seniorityLevel && (
          <Badge variant="secondary" className="h-7 px-3 text-sm">
            {seniorityLabel(labels, profile.seniorityLevel)}
          </Badge>
        )}
      </div>
      <section>
        <div className="border-b pb-4">
          <h2 className="text-lg font-semibold">
            {t("requirementsTitle")}{" "}
            <span className="ml-2 text-sm font-normal text-muted-foreground">
              {profile.requirements.length}
            </span>
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("requirementsHelp")}
          </p>
        </div>
        <ul>
          {profile.requirements.map((item, index) => {
            const customTarget = isCustomTarget(item);
            return (
              <li
                key={`${item.kind}-${item.name}`}
                className="min-w-0 border-b px-2 py-5"
              >
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="min-w-0 text-base font-medium break-words">
                    {item.name}
                  </p>
                  <Badge
                    variant="secondary"
                    aria-label={t("proficiencyLabel", { name: item.name })}
                    className="h-7 px-3 text-sm"
                  >
                    {isKnownTarget(item.target)
                      ? proficiencyLabel(labels, item.target)
                      : customTarget
                        ? labels("savedTarget")
                        : proficiencyLabel(labels, "Unspecified")}
                  </Badge>
                </div>
                {customTarget && (
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {item.target}
                  </p>
                )}
                <p className="mt-2 text-xs text-muted-foreground">
                  {requirementLabel(labels, item)}
                </p>
                {item.evidenceText && (
                  <Accordion type="single" collapsible className="mt-1">
                    <AccordionItem value="source">
                      <AccordionTrigger
                        aria-label={t("sourceLabel", { number: index + 1 })}
                        className="min-h-11 items-center text-xs font-normal text-muted-foreground"
                      >
                        {t("fromJd")}
                      </AccordionTrigger>
                      <AccordionContent>
                        <blockquote className="border-l-2 pl-3 leading-relaxed text-muted-foreground">
                          “{item.evidenceText}”
                        </blockquote>
                      </AccordionContent>
                    </AccordionItem>
                  </Accordion>
                )}
              </li>
            );
          })}
        </ul>
      </section>
      {profile.refinementNote && (
        <div className="space-y-2 rounded-xl border bg-card p-5 sm:p-6">
          <h2 className="font-semibold">{t("noteTitle")}</h2>
          <p className="text-sm leading-relaxed whitespace-pre-wrap text-muted-foreground">
            {profile.refinementNote}
          </p>
        </div>
      )}
      <div className="flex flex-wrap items-center justify-between gap-5 border-t pt-6">
        <div className="flex items-start gap-3">
          <span
            className="t-success-check mt-1 shrink-0 text-workspace-accent"
            data-state="in"
            aria-hidden="true"
          >
            <Check className="size-5" />
          </span>
          <div>
            <h2 className="font-semibold">{t("savedTitle")}</h2>
            <p role="status" className="mt-1 text-sm text-muted-foreground">
              {t("savedStatus")}
            </p>
          </div>
        </div>
        <Button asChild className="h-auto min-h-11 py-2 whitespace-normal">
          <Link
            href={`${routes.interviews.new.setup}?roleProfileId=${encodeURIComponent(profile.id)}`}
          >
            {t("continue")} <ArrowRight aria-hidden="true" />
          </Link>
        </Button>
      </div>
    </div>
  );
}
