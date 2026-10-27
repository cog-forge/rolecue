import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
import { PreflightCheck } from "@/features/interview-setup/components/preflight-check";
export default function Page() {
  return (
    <RoutePlaceholder
      title="Test audio & interview readiness"
      description="Check your microphone before your interview. You can change your input or try again if something needs attention."
    >
      <PreflightCheck />
    </RoutePlaceholder>
  );
}
