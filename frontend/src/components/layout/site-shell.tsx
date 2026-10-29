import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { site } from "@/config/site";
import { routes } from "@/config/routes";
import { CurtainThemeToggle } from "@/components/ui/curtain-theme-toggle";
import { LanguageSwitcher } from "./language-switcher";
export async function SiteShell({
  children,
  navigation,
  label,
}: {
  children: React.ReactNode;
  navigation: readonly { label: string; href: string }[];
  label: string;
}) {
  const common = await getTranslations("common");
  return (
    <>
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:block focus:p-4"
      >
        {common("skipToContent")}
      </a>
      <header className="border-b">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-6 px-6 py-5">
          <Link href={routes.home} className="font-semibold">
            {site.name}
          </Link>
          <span className="text-sm text-muted-foreground">{label}</span>
          <nav aria-label={label} className="flex flex-wrap gap-4">
            {navigation.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="rounded-sm text-sm underline-offset-4 hover:underline focus-visible:outline-2"
              >
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="ml-auto flex shrink-0 items-center gap-2">
            <LanguageSwitcher />
            <CurtainThemeToggle />
          </div>
        </div>
      </header>
      <main
        id="main-content"
        className="mx-auto max-w-6xl space-y-8 px-6 py-12"
      >
        {children}
      </main>
    </>
  );
}
