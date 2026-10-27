import { Suspense } from "react";
import { AvatarInventory } from "@/features/avatar/components/avatar-inventory";

// The inventory reads its page and open preview from the URL on the client.
export default function Page() {
  return (
    <Suspense>
      <AvatarInventory />
    </Suspense>
  );
}
