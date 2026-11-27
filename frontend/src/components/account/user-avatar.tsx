"use client";

import Image from "next/image";
import { useState } from "react";
import { Peek } from "@doan-labs/peek";
import { getApiBaseUrl } from "@/lib/api/config";
import { cn } from "@/lib/utils";

export function UserAvatar({
  id,
  name,
  image,
  previewUrl,
  className,
}: {
  id?: string;
  name: string;
  image?: string | null;
  previewUrl?: string;
  className?: string;
}) {
  const [failed, setFailed] = useState<string | null>(null);
  let source = previewUrl ?? null;
  if (!source && image) {
    const apiBase = getApiBaseUrl();
    const managed = /^\/profile\/photos\/[a-f0-9-]{36}\/[a-f0-9-]{36}$/.test(
      image,
    );
    const candidate =
      managed && apiBase ? `${apiBase.replace(/\/$/, "")}${image}` : image;
    try {
      const url = new URL(candidate);
      if (
        ["https:", "http:"].includes(url.protocol) &&
        url.hostname &&
        !url.username &&
        !url.password
      )
        source = candidate;
    } catch {
      /* Invalid legacy image falls back to a generated avatar. */
    }
  }
  return (
    <span
      data-slot="user-avatar"
      aria-hidden
      className={cn(
        "relative grid size-9 shrink-0 place-items-center overflow-hidden rounded-xl bg-muted",
        className,
      )}
    >
      {source && failed !== source ? (
        <Image
          src={source}
          alt=""
          fill
          sizes="96px"
          unoptimized
          referrerPolicy="no-referrer"
          className="object-cover"
          onError={() => setFailed(source)}
        />
      ) : (
        <Peek
          name={id || name.trim() || "RoleCue"}
          version={1}
          expression="happy"
          animate
          gaze="pointer"
          frame="none"
          title={false}
          className="size-full"
        />
      )}
    </span>
  );
}
