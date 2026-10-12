import { ModerationQueue } from "@/features/admin/moderation/components/moderation-queue";
import {
  parseQueueFilters,
  type SearchParams,
} from "@/features/admin/moderation/utils/queue-filters";

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  return <ModerationQueue filters={parseQueueFilters(await searchParams)} />;
}
