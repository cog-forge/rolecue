import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { DocumentUploadPanel } from "@/features/role-profile/components/document-upload-panel";

const pdf = (name = "Quan's JD.pdf", size = 2048) =>
  new File([new Uint8Array(size)], name, { type: "application/pdf" });

describe("DocumentUploadPanel in Vietnamese", () => {
  it("translates the empty panel and its accessible name", () => {
    renderWithIntl(
      <DocumentUploadPanel
        file={null}
        onChange={vi.fn()}
        disabled={false}
        progress={null}
      />,
      { locale: "vi" },
    );
    expect(screen.getByText("Thả tệp PDF của bạn vào đây")).toBeInTheDocument();
    expect(
      screen.getByText(/PDF có văn bản chọn được, tối đa 5 MB/),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("PDF mô tả công việc")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Chọn PDF" }),
    ).toBeInTheDocument();
  });

  it("shows the file name as given, a localized size and the progress", () => {
    renderWithIntl(
      <DocumentUploadPanel
        file={pdf("Quan's JD.pdf", 1536)}
        onChange={vi.fn()}
        disabled={false}
        progress={40}
      />,
      { locale: "vi" },
    );
    expect(screen.getByText("Quan's JD.pdf")).toBeInTheDocument();
    expect(screen.getByText("1,5 KB · Đã chọn PDF")).toBeInTheDocument();
    expect(screen.getByText("Đã tải lên 40%")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "Tiến trình tải PDF lên" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Thay PDF" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xóa PDF" })).toBeInTheDocument();
  });

  it.each([
    [
      new File(["x"], "notes.txt", { type: "text/plain" }),
      "Hãy chọn một tài liệu PDF.",
    ],
    [pdf("empty.pdf", 0), "PDF này trống. Hãy chọn tài liệu khác."],
    [pdf("big.pdf", 5 * 1024 * 1024 + 1), "Hãy chọn PDF tối đa 5 MB."],
  ])("rejects a bad file in Vietnamese", (file, message) => {
    const onChange = vi.fn();
    renderWithIntl(
      <DocumentUploadPanel
        file={null}
        onChange={onChange}
        disabled={false}
        progress={null}
      />,
      { locale: "vi" },
    );
    fireEvent.change(screen.getByLabelText("PDF mô tả công việc"), {
      target: { files: [file] },
    });
    expect(screen.getByRole("alert")).toHaveTextContent(message);
    expect(onChange).not.toHaveBeenCalled();
  });
});
