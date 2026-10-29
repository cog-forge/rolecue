"use client";

import { Globe } from "lucide-react";
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
import { isLocale, localeNames, locales } from "@/lib/i18n/config";
import { persistLocale } from "@/lib/i18n/persist-locale";
import { cn } from "@/lib/utils";

// Each language is listed in its own name. Choosing one saves a cookie and refreshes the server
// output in place, so the page, scroll position and any form input stay where they are.
export function LanguageSwitcher({
  className,
  showName = true,
}: {
  className?: string;
  // Icon-only on tight headers; the accessible name still carries the current language.
  showName?: boolean;
}) {
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
          className={cn("h-11 gap-2 px-3 text-sm", className)}
        >
          <Globe className="size-4" aria-hidden />
          {showName && (
            <span lang={locale}>
              {isLocale(locale) ? localeNames[locale] : locale}
            </span>
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="z-100 min-w-40">
        <DropdownMenuRadioGroup value={locale} onValueChange={choose}>
          {locales.map((option) => (
            <DropdownMenuRadioItem
              key={option}
              value={option}
              lang={option}
              className="min-h-10"
            >
              {localeNames[option]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
