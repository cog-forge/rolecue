"use client";

import type { ReactNode } from "react";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import {
  isCustomTarget,
  isKnownTarget,
  requirementLabel,
  type RoleProfile,
} from "../api/contract";

export function JDReviewSummary({
  profile,
  children,
}: {
  profile: RoleProfile;
  children?: ReactNode;
}) {
  return (
    <div className="space-y-7">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold">{profile.title}</h2>
        {profile.seniorityLevel && (
          <Badge variant="secondary" className="h-7 px-3 text-sm capitalize">
            {profile.seniorityLevel}
          </Badge>
        )}
      </div>
      <section>
        <div className="border-b pb-4">
          <h2 className="text-lg font-semibold">
            Technical requirements{" "}
            <span className="ml-2 text-sm font-normal text-muted-foreground">
              {profile.requirements.length}
            </span>
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            The requirements and proficiency saved for this role.
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
                    aria-label={`Proficiency for ${item.name}`}
                    className="h-7 px-3 text-sm"
                  >
                    {isKnownTarget(item.target)
                      ? item.target
                      : customTarget
                        ? "Saved target"
                        : "Unspecified"}
                  </Badge>
                </div>
                {customTarget && (
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {item.target}
                  </p>
                )}
                <p className="mt-2 text-xs text-muted-foreground">
                  {requirementLabel(item)}
                </p>
                {item.evidenceText && (
                  <Accordion type="single" collapsible className="mt-1">
                    <AccordionItem value="source">
                      <AccordionTrigger
                        aria-label={`Source for requirement ${index + 1}`}
                        className="min-h-11 items-center text-xs font-normal text-muted-foreground"
                      >
                        From the JD
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
          <h2 className="font-semibold">Your refinement note</h2>
          <p className="text-sm leading-relaxed whitespace-pre-wrap text-muted-foreground">
            {profile.refinementNote}
          </p>
        </div>
      )}
      {children}
    </div>
  );
}
