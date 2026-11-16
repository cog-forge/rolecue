import { RoadmapScreen } from "@/features/roadmap/components/roadmap-screen";

export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <RoadmapScreen key={id} profileId={id} />;
}
