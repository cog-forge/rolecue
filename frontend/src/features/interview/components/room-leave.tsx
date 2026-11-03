"use client";
import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowUpRight, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { routes } from "@/config/routes";

const leaveButton =
  "h-10 rounded-full border border-white/15 bg-neutral-900/55 px-4 text-xs text-white shadow-lg backdrop-blur-xl hover:bg-neutral-900/80 hover:text-white";

/**
 * The room's Leave. There is no pause: leaving ends the interview (decision 2026-11-03). While the
 * session can still be ended, Leave asks for confirmation and explains what happens, then submits at
 * once; the room shows "submitting…" and opens the report. Once the session has ended (or the room can
 * no longer run it), Leave is a plain link back to the history.
 */
export function RoomLeave({
  live,
  sessionType,
  submitting,
  onLeave,
}: {
  /** The session can still be ended from this room. */
  live: boolean;
  sessionType: "practice" | "recruitment" | null;
  submitting: boolean;
  onLeave: () => void;
}) {
  const t = useTranslations("candidate.interviewRoom");
  const [open, setOpen] = useState(false);
  if (!live)
    return (
      <Button asChild variant="ghost" className={leaveButton}>
        <Link href={routes.history}>
          {t("leave")}
          <ArrowUpRight aria-hidden className="size-3.5" />
        </Link>
      </Button>
    );
  return (
    <>
      <Button
        variant="ghost"
        className={leaveButton}
        disabled={submitting}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        {submitting ? (
          <LoaderCircle
            aria-hidden
            className="size-3.5 motion-safe:animate-spin"
          />
        ) : null}
        {submitting ? t("leaveDialog.submitting") : t("leave")}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("leaveDialog.title")}</DialogTitle>
            <DialogDescription>
              {t("leaveDialog.description")}
            </DialogDescription>
          </DialogHeader>
          <ul className="list-disc space-y-1.5 pl-5 text-sm text-muted-foreground">
            <li>{t("leaveDialog.answers")}</li>
            {sessionType === "practice" && <li>{t("leaveDialog.practice")}</li>}
            {sessionType === "recruitment" && (
              <li>{t("leaveDialog.recruitment")}</li>
            )}
            <li>{t("leaveDialog.disconnect")}</li>
          </ul>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("leaveDialog.stay")}</Button>
            </DialogClose>
            <Button
              variant="destructive"
              onClick={() => {
                setOpen(false);
                onLeave();
              }}
            >
              {t("leaveDialog.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
