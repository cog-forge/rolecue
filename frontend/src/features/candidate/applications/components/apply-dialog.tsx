"use client";

import { useState } from "react";
import Link from "next/link";
import { CircleCheck } from "lucide-react";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("candidate.applications.apply");
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
              <DialogTitle>{t("sentTitle")}</DialogTitle>
              <DialogDescription>
                {t("sentBody", { title: posting.title })}
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
                {t("keepBrowsing")}
              </Button>
              <Button asChild className="h-11">
                <Link href={applicationHref(created.id)}>{t("viewMine")}</Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("title", { title: posting.title })}</DialogTitle>
              <DialogDescription>
                {where ? t("descriptionWhere", { where }) : t("description")}
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
