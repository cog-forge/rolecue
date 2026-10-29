"use client";

import { useState } from "react";
import Link from "next/link";
import { CircleCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { CreatedApplication } from "../schemas/candidate-application-schema";
import { applicationHref } from "../utils/application-links";
import { CvApplyForm } from "./cv-apply-form";
import { HiringSteps } from "./hiring-steps";

export type ApplyPosting = {
  id: string;
  title: string;
  company_name: string | null;
  location: string | null;
};

// Applying happens in a dialog over the posting: send the CV, then see what happens next. The
// dialog cannot be dismissed while the upload runs.
export function ApplyDialog({
  posting,
  open,
  onOpenChange,
}: {
  posting: ApplyPosting;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<CreatedApplication | null>(null);
  const where = [posting.company_name, posting.location]
    .filter(Boolean)
    .join(" · ");

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!busy) onOpenChange(next);
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-lg">
        {created ? (
          <>
            <DialogHeader>
              <span className="mb-1 flex size-10 items-center justify-center rounded-full bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">
                <CircleCheck className="size-5" aria-hidden />
              </span>
              <DialogTitle>Application sent</DialogTitle>
              <DialogDescription>
                Your CV for {posting.title} is with the recruiter. Here is what
                happens next.
              </DialogDescription>
            </DialogHeader>
            <HiringSteps current={0} className="rounded-xl border p-4" />
            <DialogFooter className="gap-2 sm:justify-between">
              <Button
                type="button"
                variant="outline"
                className="h-11"
                onClick={() => onOpenChange(false)}
              >
                Keep browsing
              </Button>
              <Button asChild className="h-11">
                <Link href={applicationHref(created.id)}>
                  View my application
                </Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Apply for {posting.title}</DialogTitle>
              <DialogDescription>
                {where ? `${where}. ` : ""}The recruiter reviews your CV first;
                no interview starts from this step.
              </DialogDescription>
            </DialogHeader>
            <CvApplyForm
              postingId={posting.id}
              onApplied={setCreated}
              onBusyChange={setBusy}
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
