"use client";

import { useEffect, type RefObject } from "react";

export function useShakeInvalidFields(
  formRef: RefObject<HTMLFormElement | null>,
  submitCount: number,
) {
  useEffect(() => {
    if (submitCount === 0) return;

    const inputs = Array.from(
      formRef.current?.querySelectorAll<HTMLElement>(
        '.t-input[aria-invalid="true"]',
      ) ?? [],
    );
    if (inputs.length === 0) return;

    for (const input of inputs) {
      input.classList.remove("is-shaking");
      void input.offsetWidth;
      input.classList.add("is-shaking");
    }

    const styles = getComputedStyle(document.documentElement);
    const durationMs = (property: string, fallback: number) => {
      const value = styles.getPropertyValue(property).trim();
      const duration = Number.parseFloat(value);
      if (!Number.isFinite(duration)) return fallback;
      return value.endsWith("ms") ? duration : duration * 1000;
    };
    const shakeDuration =
      durationMs("--shake-dur-a", 80) * 2 + durationMs("--shake-dur-b", 60) * 2;
    const timer = window.setTimeout(
      () => inputs.forEach((input) => input.classList.remove("is-shaking")),
      shakeDuration + 20,
    );

    return () => window.clearTimeout(timer);
  }, [formRef, submitCount]);
}
