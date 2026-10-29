import type { AppLocale } from "./config";
import type { Messages } from "./catalog";

// Makes locale and message keys typed across useTranslations/getTranslations, so a typo fails typecheck.
declare module "next-intl" {
  interface AppConfig {
    Locale: AppLocale;
    Messages: Messages;
  }
}
