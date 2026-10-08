import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
export function AdminSection({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <RoutePlaceholder title={title} description={description}>
      <p className="text-sm text-muted-foreground">
        This area is being prepared. More tools will appear here as they become
        available.
      </p>
    </RoutePlaceholder>
  );
}
