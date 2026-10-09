"use client";

import { motion, useReducedMotion } from "motion/react";
import { cn } from "@/lib/utils";

/** The landing's shared drafting grid and sparse construction marks. */
export function MarketingBackdrop({
  layout = "about",
}: {
  layout?: "about" | "practice";
}) {
  const reducedMotion = useReducedMotion();
  const practice = layout === "practice";
  const paths = practice
    ? [
        "M132 270 156 246 180 270 156 294Z",
        "M1016 196h24m-12-12v24",
        "M144 676v28h28 M1016 740h28v-28",
      ]
    : [
        "M96 150 120 126 144 150 120 174Z",
        "M487 214h24m-12-12v24",
        "M88 505v28h28 M488 552h28v-28",
      ];

  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute inset-0 overflow-hidden"
    >
      <div
        className={cn(
          "absolute inset-0 bg-[linear-gradient(rgb(47_55_70/8%)_1px,transparent_1px),linear-gradient(90deg,rgb(47_55_70/8%)_1px,transparent_1px)] bg-size-[3.5rem_3.5rem]",
          practice
            ? "mask-[radial-gradient(ellipse_at_50%_58%,black,transparent_78%)]"
            : "mask-[radial-gradient(ellipse_at_78%_45%,black,transparent_72%)] max-lg:mask-[radial-gradient(ellipse_at_65%_75%,black,transparent_65%)]",
        )}
      />
      <motion.svg
        className={cn(
          "absolute inset-y-0 text-(--rolecue-brand-blue)",
          practice
            ? "inset-x-0 h-full w-full"
            : "right-[3%] h-full w-[49%] max-lg:top-auto max-lg:bottom-12 max-lg:h-[38rem] max-lg:w-[92%]",
        )}
        fill="none"
        viewBox={practice ? "0 0 1200 900" : "0 0 600 720"}
        initial="rest"
        whileInView="visible"
        viewport={{ once: true, amount: 0.2 }}
      >
        {paths.map((path, index) => (
          <motion.path
            d={path}
            key={path}
            stroke="currentColor"
            strokeOpacity={0.2}
            strokeWidth={1}
            variants={{
              rest: { pathLength: reducedMotion ? 1 : 0 },
              visible: { pathLength: 1 },
            }}
            transition={{
              duration: reducedMotion ? 0 : 1.4,
              delay: reducedMotion ? 0 : 0.2 + index * 0.15,
            }}
          />
        ))}
        <circle
          cx={practice ? 1060 : 478}
          cy={practice ? 590 : 470}
          r="22"
          stroke="currentColor"
          strokeOpacity="0.12"
        />
        <circle
          cx={practice ? 1060 : 478}
          cy={practice ? 590 : 470}
          r="3"
          fill="currentColor"
          fillOpacity="0.16"
        />
        <path
          d={practice ? "M132 488h12m-6-6v12" : "M70 318h12m-6-6v12"}
          stroke="currentColor"
          strokeOpacity="0.2"
        />
      </motion.svg>
    </div>
  );
}
