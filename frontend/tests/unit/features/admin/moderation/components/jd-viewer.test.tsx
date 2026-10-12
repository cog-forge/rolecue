import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { JdViewer, jdFile } from "@/features/admin/moderation/components/jd-viewer";

describe("jdFile", () => {
  it("recognises PDFs, ignoring case and query strings", () => {
    expect(jdFile("https://files.example.com/jd/a.PDF")?.isPdf).toBe(true);
    expect(jdFile("https://files.example.com/jd/a.pdf?v=2")?.isPdf).toBe(true);
    expect(jdFile("https://files.example.com/jd/a.txt")?.isPdf).toBe(false);
  });

  it("only accepts http(s) URLs", () => {
    expect(jdFile("javascript:alert(1)")).toBeNull();
    expect(jdFile("data:application/pdf;base64,AAAA")).toBeNull();
    expect(jdFile("jd/relative/path.pdf")).toBeNull();
  });
});

describe("JdViewer", () => {
  it("embeds the PDF and always offers a new-tab link", () => {
    render(<JdViewer contentPath="https://files.example.com/jd/go.pdf" />);
    expect(screen.getByTitle("Job description PDF")).toHaveAttribute(
      "src",
      "https://files.example.com/jd/go.pdf",
    );
    const link = screen.getByRole("link", { name: /open in new tab/i });
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("does not try to show a non-PDF file inline", () => {
    render(<JdViewer contentPath="https://files.example.com/jd/go.docx" />);
    expect(screen.getByText(/isn't a PDF/i)).toBeInTheDocument();
    expect(screen.queryByTitle("Job description PDF")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /open in new tab/i })).toBeInTheDocument();
  });

  it("never links an unsafe address", () => {
    render(<JdViewer contentPath="javascript:alert(1)" />);
    expect(screen.getByRole("alert")).toHaveTextContent(/isn't a valid web address/i);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
