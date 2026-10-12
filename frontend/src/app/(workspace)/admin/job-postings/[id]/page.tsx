import { PostingReview } from "@/features/admin/moderation/components/posting-review";
import {
  parseQueueFilters,
  queueSearch,
  type SearchParams,
} from "@/features/admin/moderation/utils/queue-filters";

export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<SearchParams>;
}) {
  const [{ id }, query] = await Promise.all([params, searchParams]);
  // "back" carries the queue filters; re-parse it so only known, valid filters survive.
  const back = typeof query.back === "string" ? query.back : "";
  const backQuery = queueSearch(
    parseQueueFilters(Object.fromEntries(new URLSearchParams(back))),
  ).slice(1);
  return <PostingReview id={id} backQuery={backQuery} />;
}
