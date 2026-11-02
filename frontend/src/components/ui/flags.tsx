import { useId } from "react";
import { cn } from "@/lib/utils";

// Flags are drawn as inline SVG: emoji flags render as "VN"/"GB" letters on Windows, and a flag
// library would be a new dependency. Every flag fills the same 3:2 box, so they line up in a
// menu or a header.
export type FlagCode = "vn" | "gb";

const svgProps = {
  className: "size-full",
  focusable: false,
  "aria-hidden": true,
} as const;

function VietnamFlag() {
  return (
    <svg viewBox="0 0 30 20" {...svgProps}>
      <rect width="30" height="20" fill="#da251d" />
      <polygon
        fill="#ffff00"
        points="15,4.6 16.35,8.75 20.71,8.75 17.18,11.31 18.53,15.45 15,12.89 11.47,15.45 12.82,11.31 9.29,8.75 13.65,8.75"
      />
    </svg>
  );
}

// The Union Jack is drawn at its own 2:1 ratio and stretched to 3:2, as flag icon sets do.
function UnitedKingdomFlag() {
  const clipId = useId();
  return (
    <svg viewBox="0 0 60 30" preserveAspectRatio="none" {...svgProps}>
      <clipPath id={clipId}>
        <path d="M30,15h30v15zv15h-30zh-30v-15zv-15h30z" />
      </clipPath>
      <rect width="60" height="30" fill="#012169" />
      <path d="M0,0 60,30M60,0 0,30" stroke="#fff" strokeWidth="6" />
      <path
        d="M0,0 60,30M60,0 0,30"
        clipPath={`url(#${clipId})`}
        stroke="#c8102e"
        strokeWidth="4"
      />
      <path d="M30,0v30M0,15h60" stroke="#fff" strokeWidth="10" />
      <path d="M30,0v30M0,15h60" stroke="#c8102e" strokeWidth="6" />
    </svg>
  );
}

// Decorative (the SVGs are aria-hidden): the language name or the control's label always sits next to it. The hairline ring
// follows the foreground colour so the white areas stay visible on light and dark surfaces.
export function Flag({
  code,
  className,
}: {
  code: FlagCode;
  className?: string;
}) {
  return (
    <span
      data-flag={code}
      className={cn(
        "relative inline-block h-4 w-6 shrink-0 overflow-hidden rounded-[3px] after:absolute after:inset-0 after:rounded-[inherit] after:ring-1 after:ring-foreground/25 after:ring-inset after:content-['']",
        className,
      )}
    >
      {code === "vn" ? <VietnamFlag /> : <UnitedKingdomFlag />}
    </span>
  );
}
