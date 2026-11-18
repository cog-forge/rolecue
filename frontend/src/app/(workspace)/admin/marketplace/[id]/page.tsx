import { AdminMarketplaceDetail } from "@/features/marketplace/components/admin-marketplace-screen";
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <AdminMarketplaceDetail id={id} />;
}
