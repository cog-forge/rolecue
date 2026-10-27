"use client";

import { useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { ArrowUpRight, FileText, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { RoleProfile } from "../api/contract";
import { getOriginalDocument, roleError } from "../api/requests";

export function OriginalRoleDocument({ profile }: { profile: RoleProfile }) {
  const tabRef = useRef<Window | null>(null);
  const [popupBlocked, setPopupBlocked] = useState(false);
  const open = useMutation({
    mutationFn: () => getOriginalDocument(profile.id),
    onSuccess: (url) => {
      const tab = tabRef.current;
      if (!tab || tab.closed) return;
      tab.opener = null;
      tab.location.replace(url);
    },
    onError: () => {
      tabRef.current?.close();
      tabRef.current = null;
    },
  });
  if (!profile.hasOriginalDocument) return null;
  return (
    <div className="rounded-xl border bg-card p-5">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <FileText
            className="mt-1 size-5 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <div className="min-w-0">
            <p className="text-xs text-muted-foreground">Original PDF</p>
            <p className="mt-1 text-sm font-medium">Private document</p>
          </div>
        </div>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={open.isPending}
          onClick={() => {
            const tab = window.open("about:blank", "_blank");
            if (!tab) {
              setPopupBlocked(true);
              return;
            }
            setPopupBlocked(false);
            tabRef.current = tab;
            open.mutate();
          }}
        >
          {open.isPending ? (
            <LoaderCircle
              className="size-4 motion-safe:animate-spin"
              aria-hidden="true"
            />
          ) : (
            <ArrowUpRight className="size-4" aria-hidden="true" />
          )}
          {open.isPending ? "Opening PDF…" : "Open original PDF"}
        </Button>
      </div>
      {open.error && (
        <p role="alert" className="mt-3 text-sm text-destructive">
          {roleError(open.error)}
        </p>
      )}
      {popupBlocked && (
        <p role="alert" className="mt-3 text-sm text-destructive">
          Allow pop-ups for this site to open the private PDF in a new tab.
        </p>
      )}
    </div>
  );
}
