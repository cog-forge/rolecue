import { JobPostingDetail } from "@/features/recruiter/job-postings/components/job-posting-detail";

export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <JobPostingDetail postingId={id} />;
}
