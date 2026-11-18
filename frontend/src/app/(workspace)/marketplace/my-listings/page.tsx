import { Suspense } from "react";
import { MyListingsScreen } from "@/features/marketplace/components/my-listings-screen";
import { MarketplaceLoading } from "@/features/marketplace/components/marketplace-shared";
export default function Page() {
  return (
    <Suspense fallback={<MarketplaceLoading />}>
      <MyListingsScreen />
    </Suspense>
  );
}
