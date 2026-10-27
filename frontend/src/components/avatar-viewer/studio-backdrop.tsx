import Image from "next/image";
import { cn } from "@/lib/utils";

// Shared by the transparent portrait renders and the interactive WebGL canvas.
export function StudioBackdrop({
  className,
  portrait = false,
}: {
  className?: string;
  portrait?: boolean;
}) {
  return (
    <div
      aria-hidden
      className={cn(
        "pointer-events-none absolute inset-0 bg-studio-wall",
        className,
      )}
    >
      <picture className="absolute inset-0">
        {!portrait && (
          <source
            media="(max-width: 639px)"
            srcSet="/images/avatar-studio/warm-stage-portrait.webp"
          />
        )}
        <Image
          src={
            portrait
              ? "/images/avatar-studio/warm-stage-portrait.webp"
              : "/images/avatar-studio/warm-stage.webp"
          }
          alt=""
          fill
          sizes={
            portrait
              ? "(min-width: 1024px) 280px, (min-width: 640px) 33vw, 50vw"
              : "(min-width: 640px) 480px, 100vw"
          }
          className="object-cover object-center"
        />
      </picture>
    </div>
  );
}
