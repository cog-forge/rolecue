import { ApplicationDetail } from "@/features/recruiter/applications/components/application-detail";

export default async function Page({
  params,
}: {
  params: Promise<{ id: string; applicationId: string }>;
}) {
  const { id, applicationId } = await params;
  return <ApplicationDetail postingId={id} applicationId={applicationId} />;
}
