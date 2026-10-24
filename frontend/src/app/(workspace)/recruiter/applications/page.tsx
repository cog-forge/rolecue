import { Suspense } from "react";
import { ReceivedApplications } from "@/features/recruiter/applications/components/received-applications";

// The list reads its filters from the URL on the client, so changing a tab or page does not
// re-render this route on the server.
export default function Page() {
  return (
    <Suspense>
      <ReceivedApplications />
    </Suspense>
  );
}
