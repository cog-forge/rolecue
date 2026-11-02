import { useTranslations } from "next-intl";
import { ApplicationsCard } from "./applications-card";
import { PracticeActivityCard } from "./practice-activity-card";
import { RoleProfilesCard } from "./role-profiles-card";
import { WalletCard } from "./wallet-card";

// Each card loads and fails on its own, so the page stays useful when one request is down.
export function DashboardOverview() {
  const t = useTranslations("candidate.dashboard");
  return (
    <div className="mx-auto w-full max-w-5xl space-y-8">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("heading")}
        </h1>
        <p className="text-muted-foreground">{t("description")}</p>
      </header>
      <div className="grid gap-4 md:grid-cols-2">
        <RoleProfilesCard />
        <PracticeActivityCard />
        <ApplicationsCard />
        <WalletCard />
      </div>
    </div>
  );
}
