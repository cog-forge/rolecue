export function normalizeRoleText(raw: string) {
  const lines = raw
    .replace(/\r\n?/g, "\n")
    .split("\n")
    .map((line) => line.trim().replace(/[^\S\n]+/gu, " "));
  return lines
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}
