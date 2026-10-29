// Display helpers for the voice catalog.

// "en-US" -> "English (United States)". Falls back to the code when the runtime can't name it.
export function localeName(locale: string) {
  try {
    const name = new Intl.DisplayNames(["en"], {
      type: "language",
      languageDisplay: "standard",
    }).of(locale);
    return name && name !== locale ? name : locale;
  } catch {
    return locale;
  }
}
