import { InterviewerPicker } from "@/features/interview-setup/components/interviewer-picker";
import { readSetupParams } from "@/features/interview-setup/lib/setup-params";

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { roleProfileId, avatarId, voiceId } = readSetupParams(
    await searchParams,
  );
  // Keyed by profile: a different role starts from a fresh form.
  return (
    <InterviewerPicker
      key={roleProfileId}
      roleProfileId={roleProfileId}
      avatarId={avatarId}
      voiceId={voiceId}
    />
  );
}
