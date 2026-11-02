import { useCallback } from "react";
import { useTranslations } from "next-intl";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Voice } from "../schemas/voice-schema";

// "2 interviews and 1 job posting" in the active language, or "" for a voice nothing uses.
export function useVoiceUsage() {
  const t = useTranslations("admin.voices.usage");
  const fmt = useFormatters();
  return useCallback(
    (voice: Voice | null) => {
      if (!voice) return "";
      const parts: string[] = [];
      if (voice.interview_count > 0)
        parts.push(t("interviews", { count: voice.interview_count }));
      if (voice.job_posting_count > 0)
        parts.push(t("jobPostings", { count: voice.job_posting_count }));
      return fmt.list(parts);
    },
    [t, fmt],
  );
}

// The provider sends the gender as free text; the two known values are translated and anything
// else is shown as stored.
export function useGenderLabel() {
  const t = useTranslations("admin.voices.gender");
  return useCallback(
    (gender: string) => {
      switch (gender.trim().toLowerCase()) {
        case "female":
          return t("female");
        case "male":
          return t("male");
        default:
          return gender;
      }
    },
    [t],
  );
}
