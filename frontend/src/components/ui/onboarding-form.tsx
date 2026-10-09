"use client";

import type { HTMLAttributes, ReactNode } from "react";
import Image from "next/image";
import { motion, useReducedMotion } from "framer-motion";
import { cn } from "@/lib/utils";

type OnboardingFormProps = HTMLAttributes<HTMLDivElement> & {
  imageSrc: string;
  imageAlt?: string;
  topBar?: ReactNode;
  progress?: ReactNode;
  heading: ReactNode;
  description: ReactNode;
  children: ReactNode;
};

/** Visual shell only; the feature owns form state, validation, and submission. */
export function OnboardingForm({
  imageSrc,
  imageAlt = "",
  topBar,
  progress,
  heading,
  description,
  children,
  className,
  ...props
}: OnboardingFormProps) {
  const reduceMotion = useReducedMotion();

  return (
    <div
      data-slot="onboarding-form"
      className={cn(
        "overflow-hidden rounded-[27px] border border-white/75 bg-[#fffaf8]/95 text-[#202438] dark:border-white/15 dark:bg-[#182033]/95 dark:text-[#eef2ff] shadow-[0_28px_100px_-30px_rgba(20,33,60,0.42)] backdrop-blur-2xl",
        className,
      )}
      {...props}
    >
      <div className="relative h-40 overflow-hidden bg-[#e5e7f5] sm:h-48">
        <Image
          src={imageSrc}
          alt={imageAlt}
          fill
          priority
          sizes="(max-width: 640px) 100vw, 640px"
          className="object-cover object-center dark:brightness-75"
        />
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-[#fffaf8] to-transparent dark:from-[#182033]" />
        {topBar && (
          <div className="absolute inset-x-4 top-4 z-10 flex items-center justify-end gap-3 sm:inset-x-6">
            {topBar}
          </div>
        )}
      </div>
      <motion.div
        initial={reduceMotion ? false : { opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.38, ease: "easeOut" }}
        className="mx-auto max-w-[600px] px-5 pb-6 sm:px-9 sm:pb-9"
      >
        {progress}
        <div className="mx-auto mb-6 max-w-[490px] space-y-2 text-center sm:mb-7">
          {heading}
          {description}
        </div>
        {children}
      </motion.div>
    </div>
  );
}
