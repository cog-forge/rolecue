"use client";

import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { JDReviewSummary } from "@/features/job-description";
import type { RoleProfile } from "../api/contract";

export function RoleReviewSummary({ profile }: { profile: RoleProfile }) {
  return (
    <JDReviewSummary profile={profile}>
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
            <h2 className="font-semibold">Your role review is saved.</h2>
            <p role="status" className="mt-1 text-sm text-muted-foreground">
              Your requirements and interview content are saved. Your Role
              Profile is ready for the next step.
            </p>
          </div>
        </div>
        <Button asChild className="h-11">
          <Link
            href={`${routes.interviews.new.setup}?roleProfileId=${encodeURIComponent(profile.id)}`}
          >
            Continue to setup <ArrowRight aria-hidden="true" />
          </Link>
        </Button>
      </div>
    </JDReviewSummary>
  );
}
