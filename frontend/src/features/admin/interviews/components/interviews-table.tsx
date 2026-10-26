import Link from "next/link";
import { ChevronRight } from "lucide-react";
import {
  DataTable,
  type DataTableColumn,
} from "@/components/data-table/data-table";
import { routes } from "@/config/routes";
import type { AdminInterviewRow } from "../api/interviews";

const typeLabels: Record<AdminInterviewRow["type"], string> = {
  practice: "Practice",
  recruitment: "Recruitment",
};

export function InterviewsTable({ rows }: { rows: AdminInterviewRow[] }) {
  const columns: DataTableColumn<AdminInterviewRow>[] = [
    {
      id: "interview",
      header: "Interview",
      cell: (row) => (
        <div className="min-w-0">
          <p className="font-medium">{typeLabels[row.type]}</p>
          <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
            {row.id}
          </p>
        </div>
      ),
    },
    {
      id: "owner",
      header: "Owner",
      cell: (row) => (
        <div className="min-w-0 max-w-64">
          <p className="truncate font-medium" title={row.owner.name}>
            {row.owner.name || "No name provided"}
          </p>
          <p className="mt-1 break-all text-xs text-muted-foreground">
            {row.owner.email}
          </p>
        </div>
      ),
    },
    {
      id: "posting",
      header: "Job Posting",
      cell: (row) =>
        row.job_posting ? (
          <span>{row.job_posting.title}</span>
        ) : (
          <span className="text-muted-foreground">Practice (no posting)</span>
        ),
    },
    {
      id: "score",
      header: "Score",
      cell: (row) =>
        row.score === null ? (
          <span className="text-muted-foreground">Not scored</span>
        ) : (
          <span>{row.score}</span>
        ),
    },
    {
      id: "report",
      header: "Report",
      cell: (row) => (row.has_report ? "Available" : "Not available"),
    },
    {
      id: "recording",
      header: "Recording",
      cell: (row) => (row.has_recording ? "Recorded" : "Not recorded"),
    },
    {
      id: "details",
      header: "Details",
      cell: (row) => (
        <Link
          href={routes.admin.interview(row.id)}
          aria-label={`View details for interview ${row.id}`}
          className="inline-flex h-11 items-center gap-1 rounded-md px-2 font-medium text-workspace-accent underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          View
          <ChevronRight className="size-4" aria-hidden />
        </Link>
      ),
    },
  ];
  return (
    <DataTable
      rows={rows}
      columns={columns}
      getRowId={(row) => row.id}
      label="Interview sessions"
    />
  );
}
