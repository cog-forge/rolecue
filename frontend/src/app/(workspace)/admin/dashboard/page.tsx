import { Suspense } from "react";
import { AdminOverview } from "@/features/admin/overview/components/admin-overview";

export default function Page() {
  return (
    <Suspense
      fallback={
        <p role="status" className="text-sm text-muted-foreground">
          Loading admin overview…
        </p>
      }
    >
      <AdminOverview />
    </Suspense>
  );
}
