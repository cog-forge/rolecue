import Link from "next/link";
import { useTranslations } from "next-intl";
import { Check, ShieldCheck } from "lucide-react";
import { UserAvatar } from "@/components/account/user-avatar";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { routes } from "@/config/routes";
import type { Profile } from "../schemas/profile-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

export function ProfileSummary({ profile }: { profile: Profile }) {
  const t = useTranslations("common.profile.summary");
  const roles = useTranslations("common.shell.roles");
  const fmt = useFormatters();
  return (
    <Card className="h-fit gap-0 py-6">
      <CardContent className="space-y-6 px-6">
        <div className="space-y-4">
          <UserAvatar
            id={profile.id}
            name={profile.full_name}
            image={profile.image}
            className="size-20 rounded-2xl"
          />
          <div className="min-w-0 space-y-1">
            <h2 className="wrap-anywhere text-xl font-semibold tracking-tight">
              {profile.full_name}
            </h2>
            <p className="text-sm text-muted-foreground">
              {roles(profile.role)}
            </p>
          </div>
        </div>
        <dl className="space-y-4 border-t pt-5 text-sm">
          <div className="space-y-1">
            <dt className="text-muted-foreground">{t("email")}</dt>
            <dd className="wrap-anywhere font-medium">{profile.email}</dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">{t("emailStatus")}</dt>
            <dd className="flex items-center gap-1.5 font-medium">
              {profile.email_verified && (
                <Check className="size-4 text-workspace-accent" aria-hidden />
              )}
              {profile.email_verified ? t("verified") : t("unverified")}
            </dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">{t("memberSince")}</dt>
            <dd className="font-medium">
              {fmt.date(profile.created_at, "monthYear")}
            </dd>
          </div>
        </dl>
        <div className="border-t pt-5">
          <p className="mb-3 text-xs leading-relaxed text-muted-foreground">
            {t("securityHint")}
          </p>
          <Button variant="outline" asChild className="w-full">
            <Link href={routes.settings}>
              <ShieldCheck aria-hidden />
              {t("security")}
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
