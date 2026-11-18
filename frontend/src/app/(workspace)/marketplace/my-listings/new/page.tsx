import { Suspense } from "react";
import { ListingFormScreen } from "@/features/marketplace/components/listing-form-screen";
import { MarketplaceLoading } from "@/features/marketplace/components/marketplace-shared";
export default function Page() {
  return (
    <Suspense fallback={<MarketplaceLoading />}>
      <ListingFormScreen />
    </Suspense>
  );
}
