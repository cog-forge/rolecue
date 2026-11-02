import { AudioLines, LoaderCircle, RotateCcw, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { UserAvatar } from "@/components/account/user-avatar";
import { Button } from "@/components/ui/button";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { Voice } from "../schemas/voice-schema";
import { useGenderLabel, useVoiceUsage } from "../utils/voice-labels";
import { PanelReveal } from "./panel-reveal";
import { VoiceStatus } from "./voice-status";

// Everything about the selected voice, and the only place its actions live. Keyed by voice so the
// panel reveal plays again when the selection changes.
export function VoiceDetails({
  voice,
  activating,
  onActivate,
  onRemove,
}: {
  voice: Voice | null;
  activating: boolean;
  onActivate: (voice: Voice) => void;
  onRemove: (voice: Voice) => void;
}) {
  const format = useFormatters();
  const t = useTranslations("admin.voices.details");
  const tCommon = useTranslations("common");
  const genderLabel = useGenderLabel();
  const usage = useVoiceUsage();
  const none = (
    <span className="text-muted-foreground">{tCommon("labels.notSet")}</span>
  );
  if (!voice)
    return (
      <div className="flex flex-col items-center gap-2 py-10 text-center">
        <AudioLines className="size-6 text-muted-foreground" aria-hidden />
        <p className="font-medium">{t("emptyTitle")}</p>
        <p className="max-w-[28ch] text-sm text-muted-foreground">
          {t("emptyHint")}
        </p>
      </div>
    );
  // An inactive voice in use must stay for its references, so only reactivating is left.
  const canRemove = voice.is_active || !voice.in_use;
  return (
    <PanelReveal key={voice.id} className="space-y-5">
      <div className="flex items-center gap-3">
        <UserAvatar id={voice.id} name={voice.name} className="size-16" />
        <div className="min-w-0 space-y-1">
          <h2 className="text-lg leading-tight font-semibold [overflow-wrap:anywhere]">
            {voice.name}
          </h2>
          <VoiceStatus active={voice.is_active} />
        </div>
      </div>

      <dl className="divide-y rounded-lg border text-sm">
        <Row label={t("gender")}>
          {voice.gender ? genderLabel(voice.gender) : none}
        </Row>
        <Row label={t("locale")}>
          {voice.locale ? (
            <>
              {format.languageName(voice.locale)}
              <span className="block text-xs text-muted-foreground">
                {voice.locale}
              </span>
            </>
          ) : (
            none
          )}
        </Row>
        <Row label={t("provider")}>{voice.provider ?? none}</Row>
        <Row label={t("providerVoiceId")}>
          {voice.provider_voice_id ? (
            <span className="font-mono text-xs [overflow-wrap:anywhere]">
              {voice.provider_voice_id}
            </span>
          ) : (
            none
          )}
        </Row>
        <Row label={t("inUse")}>
          {voice.in_use ? (
            usage(voice)
          ) : (
            <span className="text-muted-foreground">{t("notUsed")}</span>
          )}
        </Row>
      </dl>

      <p className="text-xs leading-relaxed text-muted-foreground">
        {voice.in_use ? t("hintInUse") : t("hintUnused")}
      </p>

      <div className="flex flex-wrap gap-2">
        {!voice.is_active && (
          <Button
            variant="outline"
            className="h-10 rounded-full px-5"
            aria-label={t("reactivateLabel", { name: voice.name })}
            disabled={activating}
            onClick={() => onActivate(voice)}
          >
            {activating ? (
              <LoaderCircle className="animate-spin" aria-hidden />
            ) : (
              <RotateCcw aria-hidden />
            )}
            {activating ? t("reactivating") : t("reactivate")}
          </Button>
        )}
        {canRemove && (
          <Button
            variant="outline"
            className="h-10 rounded-full border-rose-200 bg-rose-50 px-5 text-rose-700 hover:bg-rose-100 hover:text-rose-800 dark:border-rose-400/20 dark:bg-rose-400/10 dark:text-rose-300 dark:hover:bg-rose-400/20 dark:hover:text-rose-200"
            aria-label={
              voice.in_use
                ? t("deactivateLabel", { name: voice.name })
                : t("removeLabel", { name: voice.name })
            }
            onClick={() => onRemove(voice)}
          >
            <Trash2 aria-hidden />
            {voice.in_use ? t("deactivate") : t("remove")}
          </Button>
        )}
      </div>
    </PanelReveal>
  );
}

function Row({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid grid-cols-[7.5rem_1fr] gap-3 px-3 py-2.5">
      <dt className="min-w-0 text-muted-foreground [overflow-wrap:anywhere]">
        {label}
      </dt>
      <dd className="min-w-0">{children}</dd>
    </div>
  );
}
