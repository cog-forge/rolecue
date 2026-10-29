import { defaultLocale, isLocale, locales, type AppLocale } from "./config";

type LanguageRange = { tag: string; quality: number };

// Parses an Accept-Language header into ranges ordered by quality, keeping header order on ties.
export function parseAcceptLanguage(
  header: string | null | undefined,
): LanguageRange[] {
  if (!header) return [];
  return header
    .split(",")
    .flatMap((part, index) => {
      const [rawTag, ...params] = part.trim().split(";");
      const tag = rawTag.trim().toLowerCase();
      if (!tag) return [];
      const q = params
        .map((param) => param.trim())
        .find((param) => param.startsWith("q="));
      const quality = q ? Number(q.slice(2)) : 1;
      if (!Number.isFinite(quality) || quality <= 0) return [];
      return [{ tag, quality, index }];
    })
    .sort((a, b) => b.quality - a.quality || a.index - b.index)
    .map(({ tag, quality }) => ({ tag, quality }));
}

// "vi-VN" and "vi" both match "vi"; "*" and unsupported tags match nothing.
export function matchLocale(tag: string): AppLocale | undefined {
  const primary = tag.toLowerCase().split("-")[0];
  return locales.find((locale) => locale === primary);
}

// Saved cookie choice first, then the browser's Accept-Language, then English.
export function resolveLocale({
  cookie,
  acceptLanguage,
}: {
  cookie?: string | null;
  acceptLanguage?: string | null;
}): AppLocale {
  if (isLocale(cookie)) return cookie;
  for (const { tag } of parseAcceptLanguage(acceptLanguage)) {
    const match = matchLocale(tag);
    if (match) return match;
  }
  return defaultLocale;
}
