"use client";
import Link from "next/link";
import { Suspense } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import { interviewSteps } from "@/config/navigation";
export function WizardNavigation() {
  return (
    <Suspense fallback={null}>
      <WizardLinks />
    </Suspense>
  );
}
function WizardLinks() {
  const pathname = usePathname();
  const profileId = useSearchParams().get("roleProfileId");
  return (
    <nav aria-label="Interview creation steps">
      <ol className="flex flex-wrap gap-4">
        {interviewSteps.map((step, index) => (
          <li key={step.href}>
            <Link
              href={
                profileId
                  ? `${step.href}?roleProfileId=${encodeURIComponent(profileId)}`
                  : step.href
              }
              aria-current={pathname === step.href ? "step" : undefined}
              className="rounded-sm text-sm underline-offset-4 hover:underline aria-[current=step]:font-semibold aria-[current=step]:underline"
            >
              {index + 1}. {step.label}
            </Link>
          </li>
        ))}
      </ol>
    </nav>
  );
}
