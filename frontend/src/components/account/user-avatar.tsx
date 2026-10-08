"use client";

import Image from "next/image";
import { useState } from "react";
import { Peek } from "@doan-labs/peek";
import { cn } from "@/lib/utils";

export function UserAvatar({
  id,
  name,
  image,
  className,
}: {
  id?: string;
  name: string;
  image?: string | null;
  className?: string;
}) {
  const [failed, setFailed] = useState<string | null>(null);
  let source: string | null = null;
  if (image) {
    try {
      const url = new URL(image);
      if (
        ["https:", "http:"].includes(url.protocol) &&
        url.hostname &&
        !url.username &&
        !url.password
      )
        source = image;
    } catch {
      /* Invalid legacy image falls back to a generated avatar. */
    }
  }
  return (
    <span
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
