"use client";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { TriangleAlert } from "lucide-react";
import type { SetupBlocker, SetupTarget } from "../api/saved-setup";
import { blockerHref, isBlockerCode } from "../lib/setup-blockers";
import type { SetupParams } from "../lib/setup-params";

// What still stops the interview from starting, as the server computed it. Each line says what is
// wrong and, where it can be fixed, links to the step that fixes it.
export function SetupBlockers({
  blockers,
  target,
  params,
}: {
  blockers: SetupBlocker[];
  target: SetupTarget;
  params: SetupParams;
}) {
  const t = useTranslations("candidate.setup.blockers");
  if (blockers.length === 0) return null;
  return (
    <section
      aria-label={t("title")}
      className="space-y-3 rounded-xl border border-destructive/30 bg-card p-5 text-sm"
    >
      <h2 className="flex items-center gap-2 font-semibold">
        <TriangleAlert aria-hidden="true" className="size-4 text-destructive" />
        {t("title")}
      </h2>
      <p className="text-muted-foreground">{t("description")}</p>
      <ul className="divide-y">
        {blockers.map((blocker) => {
          const fix = blockerHref(blocker, target, params);
          return (
            <li
              key={`${blocker.code}-${blocker.field ?? ""}`}
              className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 py-2 first:pt-0 last:pb-0"
            >
              <span>
                {isBlockerCode(blocker.code)
                  ? t(`messages.${blocker.code}`)
                  : t("messages.unknown")}
              </span>
              {isLocked(blocker, target) ? null : (
                <Link
                  href={fix.href}
                  onClick={(event) => focusHere(event, fix)}
                  className="inline-flex min-h-11 items-center font-medium underline underline-offset-4"
                >
                  {t(`actions.${fix.kind}`)}
                </Link>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

// A locked avatar or voice is the company's to fix, so there is nothing for the Candidate to open.
function isLocked(blocker: SetupBlocker, target: SetupTarget) {
  return target.mode === "recruitment" && blocker.code.startsWith("locked_");
}

// A link to a section of this very page moves focus to it, so keyboard users land on the picker.
function focusHere(
  event: React.MouseEvent<HTMLAnchorElement>,
  fix: { href: string; kind: string },
) {
  if (fix.kind !== "here") return;
  event.preventDefault();
  const section = document.getElementById(fix.href.slice(1));
  section?.scrollIntoView({ block: "center" });
  section?.focus({ preventScroll: true });
}
