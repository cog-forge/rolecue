import { CatalogDetailScreen } from "@/features/marketplace/components/catalog-screen";
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <CatalogDetailScreen id={id} />;
}
