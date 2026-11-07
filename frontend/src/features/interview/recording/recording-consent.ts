/**
 * The version of the recording consent text shown in the preflight (KAN-297). Change it together with the
 * text in the `consent` messages (en and vi); the API stores the version the Candidate accepted.
 */
export const CONSENT_VERSION = "2026-11-v1";

const keyFor = (userId: string) => `rolecue:recording-consent:${userId}`;

/**
 * The tick happens in the preflight, the recording is uploaded after the interview, so the accepted
 * version is kept in this browser, per account, until the upload asks the API for its grant. Storage can
 * be unavailable (private window, blocked data); then nothing is remembered and no version is sent.
 */
export function rememberRecordingConsent(userId: string) {
  try {
    window.localStorage.setItem(keyFor(userId), CONSENT_VERSION);
  } catch {
    // No storage: the upload sends no consent version.
  }
}

export function forgetRecordingConsent(userId: string) {
  try {
    window.localStorage.removeItem(keyFor(userId));
  } catch {
    // Nothing to forget.
  }
}

/** The version this account accepted in this browser, or undefined when none was. */
export function acceptedConsentVersion(userId: string): string | undefined {
  try {
    return window.localStorage.getItem(keyFor(userId)) ?? undefined;
  } catch {
    return undefined;
  }
}
