import type { ReactNode } from "react";
import { PostingNavigation } from "@/features/recruiter/job-postings/components/posting-navigation";

export default async function Layout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <PostingNavigation postingId={id}>{children}</PostingNavigation>;
}
