import { localeCookieName, type AppLocale } from "./config";
import { resolveLocale } from "./resolve-locale";

function readCookie(name: string) {
  const prefix = `${name}=`;
  const entry = document.cookie
    .split("; ")
    .find((part) => part.startsWith(prefix));
  return entry ? decodeURIComponent(entry.slice(prefix.length)) : undefined;
}

// Same order as the server (saved cookie, browser languages, English) for code that renders
// outside the provider tree, such as the global error boundary.
export function readBrowserLocale(): AppLocale {
  return resolveLocale({
    cookie: readCookie(localeCookieName),
    acceptLanguage: navigator.languages.join(","),
  });
}
