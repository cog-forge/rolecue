import { Skeleton } from "@/components/ui/skeleton";
export default function Loading() {
  return (
    <div role="status" className="space-y-4">
      <span className="sr-only">Loading page</span>
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-32 w-full" />
    </div>
  );
}
