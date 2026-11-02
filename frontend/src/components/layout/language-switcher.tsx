"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Flag } from "@/components/ui/flags";
import { isLocale, localeFlags, localeNames, locales } from "@/lib/i18n/config";
import { persistLocale } from "@/lib/i18n/persist-locale";
import { cn } from "@/lib/utils";

// The button shows the flag of the active language; the menu lists each language by flag and its
// own name. Choosing one saves a cookie and refreshes the server output in place, so the page,
// scroll position and any form input stay where they are.
export function LanguageSwitcher({ className }: { className?: string }) {
  const t = useTranslations("common.language");
  const locale = useLocale();
  const router = useRouter();
  const choose = (next: string) => {
    if (!isLocale(next) || next === locale) return;
    persistLocale(next);
    router.refresh();
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          aria-label={`${t("change")}: ${isLocale(locale) ? localeNames[locale] : locale}`}
          className={cn("h-11 min-w-11 px-3", className)}
        >
          {isLocale(locale) && <Flag code={localeFlags[locale]} />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="z-100 min-w-40">
        <DropdownMenuRadioGroup value={locale} onValueChange={choose}>
          {locales.map((option) => (
            <DropdownMenuRadioItem
              key={option}
              value={option}
              lang={option}
              className="min-h-10 gap-2.5"
            >
              <Flag code={localeFlags[option]} />
              {localeNames[option]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
