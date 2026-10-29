import { isLocale, localeCookieMaxAge, localeCookieName } from "./config";

// Saves the visitor's language choice for a year. A plain cookie is enough: it holds no secret,
// the server reads it on the next request, and nothing about it touches the backend.
export function persistLocale(locale: string) {
  if (!isLocale(locale)) return;
  const secure = window.location.protocol === "https:" ? "; Secure" : "";
  document.cookie = `${localeCookieName}=${locale}; Path=/; Max-Age=${localeCookieMaxAge}; SameSite=Lax${secure}`;
}
