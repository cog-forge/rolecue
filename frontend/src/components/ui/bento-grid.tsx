import type { ComponentPropsWithoutRef } from "react";
import { cn } from "@/lib/utils";

// Magic UI Bento Grid: https://magicui.design/docs/components/bento-grid
// Content controls row height; cards stack before the workspace has room for columns.
export function BentoGrid({
  className,
  ...props
}: ComponentPropsWithoutRef<"div">) {
  return (
    <div
      className={cn(
        "grid w-full grid-cols-1 auto-rows-auto gap-4 md:grid-cols-6 xl:grid-cols-12",
        className,
      )}
      {...props}
    />
  );
}
