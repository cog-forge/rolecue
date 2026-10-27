import { RoleProfileDetail } from "@/features/role-profile/components/role-profile-detail";
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <RoleProfileDetail key={id} id={id} />;
}
