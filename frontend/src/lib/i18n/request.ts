import { cookies, headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";
import { loadMessages } from "./catalog";
import { localeCookieName } from "./config";
import { resolveLocale } from "./resolve-locale";

// Resolved per request with no locale prefix in the URL, so existing routes and links are unchanged.
export default getRequestConfig(async () => {
  const [cookieStore, headerStore] = await Promise.all([cookies(), headers()]);
  const locale = resolveLocale({
    cookie: cookieStore.get(localeCookieName)?.value,
    acceptLanguage: headerStore.get("accept-language"),
  });
  return {
    locale,
    messages: await loadMessages(locale),
    // A missing key falls back to English above; anything still unknown renders its key path.
    onError(error) {
      if (process.env.NODE_ENV === "development") console.error(error);
    },
    getMessageFallback: ({ namespace, key }) =>
      namespace ? `${namespace}.${key}` : key,
  };
});
