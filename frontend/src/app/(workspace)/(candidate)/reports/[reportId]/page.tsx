import { ReportOverview } from "@/features/report/components/report-overview";
export default async function Page({
  params,
}: {
  params: Promise<{ reportId: string }>;
}) {
  const { reportId } = await params;
  return <ReportOverview reportId={reportId} />;
}
