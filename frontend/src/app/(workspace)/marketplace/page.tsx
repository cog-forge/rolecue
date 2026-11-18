import { Suspense } from "react";
import { CatalogScreen } from "@/features/marketplace/components/catalog-screen";
import { MarketplaceLoading } from "@/features/marketplace/components/marketplace-shared";
export default function Page() {
  return (
    <Suspense fallback={<MarketplaceLoading />}>
      <CatalogScreen />
    </Suspense>
  );
}
