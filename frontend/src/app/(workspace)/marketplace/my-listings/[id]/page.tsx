import { Suspense } from "react";
import { ListingFormScreen } from "@/features/marketplace/components/listing-form-screen";
import { MarketplaceLoading } from "@/features/marketplace/components/marketplace-shared";
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <Suspense fallback={<MarketplaceLoading />}>
      <ListingFormScreen id={id} />
    </Suspense>
  );
}
