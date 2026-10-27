import { Camera, Check, ScanFace, Sun, UserRound } from "lucide-react";
import { cn } from "@/lib/utils";

const steps = [
  { title: "Details", description: "Name & personality" },
  { title: "Create", description: "Photos & appearance" },
  { title: "Save", description: "Your personal avatar" },
];

export function StudioJourney({
  current,
  complete,
}: {
  current: number;
  complete: boolean;
}) {
  return (
    <ol
      aria-label="Avatar creation progress"
      className="grid grid-cols-3 border-y py-5 sm:py-6"
    >
      {steps.map((step, index) => {
        const done = complete || index < current;
        return (
          <li
            key={step.title}
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
                {step.title}
                <span className="sr-only">
                  {done
                    ? ", complete"
                    : current === index
                      ? ", current step"
                      : ", upcoming"}
                </span>
              </p>
              <p className="mt-0.5 hidden text-xs text-muted-foreground sm:block">
                {step.description}
              </p>
            </div>
          </li>
        );
      })}
    </ol>
  );
}

const photoTips = [
  {
    icon: Sun,
    title: "Find soft, even light",
    detail: "Face a window. Avoid strong shadows or light behind you.",
  },
  {
    icon: ScanFace,
    title: "Keep your face clear",
    detail: "Use a neutral expression and keep hair away from your face.",
  },
  {
    icon: Camera,
    title: "Follow the photo prompts",
    detail: "Avaturn guides you through capture or uploading photos.",
  },
];

export function StudioPhotoGuide() {
  return (
    <aside
      aria-label="Photo preparation guide"
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
        A little preparation helps
      </p>
      <h2 className="mt-2 text-xl font-semibold tracking-tight">
        A good photo makes it yours.
      </h2>
      <ul className="mt-6 space-y-5">
        {photoTips.map(({ icon: Icon, title, detail }) => (
          <li key={title} className="flex gap-3">
            <Icon
              aria-hidden
              className="mt-0.5 size-5 shrink-0 text-muted-foreground"
              strokeWidth={1.5}
            />
            <div>
              <p className="text-sm font-medium">{title}</p>
              <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                {detail}
              </p>
            </div>
          </li>
        ))}
      </ul>
      <p className="mt-6 border-t pt-4 text-xs leading-relaxed text-muted-foreground">
        You’ll sign in to Avaturn separately. Your photos are captured or
        uploaded inside Avaturn.
      </p>
    </aside>
  );
}
