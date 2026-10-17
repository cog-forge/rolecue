"use client";

import Image from "next/image";
import { useState } from "react";
import { ExternalLink, Globe } from "lucide-react";
import {
  isHttpsProfileUrl,
  normalizeHttpsProfileUrlInput,
} from "@/features/profile/schemas/profile-schema";

export function WebsitePreview({ website }: { website: string }) {
  const [failedImage, setFailedImage] = useState<string | null>(null);
  const value = normalizeHttpsProfileUrlInput(website);
  const url = isHttpsProfileUrl(value) ? new URL(value) : null;
  const icon = url ? `${url.origin}/favicon.ico` : null;

  return (
    <aside
      aria-label="Company website preview"
      className="rounded-2xl border border-[#e8e3e7] bg-white/65 p-4 dark:border-[#3c4962] dark:bg-[#202b40]"
    >
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold text-[#59657d] dark:text-[#aab7cf]">
        <Globe className="size-4" aria-hidden />
        Website preview
      </div>
      {url ? (
        <a
          href={url.href}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-3 rounded-lg focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
        >
          {icon && failedImage !== icon ? (
            <Image
              unoptimized
              width={40}
              height={40}
              src={icon}
              alt=""
              referrerPolicy="no-referrer"
              className="size-10 shrink-0 rounded-lg bg-white object-contain dark:bg-[#2a354b]"
              onError={() => setFailedImage(icon)}
            />
          ) : (
            <Globe
              className="size-10 shrink-0 rounded-lg bg-white p-2 text-[#748199] dark:bg-[#2a354b] dark:text-[#aab7cf]"
              aria-hidden
            />
          )}
          <div className="min-w-0 flex-1">
            <p className="wrap-anywhere text-sm font-semibold">
              {url.hostname}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              Open website in a new tab
            </p>
          </div>
          <ExternalLink
            className="size-4 shrink-0 text-[#748199] dark:text-[#aab7cf]"
            aria-hidden
          />
        </a>
      ) : (
        <p className="text-sm leading-relaxed text-muted-foreground">
          Enter your company website to see it here.
        </p>
      )}
    </aside>
  );
}
