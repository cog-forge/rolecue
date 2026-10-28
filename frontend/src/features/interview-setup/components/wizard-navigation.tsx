"use client";
import Link from "next/link";
import { Suspense } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import { Check } from "lucide-react";
import { interviewSteps } from "@/config/navigation";
import { cn } from "@/lib/utils";
import { readSetupParams, setupHref } from "../lib/setup-params";

export function WizardNavigation() {
  return (
    <Suspense fallback={null}>
      <WizardLinks />
    </Suspense>
  );
}

// The practice wizard. A recruitment interview enters at the preflight from its Application and has
// no practice steps, so the navigation steps aside.
function WizardLinks() {
  const pathname = usePathname();
  const params = readSetupParams(useSearchParams());
  if (params.applicationId) return null;
  const { roleProfileId, avatarId, voiceId } = params;
  const active = interviewSteps.findIndex((step) => step.href === pathname);
  return (
    <nav aria-label="Interview creation steps">
      <ol className="flex flex-wrap items-center gap-x-5 gap-y-3 text-xs sm:gap-x-8">
        {interviewSteps.map((step, index) => {
          const current = index === active;
          const done = active > index;
          return (
            <li key={step.href}>
              <Link
                href={setupHref(step.href, {
                  roleProfileId,
                  avatarId,
                  voiceId,
                })}
                aria-current={current ? "step" : undefined}
                className={cn(
                  "group flex min-h-11 items-center gap-2 rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  current
                    ? "font-medium text-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    "flex size-6 shrink-0 items-center justify-center rounded-full tabular-nums transition-colors duration-(--duration-fast) motion-reduce:transition-none",
                    done && "bg-workspace-accent/10 text-workspace-accent",
                    current && "bg-foreground text-background",
                    !done &&
                      !current &&
                      "border group-hover:border-foreground/40",
                  )}
                >
                  {done ? <Check className="size-3.5" /> : index + 1}
                </span>
                {step.label}
              </Link>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
