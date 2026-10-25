import { CandidateApplicationDetail } from "@/features/candidate/applications/components/application-detail";

export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <CandidateApplicationDetail id={id} />;
}
