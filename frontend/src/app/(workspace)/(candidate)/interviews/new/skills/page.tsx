import { redirect } from "next/navigation";
import { routes } from "@/config/routes";
import { RequirementsStep } from "@/features/interview-setup/components/requirements-step";
import { readSetupParams } from "@/features/interview-setup/lib/setup-params";

// Step 2 of the wizard. Without a selected Role Profile there is nothing to show, so it still sends
// the reader to start one.
export default async function Page({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { roleProfileId, focusItemId } = readSetupParams(await searchParams);
  if (!roleProfileId) redirect(routes.roleProfiles.new);
  return (
    <RequirementsStep roleProfileId={roleProfileId} focusItemId={focusItemId} />
  );
}
