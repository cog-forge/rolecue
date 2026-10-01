"use client";

import { RoleCueMark } from "@/components/brand/rolecue-mark";
import {
  FacebookIcon,
  GitHubIcon,
  GoogleIcon,
} from "@/components/icons/social-icons";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { ArrowLeft, Info } from "lucide-react";
import Link from "next/link";
import * as React from "react";

export type OAuthProvider = "google" | "github" | "facebook";

type OAuthProviderDetails = {
  name: string;
  Mark: React.ComponentType<React.ComponentProps<"svg">>;
  markClassName?: string;
};

const providerDetails: Record<OAuthProvider, OAuthProviderDetails> = {
  google: { name: "Google", Mark: GoogleIcon },
  github: { name: "GitHub", Mark: GitHubIcon, markClassName: "text-foreground" },
  facebook: {
    name: "Facebook",
    Mark: FacebookIcon,
    markClassName: "text-[#1877f2]",
  },
};

export interface OAuthPreviewCardProps {
  provider: OAuthProvider;
}

export function OAuthPreviewCard({ provider }: OAuthPreviewCardProps) {
  const [clicked, setClicked] = React.useState(false);
  const {
    name: providerName,
    Mark: ProviderMark,
    markClassName = "",
  } = providerDetails[provider];

  return (
    <div className="mx-auto w-full max-w-[440px] rounded-2xl border border-border bg-card p-6 text-card-foreground shadow-(--rolecue-shadow-low) transition-all duration-300 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-500 sm:p-8">
      {/* Back to sign in link */}
      <div className="mb-5">
        <Link
          href={routes.login}
          className="inline-flex items-center gap-2 rounded-md py-1 pr-1.5 pl-0 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
        >
          <ArrowLeft className="size-3.5" />
          <span>Back to sign in</span>
        </Link>
      </div>

      {/* Visual Handoff Bridge: RoleCue <---> Provider */}
      <div className="flex items-center justify-center gap-4 my-3 py-2">
        <div className="flex size-14 items-center justify-center rounded-2xl border border-border bg-muted shadow-xs">
          <RoleCueMark className="size-9 shrink-0" priority />
        </div>

        <div className="flex items-center gap-1 text-muted-foreground">
          <span className="h-0.5 w-3 animate-pulse rounded-full bg-border" />
          <span className="h-0.5 w-3 rounded-full bg-muted-foreground" />
          <span className="h-0.5 w-3 animate-pulse rounded-full bg-border" />
        </div>

        <div className="flex size-14 items-center justify-center rounded-2xl border border-border bg-muted shadow-xs">
          <ProviderMark className={`size-7 shrink-0 ${markClassName}`} />
        </div>
      </div>

      {/* Heading & Subheading */}
      <div className="text-center mt-4 mb-6">
        <div className="mb-2.5 inline-flex items-center gap-1.5 rounded-full border border-rolecue-brand/20 bg-rolecue-brand/10 px-2.5 py-0.5 text-[11px] font-semibold text-rolecue-brand">
          <Info className="size-3" />
          <span>Authorization Preview</span>
        </div>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">
          Continue with {providerName}
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
          OAuth sign-in is not connected yet. When OAuth integration is
          available, RoleCue will hand off to {providerName} for authorization.
        </p>
      </div>

      {/* Transparent Preview Mode Callout */}
      <div className="mb-5 flex items-start gap-3 rounded-xl border border-border bg-muted p-4 text-left text-xs leading-relaxed text-muted-foreground">
        <Info className="mt-0.5 size-4 shrink-0 text-foreground" />
        <div>
          <span className="mb-1 block font-semibold text-foreground">
            Preview Mode
          </span>
          OAuth sign-in is not connected yet. When OAuth integration is
          available, RoleCue will hand off to {providerName} for authorization.
        </div>
      </div>

      {/* Feedback banner if clicked */}
      {clicked && (
        <div
          role="alert"
          className="mb-5 animate-in slide-in-from-top-1 rounded-xl border border-rolecue-brand/25 bg-rolecue-brand/10 p-3.5 text-xs leading-relaxed text-rolecue-brand fade-in duration-200"
        >
          <strong>Handoff Preview:</strong> OAuth sign-in is not connected yet.
          When OAuth integration is available, RoleCue will hand off to{" "}
          {providerName} for authorization.
        </div>
      )}

      {/* Actions */}
      <div className="space-y-2.5">
        <Button
          type="button"
          onClick={() => setClicked(true)}
          className="h-11 w-full rounded-xl bg-primary px-4 text-sm font-medium text-primary-foreground shadow-sm transition hover:bg-primary/90 active:scale-[0.99] sm:h-12"
        >
          Preview {providerName} handoff
        </Button>

        <Button
          asChild
          variant="outline"
          className="h-11 w-full rounded-xl border-border bg-card px-4 text-sm font-medium text-card-foreground shadow-2xs transition hover:bg-muted active:scale-[0.99]"
        >
          <Link href={routes.login}>Cancel</Link>
        </Button>
      </div>
    </div>
  );
}
