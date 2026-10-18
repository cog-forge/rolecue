import { JobPostingDetail } from "@/features/job-board/components/job-posting-detail";
import {
  boardSearch,
  parseBoardFilters,
  type SearchParams,
} from "@/features/job-board/utils/board-filters";

export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<SearchParams>;
}) {
  const [{ id }, query] = await Promise.all([params, searchParams]);
  // "back" carries the board filters; re-parse it so only known, valid filters survive.
  const back = typeof query.back === "string" ? query.back : "";
  const backQuery = boardSearch(
    parseBoardFilters(Object.fromEntries(new URLSearchParams(back))),
  ).slice(1);
  return <JobPostingDetail id={id} backQuery={backQuery} />;
}
