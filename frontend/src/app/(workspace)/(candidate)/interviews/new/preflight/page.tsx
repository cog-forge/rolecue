import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
import { PreflightCheck } from "@/features/interview-setup/components/preflight-check";
export default function Page() {
  return (
    <RoutePlaceholder
      title="Preflight"
      description="Check browser support before the device and network readiness flow."
    >
      <PreflightCheck />
    </RoutePlaceholder>
  );
}
