import { useEffect, useState } from "react";
import type { AdminAccount } from "../api/accounts";

// Re-render once at the next expiry instead of polling the list or the server.
export function useAccountClock(
  users: readonly AdminAccount[],
  updatedAt: number,
) {
  const [clock, setClock] = useState(Date.now);
  const now = Math.max(clock, updatedAt);
  useEffect(() => {
    const next = Math.min(
      ...users
        .filter(
          (user) =>
            user.banned && user.banExpires && user.banExpires.getTime() > now,
        )
        .map((user) => user.banExpires?.getTime() ?? Infinity),
    );
    if (!Number.isFinite(next)) return;
    const timer = setTimeout(
      () => setClock(Date.now()),
      Math.min(2_147_483_647, Math.max(0, next - Date.now() + 1)),
    );
    return () => clearTimeout(timer);
  }, [users, now]);
  return now;
}
