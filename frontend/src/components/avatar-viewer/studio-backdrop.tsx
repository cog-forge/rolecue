import Image from "next/image";
import { cn } from "@/lib/utils";

// Shared by the transparent portrait renders and the interactive WebGL canvas.
export function StudioBackdrop({
  className,
  portrait = false,
  immersive = false,
  src,
}: {
  className?: string;
  portrait?: boolean;
  immersive?: boolean;
  src?: string;
}) {
  return (
    <div
      data-slot="studio-backdrop"
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-0 bg-studio-wall",
        className,
      )}
    >
      <picture className="absolute inset-0">
        {!portrait && !immersive && !src && (
          <source
            media="(max-width: 639px)"
            srcSet="/images/avatar-studio/warm-stage-portrait.webp"
          />
        )}
        <Image
          src={
            src ??
            (portrait
              ? "/images/avatar-studio/warm-stage-portrait.webp"
              : "/images/avatar-studio/warm-stage.webp")
          }
          alt=""
          fill
          sizes={
            immersive
              ? "100vw"
              : portrait
                ? "(min-width: 1024px) 280px, (min-width: 640px) 33vw, 50vw"
                : "(min-width: 640px) 480px, 100vw"
          }
          className="object-cover object-center"
        />
      </picture>
    </div>
  );
}
