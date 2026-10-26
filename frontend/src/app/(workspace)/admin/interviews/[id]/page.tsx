import { AdminInterviewDetail } from "@/features/admin/interviews/components/admin-interview-detail";

export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <AdminInterviewDetail id={id} />;
}
