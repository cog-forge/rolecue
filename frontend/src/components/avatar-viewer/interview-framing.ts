/** Static upper-body framing. No animation or seated-skeleton contract is assumed. */
export function interviewFraming(
  height: number,
  floor: number,
  width: number,
  viewportHeight: number,
  close: boolean,
  horizontalOffset = 0,
) {
  const mobile = width < 768;
  return {
    targetY: floor + height * (mobile ? 0.84 : 0.78),
    eyeY: floor + height * (mobile ? 0.86 : 0.84),
    distance: height * (mobile ? (close ? 1.35 : 1.65) : close ? 1.02 : 1.16),
    offsetX: mobile ? 0 : -Math.min(240, width * horizontalOffset),
    offsetY: mobile ? viewportHeight * 0.17 : 0,
  };
}
