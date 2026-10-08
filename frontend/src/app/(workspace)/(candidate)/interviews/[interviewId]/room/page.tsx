import { InterviewRoom } from "@/features/interview/components/interview-room";
export default async function Page({
  params,
}: {
  params: Promise<{ interviewId: string }>;
}) {
  const { interviewId } = await params;
  return <InterviewRoom interviewId={interviewId} />;
}
