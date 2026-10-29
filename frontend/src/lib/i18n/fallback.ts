type Tree = { [key: string]: string | Tree };

function isTree(value: unknown): value is Tree {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// Layers a (possibly partial) catalog over the English base so a missing key shows English.
export function mergeMessages<T extends Tree>(base: T, override: Tree): T {
  const merged: Tree = { ...base };
  for (const [key, value] of Object.entries(override)) {
    const baseValue = merged[key];
    merged[key] =
      isTree(baseValue) && isTree(value)
        ? mergeMessages(baseValue, value)
        : typeof value === "string" || isTree(value)
          ? value
          : (baseValue ?? "");
  }
  return merged as T;
}

// Dotted paths that exist in the base catalog but not in the translation.
export function findMissingKeys(
  base: Tree,
  translation: Tree,
  prefix = "",
): string[] {
  return Object.entries(base).flatMap(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key;
    const translated = translation[key];
    if (isTree(value))
      return findMissingKeys(value, isTree(translated) ? translated : {}, path);
    return typeof translated === "string" ? [] : [path];
  });
}
