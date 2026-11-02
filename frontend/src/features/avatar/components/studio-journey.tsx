import { useTranslations } from "next-intl";
import { Camera, Check, ScanFace, Sun, UserRound } from "lucide-react";
import { cn } from "@/lib/utils";

const steps = ["details", "create", "save"] as const;

export function StudioJourney({
  current,
  complete,
}: {
  current: number;
  complete: boolean;
}) {
  const t = useTranslations("candidate.avatarStudio.journey");
  return (
    <ol
      aria-label={t("label")}
      className="grid grid-cols-3 border-y py-5 sm:py-6"
    >
      {steps.map((step, index) => {
        const done = complete || index < current;
        return (
          <li
            key={step}
            aria-current={!complete && current === index ? "step" : undefined}
            className={cn(
              "flex min-w-0 items-center gap-2 sm:gap-3",
              index > 0 && "border-l pl-3 sm:pl-6",
            )}
          >
            <span
              aria-hidden
              className={cn(
                "flex size-7 shrink-0 items-center justify-center rounded-full border text-xs font-medium transition-colors duration-(--duration-fast) motion-reduce:transition-none sm:size-9",
                done
                  ? "border-primary bg-primary text-primary-foreground"
                  : current === index
                    ? "border-rolecue-brand bg-rolecue-brand/10 text-foreground"
                    : "text-muted-foreground",
              )}
            >
              {done ? <Check className="size-4" /> : `0${index + 1}`}
            </span>
            <div className="min-w-0">
              <p
                className={cn(
                  "text-sm font-medium",
                  current < index && !complete && "text-muted-foreground",
                )}
              >
                {t(`${step}.title`)}
                <span className="sr-only">
                  {done
                    ? t("complete")
                    : current === index
                      ? t("current")
                      : t("upcoming")}
                </span>
              </p>
              <p className="mt-0.5 hidden text-xs text-muted-foreground sm:block">
                {t(`${step}.description`)}
              </p>
            </div>
          </li>
        );
      })}
    </ol>
  );
}

const photoTips = [
  { icon: Sun, key: "light" },
  { icon: ScanFace, key: "face" },
  { icon: Camera, key: "prompts" },
] as const;

export function StudioPhotoGuide() {
  const t = useTranslations("candidate.avatarStudio.photoGuide");
  return (
    <aside
      aria-label={t("label")}
      className="rounded-2xl bg-muted/60 p-6 sm:p-8"
    >
      <div
        aria-hidden
        className="relative mx-auto mb-6 flex size-32 items-center justify-center rounded-3xl border border-border bg-background/70"
      >
        <UserRound className="size-14 text-muted-foreground" strokeWidth={1} />
        <ScanFace
          className="absolute inset-0 size-full p-2 text-rolecue-brand/70"
          strokeWidth={0.6}
        />
      </div>
      <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">
        {t("eyebrow")}
      </p>
      <h2 className="mt-2 text-xl font-semibold tracking-tight">
        {t("title")}
      </h2>
      <ul className="mt-6 space-y-5">
        {photoTips.map(({ icon: Icon, key }) => (
          <li key={key} className="flex gap-3">
            <Icon
              aria-hidden
              className="mt-0.5 size-5 shrink-0 text-muted-foreground"
              strokeWidth={1.5}
            />
            <div>
              <p className="text-sm font-medium">{t(`${key}Title`)}</p>
              <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                {t(`${key}Detail`)}
              </p>
            </div>
          </li>
        ))}
      </ul>
      <p className="mt-6 border-t pt-4 text-xs leading-relaxed text-muted-foreground">
        {t("footer")}
      </p>
    </aside>
  );
}
