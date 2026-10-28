import { InterviewReadiness } from "@/features/interview-setup/components/interview-readiness";
import { readSetupParams } from "@/features/interview-setup/lib/setup-params";

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  return <InterviewReadiness {...readSetupParams(await searchParams)} />;
}
