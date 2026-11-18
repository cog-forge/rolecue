import { queryOptions } from "@tanstack/react-query";
import { getEligibleAvatars, marketplaceKeys } from "./api/marketplace";

export function eligibleAvatarsQuery(userId: string) {
  return queryOptions({
    queryKey: marketplaceKeys.eligible(userId),
    queryFn: ({ signal }) => getEligibleAvatars(signal),
  });
}
