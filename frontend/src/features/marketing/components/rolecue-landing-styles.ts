export const landingContainer =
  "relative mx-auto w-[min(85rem,calc(100%-clamp(2.5rem,9vw,8rem)))] max-[760px]:w-[calc(100%-2rem)]";

export const landingFocus =
  "focus-visible:outline focus-visible:outline-[3px] focus-visible:outline-offset-4 focus-visible:outline-(--rolecue-focus)";

export const eyebrow =
  "mb-4 font-mono text-(length:--rolecue-type-label) tracking-(--rolecue-tracking-label) text-(--rolecue-ink-muted) uppercase font-(--rolecue-weight-label)";

export const sectionTitle =
  "m-0 text-balance text-(length:--rolecue-type-section) leading-(--rolecue-leading-heading) tracking-(--rolecue-tracking-heading) text-(--rolecue-ink) font-(--rolecue-weight-heading)";

export const sectionIntro =
  "mt-[1.35rem] max-w-(--rolecue-width-copy) text-balance text-(length:--rolecue-type-body-lg) leading-(--rolecue-leading-copy) text-(--rolecue-ink-muted)";

const buttonBase =
  "inline-flex min-h-[3.2rem] items-center justify-center gap-[0.62rem] rounded-full border px-5 py-[0.78rem] text-[0.88rem] no-underline [font-variation-settings:'wght'_700] transition-[transform,border-color,color,background,box-shadow] duration-(--duration-quick) ease-(--ease-smooth-out) hover:-translate-y-[3px] motion-reduce:transform-none! motion-reduce:transition-none!";

export const primaryButton = `${buttonBase} border-transparent bg-(--rolecue-button-primary) text-(--rolecue-on-dark) shadow-(--rolecue-shadow-low) hover:bg-(--rolecue-button-primary-hover) hover:text-(--rolecue-on-dark) hover:shadow-(--rolecue-shadow-float)`;

export const secondaryButton = `${buttonBase} border-(--rolecue-border-strong) bg-(--rolecue-button-secondary) text-(--rolecue-ink) hover:border-[rgb(var(--rolecue-ink-rgb)/42%)] hover:bg-(--rolecue-surface) hover:shadow-(--rolecue-shadow-low)`;

export const actionIcon =
  "transition-transform duration-(--duration-quick) ease-(--ease-smooth-out) group-hover/button:translate-x-[0.13rem] group-hover/button:-translate-y-[0.1rem] motion-reduce:transform-none! motion-reduce:transition-none!";
