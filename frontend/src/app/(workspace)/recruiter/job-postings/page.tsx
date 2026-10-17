import { Suspense } from "react";
import { JobPostingList } from "@/features/recruiter/job-postings/components/job-posting-list";

// The list reads its filters from the URL on the client, so changing a tab or page does not
// re-render this route on the server.
export default function Page() {
  return (
    <Suspense>
      <JobPostingList />
    </Suspense>
  );
}
