import type { useTranslations } from "next-intl";

export type PreflightTranslator = ReturnType<
  typeof useTranslations<"candidate.setup.preflight">
>;

// The microphone controller (features/interview/audio, owned by the room) reports failures as
// English sentences. The preflight shows them before the room, so it recognises each known
// sentence and shows its translation; an unknown one is shown as received. The room's files stay
// untouched, and a test pins every sentence below to the controller's source.
export const microphoneMessageKeys = {
  "Microphone access is not allowed. Check this site's browser permission, your system privacy settings or site policy, then retry.":
    "permissionDenied",
  "This browser could not capture microphone audio. Try a supported browser and check your device settings.":
    "unsupportedBrowser",
  "No microphone was found. Connect a microphone or choose another input, then retry.":
    "noDevice",
  "The microphone could not be opened. Check the connection and close other apps using it, then retry.":
    "deviceUnavailable",
  "The selected microphone is unavailable. Choose another input and retry.":
    "constraints",
  "The microphone check could not finish. Check your device and retry.":
    "unknown",
  "Microphone access requires a supported browser on HTTPS or localhost.":
    "secureContext",
  "The microphone disconnected. Reconnect it or choose another input, then retry.":
    "deviceLost",
  "The microphone list could not be refreshed. Your current input may still work; refresh inputs to try again.":
    "deviceListFailed",
  "Input level and voice activity are unavailable. Retry the audio check to enable them.":
    "analysisUnavailable",
} as const;

function isKnown(
  message: string,
): message is keyof typeof microphoneMessageKeys {
  return Object.hasOwn(microphoneMessageKeys, message);
}

export function microphoneMessage(message: string, t: PreflightTranslator) {
  return isKnown(message)
    ? t(`message.${microphoneMessageKeys[message]}`)
    : message;
}
