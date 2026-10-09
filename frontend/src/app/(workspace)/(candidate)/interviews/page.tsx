import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
import Link from "next/link";
import { routes } from "@/config/routes";
export default function Page() {
  return (
    <RoutePlaceholder
      title="Interviews"
      description="Create an interview or return to a session when the session service is connected."
    >
      <Link className="underline" href={routes.interviews.new.jobDescription}>
        Create an interview
      </Link>
    </RoutePlaceholder>
  );
}
