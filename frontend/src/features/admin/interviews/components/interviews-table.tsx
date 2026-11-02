import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronRight } from "lucide-react";
import {
  DataTable,
  type DataTableColumn,
} from "@/components/data-table/data-table";
import { routes } from "@/config/routes";
import type { AdminInterviewRow } from "../api/interviews";

export function InterviewsTable({ rows }: { rows: AdminInterviewRow[] }) {
  const t = useTranslations("admin.interviews.table");
  const types = useTranslations("admin.interviews.type");
  const values = useTranslations("admin.interviews.values");
  const columns: DataTableColumn<AdminInterviewRow>[] = [
    {
      id: "interview",
      header: t("interview"),
      cell: (row) => (
        <div className="min-w-0">
          <p className="font-medium">{types(row.type)}</p>
          <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
            {row.id}
          </p>
        </div>
      ),
    },
    {
      id: "owner",
      header: t("owner"),
      cell: (row) => (
        <div className="min-w-0 max-w-64">
          <p className="truncate font-medium" title={row.owner.name}>
            {row.owner.name || t("noName")}
          </p>
          <p className="mt-1 break-all text-xs text-muted-foreground">
            {row.owner.email}
          </p>
        </div>
      ),
    },
    {
      id: "posting",
      header: t("posting"),
      cell: (row) =>
        row.job_posting ? (
          <span>{row.job_posting.title}</span>
        ) : (
          <span className="text-muted-foreground">
            {t("practiceNoPosting")}
          </span>
        ),
    },
    {
      id: "score",
      header: t("score"),
      cell: (row) =>
        row.score === null ? (
          <span className="text-muted-foreground">{values("notScored")}</span>
        ) : (
          <span>{row.score}</span>
        ),
    },
    {
      id: "report",
      header: t("report"),
      cell: (row) =>
        row.has_report ? values("available") : values("notAvailable"),
    },
    {
      id: "recording",
      header: t("recording"),
      cell: (row) =>
        row.has_recording ? values("recorded") : values("notRecorded"),
    },
    {
      id: "details",
      header: t("details"),
      cell: (row) => (
        <Link
          href={routes.admin.interview(row.id)}
          aria-label={t("viewLabel", { id: row.id })}
          className="inline-flex h-11 items-center gap-1 rounded-md px-2 font-medium text-workspace-accent underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          {t("view")}
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
      label={t("label")}
    />
  );
}
