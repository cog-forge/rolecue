/** Recorder types in order of preference. VP8 is the cheapest encoder in every Chromium and Firefox;
 * Safari falls back to MP4. The upload declares only the container (`uploadContentType`). */
export const RECORDING_MIME_TYPES = [
  "video/webm;codecs=vp8,opus",
  "video/webm;codecs=vp9,opus",
  "video/webm",
  "video/mp4;codecs=avc1,mp4a.40.2",
  "video/mp4",
] as const;

/** ~550 kbit/s in total: about 4 MB a minute, so a 40-minute interview is about 165 MB and a whole
 * hour about 250 MB, well under the 500 MB the upload accepts. */
export const RECORDING_VIDEO_BITS_PER_SECOND = 500_000;
export const RECORDING_AUDIO_BITS_PER_SECOND = 48_000;
export const RECORDING_VIDEO_CONSTRAINTS = {
  facingMode: "user",
  width: { ideal: 640 },
  height: { ideal: 480 },
  frameRate: { ideal: 24, max: 30 },
} as const satisfies MediaTrackConstraints;

/** Each slice is saved as it arrives, so a crash or reload loses at most this much. */
export const RECORDING_TIMESLICE_MS = 5_000;

/** The upload limit of `POST /interviews/:id/recording`. */
export const RECORDING_MAX_BYTES = 524_288_000;
/** Capture stops a little before the limit, so the merged file still fits. */
export const RECORDING_CAPTURE_LIMIT_BYTES = 500_000_000;

export function pickRecordingMimeType(
  isTypeSupported: (type: string) => boolean,
) {
  return RECORDING_MIME_TYPES.find((type) => isTypeSupported(type)) ?? null;
}

/** The media type the upload grant accepts for a recorder type. */
export function uploadContentType(
  mimeType: string,
): "video/webm" | "video/mp4" {
  return mimeType.startsWith("video/mp4") ? "video/mp4" : "video/webm";
}
