import { EvaluationCriteria } from "@/features/recruiter/evaluation/components/evaluation-criteria";

export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <EvaluationCriteria postingId={id} />;
}
