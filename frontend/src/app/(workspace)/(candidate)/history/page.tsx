import { Suspense } from "react";
import { InterviewHistory } from "@/features/history/components/interview-history";

// The list reads its type and page from the URL on the client, so filtering does not re-render
// this route.
export default function Page() {
  return (
    <Suspense>
      <InterviewHistory />
    </Suspense>
  );
}
