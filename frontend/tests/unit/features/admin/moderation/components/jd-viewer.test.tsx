import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  JdViewer,
  jdFile,
} from "@/features/admin/moderation/components/jd-viewer";

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
  it("embeds the JD PDF of a posting created in the app, with its text on request", () => {
    const url = "https://storage.example.com/role-cue/abc?X-Amz-Signature=sig";
    render(
      <JdViewer
        contentPath="0b6c3a52-6a3d-4b7e-9d55-8c1f2a3b4c5d"
        documentUrl={url}
        rawText="Text read from the PDF."
      />,
    );
    expect(screen.getByTitle("Job description PDF")).toHaveAttribute(
      "src",
      url,
    );
    expect(
      screen.getByRole("link", { name: /open in new tab/i }),
    ).toHaveAttribute("href", url);
    expect(screen.getByText("Text read from the PDF")).toBeInTheDocument();
    expect(screen.getByText("Text read from the PDF.")).not.toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the stored JD text of a posting created before PDF upload", () => {
    const text = "Senior Go engineer.\n\nBuilds our payment APIs.";
    render(
      <JdViewer
        contentPath="job-descriptions/abc.txt"
        documentUrl={null}
        rawText={text}
      />,
    );
    const region = screen.getByLabelText("Job description text");
    expect(region.textContent).toBe(text);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByTitle("Job description PDF")).not.toBeInTheDocument();
  });

  it("renders JD text as plain text, never as markup", () => {
    render(
      <JdViewer
        contentPath="job-descriptions/abc.txt"
        documentUrl={null}
        rawText={'<img src=x onerror="alert(1)"><b>bold</b>'}
      />,
    );
    const region = screen.getByLabelText("Job description text");
    expect(region.querySelector("img, b")).toBeNull();
    expect(region).toHaveTextContent("<b>bold</b>");
  });

  it("embeds the PDF and always offers a new-tab link", () => {
    render(
      <JdViewer
        contentPath="https://files.example.com/jd/go.pdf"
        documentUrl={null}
        rawText={null}
      />,
    );
    expect(screen.getByTitle("Job description PDF")).toHaveAttribute(
      "src",
      "https://files.example.com/jd/go.pdf",
    );
    const link = screen.getByRole("link", { name: /open in new tab/i });
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("does not try to show a non-PDF file inline", () => {
    render(
      <JdViewer
        contentPath="https://files.example.com/jd/go.docx"
        documentUrl={null}
        rawText={null}
      />,
    );
    expect(screen.getByText(/isn't a PDF/i)).toBeInTheDocument();
    expect(screen.queryByTitle("Job description PDF")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /open in new tab/i }),
    ).toBeInTheDocument();
  });

  it("explains a missing JD instead of showing a broken view", () => {
    render(
      <JdViewer
        contentPath="job-descriptions/gone.txt"
        documentUrl={null}
        rawText={null}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/can't be found/i);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("never links an unsafe address", () => {
    render(
      <JdViewer
        contentPath="javascript:alert(1)"
        documentUrl={null}
        rawText={null}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/can't be found/i);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
