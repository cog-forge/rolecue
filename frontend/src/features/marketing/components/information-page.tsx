import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { ArrowRight } from "lucide-react";
import { routes } from "@/config/routes";
import { PublicPageShell } from "./public-page-shell";

export async function InformationPage({
  kind,
}: {
  kind: "pricing" | "privacy" | "terms";
}) {
  const t = await getTranslations("public.pages");
  const sectionIds =
    kind === "pricing"
      ? (["practice", "recruitment", "avatar"] as const)
      : kind === "privacy"
        ? ([
            "information",
            "practiceData",
            "recruitmentData",
            "access",
          ] as const)
        : (["account", "coins", "sessions", "responsibility"] as const);
  return (
    <PublicPageShell>
      <div className="mx-auto max-w-7xl px-5 sm:px-8">
        <section className="grid gap-8 border-b border-black/10 py-16 sm:py-24 lg:grid-cols-[1.2fr_0.8fr] lg:gap-20 lg:py-32">
          <div>
            <p className="mb-6 text-xs font-semibold tracking-[0.16em] text-blue-700 uppercase">
              RoleCue / {t(kind)}
            </p>
            <h1 className="max-w-3xl text-5xl leading-[1.04] font-semibold tracking-[-0.055em] sm:text-7xl">
              {t(`${kind}Title`)}
            </h1>
          </div>
          <div className="self-end">
            <p className="max-w-xl text-lg leading-8 text-neutral-600">
              {t(`${kind}Intro`)}
            </p>
            {kind !== "pricing" && (
              <p className="mt-6 border-l-2 border-blue-700 pl-4 text-sm leading-6 text-neutral-600">
                {t("draftNotice")}
              </p>
            )}
          </div>
        </section>
        {kind === "pricing" && (
          <section className="border-b border-black/10 py-16 sm:py-20">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <p className="text-xs font-semibold tracking-[0.16em] text-blue-700 uppercase">
                  01 / {t("catalogEyebrow")}
                </p>
                <h2 className="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">
                  {t("catalogTitle")}
                </h2>
              </div>
              <p className="max-w-md text-base leading-7 text-neutral-600">
                {t("catalogIntro")}
              </p>
            </div>
            <p className="py-8 text-neutral-600">{t("catalogSignIn")}</p>
            <Link
              href={routes.login}
              className="group inline-flex min-h-12 items-center gap-3 rounded-full bg-[#191b1e] px-6 text-sm font-semibold text-white transition-colors hover:bg-blue-700 focus-visible:outline-2 focus-visible:outline-offset-4 motion-reduce:transition-none"
            >
              {t("accountCta")}{" "}
              <ArrowRight
                aria-hidden="true"
                className="size-4 transition-transform group-hover:translate-x-1 motion-reduce:transition-none"
              />
            </Link>
          </section>
        )}
        <section className="grid gap-10 py-16 sm:py-24 lg:grid-cols-[0.8fr_1.2fr] lg:gap-20">
          <div>
            <p className="text-xs font-semibold tracking-[0.16em] text-blue-700 uppercase">
              {kind === "pricing" ? "02" : "01"} / {t(`${kind}SectionLabel`)}
            </p>
            <h2 className="mt-4 max-w-sm text-3xl font-semibold tracking-tight sm:text-4xl">
              {t(`${kind}SectionTitle`)}
            </h2>
          </div>
          <div>
            {sectionIds.map((id, i) => (
              <article
                key={id}
                className="grid gap-3 border-t border-black/15 py-7 sm:grid-cols-[3rem_1fr] sm:gap-5"
              >
                <span className="text-sm font-medium text-blue-700">
                  0{i + 1}
                </span>
                <div>
                  <h3 className="text-xl font-semibold tracking-tight">
                    {t(`${id}Title`)}
                  </h3>
                  <p className="mt-3 max-w-2xl text-base leading-7 text-neutral-600">
                    {t(`${id}Body`)}
                  </p>
                </div>
              </article>
            ))}
          </div>
        </section>
      </div>
    </PublicPageShell>
  );
}
