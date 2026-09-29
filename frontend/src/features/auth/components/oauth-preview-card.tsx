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
  github: { name: "GitHub", Mark: GitHubIcon, markClassName: "text-[#121814]" },
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
    <div className="mx-auto w-full max-w-[440px] rounded-2xl border border-[#dbdbdb] bg-white p-6 shadow-[0_4px_32px_rgba(0,0,0,0.06)] transition-all duration-300 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-500 sm:p-8">
      {/* Back to sign in link */}
      <div className="mb-5">
        <Link
          href={routes.login}
          className="inline-flex items-center gap-2 text-xs font-medium text-[#5c5c5c] hover:text-[#121814] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#202020]/30 rounded-md py-1 px-1.5 -ml-1.5"
        >
          <ArrowLeft className="size-3.5" />
          <span>Back to sign in</span>
        </Link>
      </div>

      {/* Visual Handoff Bridge: RoleCue <---> Provider */}
      <div className="flex items-center justify-center gap-4 my-3 py-2">
        <div className="flex size-14 items-center justify-center rounded-2xl border border-[#ebebeb] bg-[#fafafa] shadow-xs">
          <RoleCueMark className="size-9 shrink-0" priority />
        </div>

        <div className="flex items-center gap-1 text-[#b5b5b5]">
          <span className="h-0.5 w-3 rounded-full bg-[#d4d4d4] animate-pulse" />
          <span className="h-0.5 w-3 rounded-full bg-[#a3a3a3]" />
          <span className="h-0.5 w-3 rounded-full bg-[#d4d4d4] animate-pulse" />
        </div>

        <div className="flex size-14 items-center justify-center rounded-2xl border border-[#ebebeb] bg-[#fafafa] shadow-xs">
          <ProviderMark className={`size-7 shrink-0 ${markClassName}`} />
        </div>
      </div>

      {/* Heading & Subheading */}
      <div className="text-center mt-4 mb-6">
        <div className="inline-flex items-center gap-1.5 rounded-full border border-[#e0e7ff] bg-[#f0f4ff] px-2.5 py-0.5 text-[11px] font-semibold text-[#3b5998] mb-2.5">
          <Info className="size-3" />
          <span>Authorization Preview</span>
        </div>
        <h1 className="text-2xl font-semibold tracking-tight text-[#121814]">
          Continue with {providerName}
        </h1>
        <p className="mt-2 text-sm text-[#5c5c5c] leading-relaxed">
          OAuth sign-in is not connected yet. When OAuth integration is
          available, RoleCue will hand off to {providerName} for authorization.
        </p>
      </div>

      {/* Transparent Preview Mode Callout */}
      <div className="rounded-xl border border-[#ebebeb] bg-[#fafafa] p-4 mb-5 text-left text-xs text-[#5c5c5c] leading-relaxed flex items-start gap-3">
        <Info className="size-4 text-[#494949] shrink-0 mt-0.5" />
        <div>
          <span className="font-semibold text-[#121814] block mb-1">
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
          className="rounded-xl border border-[#bfdbfe] bg-[#eff6ff] p-3.5 mb-5 text-xs text-[#1e40af] leading-relaxed animate-in fade-in slide-in-from-top-1 duration-200"
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
          className="h-11 w-full rounded-xl bg-[#121814] px-4 text-sm font-medium text-white shadow-sm transition hover:bg-[#252f28] active:scale-[0.99] focus-visible:ring-[#202020]/35 sm:h-12"
        >
          Preview {providerName} handoff
        </Button>

        <Button
          asChild
          variant="outline"
          className="h-11 w-full rounded-xl border-[#dbdbdb] bg-white px-4 text-sm font-medium text-[#494949] shadow-2xs transition hover:bg-[#f5f5f5] active:scale-[0.99] focus-visible:ring-[#202020]/35"
        >
          <Link href={routes.login}>Cancel</Link>
        </Button>
      </div>
    </div>
  );
}
