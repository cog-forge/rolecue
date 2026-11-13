"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { routes } from "@/config/routes";

export function NotFoundView() {
  const t = useTranslations("common.notFound");
  const [query, setQuery] = useState("");
  const pages = [
    { label: t("home"), href: routes.home },
    { label: t("jobs"), href: routes.jobs },
    { label: t("pricing"), href: routes.pricing },
    { label: t("login"), href: routes.login },
  ];
  const matches = pages.filter(({ label }) =>
    label.toLowerCase().includes(query.trim().toLowerCase()),
  );
  return (
    <main className="flex min-h-svh w-full flex-col items-center justify-center gap-8 bg-background px-6 py-12 text-center text-foreground">
      <div className="flex flex-col items-center gap-2">
        <span className="text-6xl font-bold tracking-tight tabular-nums sm:text-7xl">
          404
        </span>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
          {t("title")}
        </h1>
        <p className="max-w-md text-sm text-muted-foreground">
          {t("description")}
        </p>
      </div>
      <form
        onSubmit={(event) => event.preventDefault()}
        className="flex w-full max-w-md flex-col gap-2 sm:flex-row"
      >
        <Input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("searchPlaceholder")}
          aria-label={t("searchPlaceholder")}
          className="h-10 flex-1 text-sm"
        />
        <Button type="submit" className="h-10 w-full sm:w-auto">
          <Search aria-hidden="true" />
          {t("search")}
        </Button>
      </form>
      <div className="flex w-full max-w-md flex-col gap-3">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {t("popularLinks")}
        </p>
        <ul className="flex flex-col border-t border-border text-left">
          {matches.map((page) => (
            <li key={page.href} className="border-b border-border">
              <Link
                href={page.href}
                className="group flex min-h-11 items-center justify-between gap-4 py-2.5 text-sm"
              >
                <span>{page.label}</span>
                <ArrowRight
                  className="size-4 text-muted-foreground transition-colors group-hover:text-foreground"
                  aria-hidden="true"
                />
              </Link>
            </li>
          ))}
        </ul>
        {matches.length === 0 && (
          <p role="status" className="text-sm text-muted-foreground">
            {t("noResults")}
          </p>
        )}
      </div>
    </main>
  );
}
