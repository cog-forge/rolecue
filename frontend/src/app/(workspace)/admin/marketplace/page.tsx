import { Suspense } from "react";
import { AdminMarketplaceScreen } from "@/features/marketplace/components/admin-marketplace-screen";
import { MarketplaceLoading } from "@/features/marketplace/components/marketplace-shared";
export default function Page() {
  return (
    <Suspense fallback={<MarketplaceLoading />}>
      <AdminMarketplaceScreen />
    </Suspense>
  );
}
