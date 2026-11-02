import { Suspense } from "react";
import { getTranslations } from "next-intl/server";
import { CandidateApplicationList } from "@/features/candidate/applications/components/application-list";

// The list reads its page from the URL on the client, so paging does not re-render this route.
export default async function Page() {
  const t = await getTranslations("candidate.applications.page");
  return (
    <section className="mx-auto w-full max-w-4xl space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("title")}
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          {t("description")}
        </p>
      </header>
      <Suspense>
        <CandidateApplicationList />
      </Suspense>
    </section>
  );
}
