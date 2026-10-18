import { Suspense } from "react";
import { JobBoard } from "@/features/job-board/components/job-board";

// The board reads its filters from the URL on the client, so a new search or page does not
// re-render this route on the server.
export default function Page() {
  return (
    <Suspense>
      <JobBoard />
    </Suspense>
  );
}
