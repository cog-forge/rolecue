import { ExternalLink, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";

// JDs are PDF files. Only http(s) links are used, so a stored value can never become a
// javascript: URL.
export function jdFile(contentPath: string): { url: URL; isPdf: boolean } | null {
  let url: URL;
  try {
    url = new URL(contentPath);
  } catch {
    return null;
  }
  if (url.protocol !== "https:" && url.protocol !== "http:") return null;
  return { url, isPdf: url.pathname.toLowerCase().endsWith(".pdf") };
}

export function JdViewer({ contentPath }: { contentPath: string }) {
  const file = jdFile(contentPath);
  return (
    <section aria-labelledby="jd-heading" className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="jd-heading" className="flex items-center gap-2 font-semibold">
          <FileText className="size-4 text-muted-foreground" aria-hidden />
          Job description
        </h2>
        {file && (
          <Button asChild variant="outline" size="sm" className="h-9">
            <a href={file.url.href} target="_blank" rel="noopener noreferrer">
              <ExternalLink aria-hidden />
              Open in new tab
              <span className="sr-only"> (job description file)</span>
            </a>
          </Button>
        )}
      </div>
      {!file ? (
        <p role="alert" className="text-sm text-muted-foreground">
          The job description link for this posting isn&apos;t a valid web
          address, so it can&apos;t be shown. Ask the recruiter to upload it
          again.
        </p>
      ) : file.isPdf ? (
        <iframe
          src={file.url.href}
          title="Job description PDF"
          referrerPolicy="no-referrer"
          className="h-[70vh] w-full rounded-lg border bg-muted/30"
        />
      ) : (
        <p className="text-sm text-muted-foreground">
          This job description isn&apos;t a PDF, so it can&apos;t be shown
          here. Open it in a new tab to read it.
        </p>
      )}
    </section>
  );
}
