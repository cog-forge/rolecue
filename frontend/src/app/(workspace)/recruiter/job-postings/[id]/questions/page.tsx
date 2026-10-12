import { QuestionBank } from "@/features/recruiter/question-bank/components/question-bank";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <QuestionBank postingId={id} />;
}
