import { AudioLines, LoaderCircle, RotateCcw, Trash2 } from "lucide-react";
import { UserAvatar } from "@/components/account/user-avatar";
import { Button } from "@/components/ui/button";
import type { Voice } from "../schemas/voice-schema";
import { localeName } from "../utils/voice-format";
import { PanelReveal } from "./panel-reveal";
import { usage } from "./remove-voice-dialog";
import { VoiceStatus } from "./voice-status";

const none = <span className="text-muted-foreground">Not set</span>;

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
  if (!voice)
    return (
      <div className="flex flex-col items-center gap-2 py-10 text-center">
        <AudioLines className="size-6 text-muted-foreground" aria-hidden />
        <p className="font-medium">Select a voice</p>
        <p className="max-w-[28ch] text-sm text-muted-foreground">
          Its details and actions appear here.
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
        <Row label="Gender">{voice.gender ?? none}</Row>
        <Row label="Locale">
          {voice.locale ? (
            <>
              {localeName(voice.locale)}
              <span className="block text-xs text-muted-foreground">
                {voice.locale}
              </span>
            </>
          ) : (
            none
          )}
        </Row>
        <Row label="Provider">{voice.provider ?? none}</Row>
        <Row label="Provider voice ID">
          {voice.provider_voice_id ? (
            <span className="font-mono text-xs [overflow-wrap:anywhere]">
              {voice.provider_voice_id}
            </span>
          ) : (
            none
          )}
        </Row>
        <Row label="In use">
          {voice.in_use ? (
            usage(voice)
          ) : (
            <span className="text-muted-foreground">Not used</span>
          )}
        </Row>
      </dl>

      <p className="text-xs leading-relaxed text-muted-foreground">
        {voice.in_use
          ? "Interviews and postings that use this voice keep it, so it can only be deactivated."
          : "Nothing uses this voice, so removing it deletes it from the catalog."}
      </p>

      <div className="flex flex-wrap gap-2">
        {!voice.is_active && (
          <Button
            variant="outline"
            className="h-10 rounded-full px-5"
            aria-label={`Reactivate ${voice.name}`}
            disabled={activating}
            onClick={() => onActivate(voice)}
          >
            {activating ? (
              <LoaderCircle className="animate-spin" aria-hidden />
            ) : (
              <RotateCcw aria-hidden />
            )}
            {activating ? "Reactivating…" : "Reactivate"}
          </Button>
        )}
        {canRemove && (
          <Button
            variant="outline"
            className="h-10 rounded-full border-rose-200 bg-rose-50 px-5 text-rose-700 hover:bg-rose-100 hover:text-rose-800 dark:border-rose-400/20 dark:bg-rose-400/10 dark:text-rose-300 dark:hover:bg-rose-400/20 dark:hover:text-rose-200"
            aria-label={`${voice.in_use ? "Deactivate" : "Remove"} ${voice.name}`}
            onClick={() => onRemove(voice)}
          >
            <Trash2 aria-hidden />
            {voice.in_use ? "Deactivate" : "Remove"}
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
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0">{children}</dd>
    </div>
  );
}
