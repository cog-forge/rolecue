"use client";

import { useEffect, useRef, useState } from "react";
import { DataTable } from "@/components/data-table/data-table";
import { AiLoader } from "@/components/ui/ai-loader";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { LoadingMorph } from "@/components/ui/loading-morph";
import { NotFoundView } from "@/components/ui/not-found-view";

const views = [
  "Page loading",
  "Page error",
  "Table loading",
  "Table error",
  "Table empty",
  "AI generating",
  "AI analyzing",
  "AI evaluating",
  "404",
  "Success",
] as const;
type View = (typeof views)[number];
const names: Record<View, string> = {
  "Page loading": "Đang chuyển trang",
  "Page error": "Lỗi tải trang",
  "Table loading": "Bảng đang tải",
  "Table error": "Bảng tải lỗi",
  "Table empty": "Bảng trống",
  "AI generating": "AI tạo câu hỏi",
  "AI analyzing": "AI phân tích",
  "AI evaluating": "AI chấm điểm",
  "404": "Không tìm thấy trang",
  Success: "Tải thành công",
};

export function StatePreview({ initialView }: { initialView?: string }) {
  const [view, setView] = useState<View>(
    views.find((item) => item === initialView) ?? "Page loading",
  );
  const [retrying, setRetrying] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  function select(next: View) {
    if (timer.current) clearTimeout(timer.current);
    setRetrying(false);
    setView(next);
    const url = new URL(window.location.href);
    url.searchParams.set("state", next);
    window.history.replaceState(null, "", url);
  }
  function retry(table = false) {
    setView(table ? "Table loading" : "Page loading");
    setRetrying(true);
    timer.current = setTimeout(() => {
      setView("Success");
      setRetrying(false);
    }, 2000);
  }
  const columns = [
    {
      id: "name",
      header: "Job posting",
      cell: (row: { name: string; status: string }) => row.name,
    },
    {
      id: "status",
      header: "Status",
      cell: (row: { name: string; status: string }) => row.status,
    },
  ];
  return (
    <main className="mx-auto max-w-6xl px-6 py-8">
      <h1 className="text-2xl font-semibold">Kiểm tra trạng thái RoleCue</h1>
      <p className="mt-2 text-sm text-muted-foreground">
        Chỉ dùng ở development. Trạng thái được giữ nguyên đến khi bạn chọn mục
        khác. Nút thử lại mô phỏng tải trong 2 giây rồi thành công.
      </p>
      <nav
        aria-label="Preview states"
        className="mt-6 flex flex-wrap gap-2 border-b pb-5"
      >
        {views.map((item) => (
          <button
            key={item}
            type="button"
            aria-pressed={view === item}
            onClick={() => select(item)}
            className="min-h-11 rounded-md border px-4 text-sm aria-pressed:bg-primary aria-pressed:text-primary-foreground"
          >
            {names[item]}
          </button>
        ))}
      </nav>
      <p role="status" className="mt-4 text-xs text-muted-foreground">
        {retrying ? "Đang mô phỏng tải lại…" : `Đang giữ: ${names[view]}`}
      </p>
      <div className="py-6">
        {view === "Page loading" && <LoadingMorph className="min-h-[60vh]" />}
        {view === "Page error" && <ErrorState onRetry={() => retry()} />}
        {view.startsWith("Table") && (
          <DataTable
            rows={[]}
            columns={columns}
            getRowId={(row) => row.name}
            label="Example table"
            isLoading={view === "Table loading"}
            error={view === "Table error" ? new Error("example") : undefined}
            onRetry={() => retry(true)}
            emptyState={
              <EmptyState
                variant="plain"
                action={{
                  label: "Tạo job posting mẫu",
                  onClick: () => select("Success"),
                  variant: "outline",
                }}
              />
            }
          />
        )}
        {view.startsWith("AI") && (
          <AiLoader
            stage={
              view === "AI analyzing"
                ? "thinking"
                : view === "AI evaluating"
                  ? "evaluating"
                  : "generating"
            }
          />
        )}
        {view === "404" && <NotFoundView />}
        {view === "Success" && (
          <DataTable
            rows={[
              { name: "Backend Engineer", status: "Pending review" },
              { name: "Frontend Engineer", status: "Open" },
            ]}
            columns={columns}
            getRowId={(row) => row.name}
            label="Example table"
          />
        )}
      </div>
    </main>
  );
}
