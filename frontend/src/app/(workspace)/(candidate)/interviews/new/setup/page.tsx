import { RoleProfilePicker } from "@/features/interview-setup/components/role-profile-picker";
import { readSetupParams } from "@/features/interview-setup/lib/setup-params";

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { roleProfileId } = readSetupParams(await searchParams);
  return <RoleProfilePicker roleProfileId={roleProfileId} />;
}
