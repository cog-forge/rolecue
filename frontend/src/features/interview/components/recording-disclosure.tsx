"use client";
import { useTranslations } from "next-intl";

/** What a recruitment recording holds, who sees it, why, and how long it is kept (KAN-87). */
export function RecordingDisclosure({ className }: { className?: string }) {
  const t = useTranslations("candidate.interviewRoom.recording.disclosure");
  return (
    <section aria-label={t("title")} className={className}>
      <h3 className="font-medium">{t("title")}</h3>
      <ul className="mt-1.5 list-disc space-y-1 pl-5 opacity-85">
        <li>{t("what")}</li>
        <li>{t("who")}</li>
        <li>{t("why")}</li>
        <li>{t("retention")}</li>
      </ul>
    </section>
  );
}
