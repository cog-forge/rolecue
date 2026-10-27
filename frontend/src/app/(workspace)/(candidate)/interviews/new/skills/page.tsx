import { redirect } from "next/navigation";
import { routes } from "@/config/routes";
export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{ roleProfileId?: string }>;
}) {
  const { roleProfileId } = await searchParams;
  redirect(
    roleProfileId
      ? routes.roleProfiles.detail(roleProfileId)
      : routes.roleProfiles.new,
  );
}
