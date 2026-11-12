import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { RoleCueMark } from "@/components/brand/rolecue-mark";
import { routes } from "@/config/routes";

export async function PublicPageShell({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = await getTranslations("public.pages");
  const common = await getTranslations("common");
  const links = [
    [routes.pricing, t("pricing")],
    [routes.privacy, t("privacy")],
    [routes.terms, t("terms")],
  ] as const;
  return (
    <div
      data-rolecue-theme="light"
      className="min-h-screen bg-[#fafaf8] text-[#191b1e]"
    >
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:bg-black focus:p-3 focus:text-white"
      >
        {common("skipToContent")}
      </a>
      <header className="border-b border-black/10 bg-[#fafaf8]">
        <div className="mx-auto flex min-h-20 max-w-7xl flex-wrap items-center justify-between gap-3 px-5 py-3 sm:px-8">
          <Link
            href={routes.home}
            className="flex min-h-11 items-center gap-2 text-xl font-bold tracking-tight"
          >
            <RoleCueMark alt="" className="size-9 object-contain" />
            RoleCue
          </Link>
          <nav
            aria-label={t("navigation")}
            className="flex flex-wrap items-center gap-x-4 text-sm font-medium sm:gap-x-7"
          >
            {links.map(([href, label]) => (
              <Link
                key={href}
                href={href}
                className="inline-flex min-h-11 items-center transition-colors hover:text-blue-700 focus-visible:outline-2 focus-visible:outline-offset-4 motion-reduce:transition-none"
              >
                {label}
              </Link>
            ))}
            <LanguageSwitcher />
          </nav>
        </div>
      </header>
      <main id="main-content">{children}</main>
      <footer className="border-t border-black/10">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-5 py-8 text-sm text-neutral-600 sm:px-8">
          <span>© 2026 RoleCue</span>
          <nav aria-label={t("navigation")} className="flex flex-wrap gap-x-6">
            {links.map(([href, label]) => (
              <Link
                key={href}
                className="inline-flex min-h-11 items-center hover:text-blue-700 focus-visible:outline-2 focus-visible:outline-offset-4"
                href={href}
              >
                {label}
              </Link>
            ))}
          </nav>
        </div>
      </footer>
    </div>
  );
}
