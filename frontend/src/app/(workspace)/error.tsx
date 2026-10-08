"use client";
import { Button } from "@/components/ui/button";
export default function WorkspaceError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <section role="alert" className="space-y-4">
      <h1 className="text-2xl font-semibold">Unable to load this page</h1>
      <Button onClick={reset}>Try again</Button>
    </section>
  );
}
