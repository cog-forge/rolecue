import { RoleProfileConfirmedGate } from "@/features/interview-setup/components/role-profile-confirmed-gate";
export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{ roleProfileId?: string }>;
}) {
  const { roleProfileId } = await searchParams;
  return <RoleProfileConfirmedGate key={roleProfileId} id={roleProfileId} />;
}
