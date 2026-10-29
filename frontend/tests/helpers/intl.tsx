import type { ReactElement, ReactNode } from "react";
import { render, type RenderOptions } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { AppLocale } from "@/lib/i18n/config";
import { mergeMessages } from "@/lib/i18n/fallback";
import en from "@/lib/i18n/messages/en";
import vi from "@/lib/i18n/messages/vi";

const catalogs = { en, vi };

// Same fallback rule as production: the active catalog layered over English.
export function messagesFor(locale: AppLocale) {
  return mergeMessages(en, catalogs[locale]);
}

export function IntlWrapper({
  locale = "en",
  children,
}: {
  locale?: AppLocale;
  children: ReactNode;
}) {
  return (
    <NextIntlClientProvider locale={locale} messages={messagesFor(locale)}>
      {children}
    </NextIntlClientProvider>
  );
}

export function renderWithIntl(
  ui: ReactElement,
  {
    locale = "en",
    ...options
  }: { locale?: AppLocale } & Omit<RenderOptions, "wrapper"> = {},
) {
  return render(ui, {
    ...options,
    wrapper: ({ children }) => (
      <IntlWrapper locale={locale}>{children}</IntlWrapper>
    ),
  });
}
