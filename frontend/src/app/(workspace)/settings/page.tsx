import {
  AccountSecurity,
  PasswordSettings,
} from "@/features/account-security/components/account-security";
import { AccountCredentials } from "@/features/auth/components/account-credentials";
import { enabledSocialProviders } from "@/features/auth/utils/social-providers";

export default async function SettingsPage({
  searchParams,
}: PageProps<"/settings">) {
  const { linked } = await searchParams;
  return (
    <AccountSecurity>
      <AccountCredentials
        enabledProviders={enabledSocialProviders(
          process.env.AUTH_SOCIAL_PROVIDERS,
        )}
        linkingFailed={linked === "failed"}
        password={<PasswordSettings />}
      />
    </AccountSecurity>
  );
}
