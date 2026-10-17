import { ExternalLink, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";

// Linked JD files are often PDFs. Only http(s) links are used, so a stored value can never become
// a javascript: URL.
export function jdFile(
  contentPath: string,
): { url: URL; isPdf: boolean } | null {
  let url: URL;
  try {
    url = new URL(contentPath);
  } catch {
    return null;
  }
  if (url.protocol !== "https:" && url.protocol !== "http:") return null;
  return { url, isPdf: url.pathname.toLowerCase().endsWith(".pdf") };
}

// Postings created in the app store the JD as a PDF, which the API links in documentUrl. The link
// expires after 15 minutes; the embedded PDF stays loaded, and reopening the review gets a new
// link. Postings created before PDF upload return their text in rawText. Older rows keep a link to
// a file in contentPath.
export function JdViewer({
  contentPath,
  documentUrl,
  rawText,
}: {
  contentPath: string;
  documentUrl: string | null;
  rawText: string | null;
}) {
  const file = documentUrl
    ? { url: new URL(documentUrl), isPdf: true }
    : rawText === null
      ? jdFile(contentPath)
      : null;
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
      {file?.isPdf ? (
        <>
          <iframe
            src={file.url.href}
            title="Job description PDF"
            referrerPolicy="no-referrer"
            className="h-[70vh] w-full rounded-lg border bg-muted/30"
          />
          {documentUrl && rawText !== null && (
            <details className="rounded-lg border p-3 text-sm">
              <summary className="cursor-pointer font-medium">
                Text read from the PDF
              </summary>
              <div className="mt-3 max-h-96 overflow-y-auto leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
                {rawText}
              </div>
            </details>
          )}
        </>
      ) : rawText !== null ? (
        <div
          tabIndex={0}
          aria-label="Job description text"
          className="max-h-[70vh] overflow-y-auto rounded-lg border bg-muted/30 p-4 text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]"
        >
          {rawText}
        </div>
      ) : file ? (
        <p className="text-sm text-muted-foreground">
          This job description isn&apos;t a PDF, so it can&apos;t be shown here.
          Open it in a new tab to read it.
        </p>
      ) : (
        <p role="alert" className="text-sm text-muted-foreground">
          The job description for this posting can&apos;t be found, so it
          can&apos;t be shown. Reject the posting and ask the recruiter to
          submit it again.
        </p>
      )}
    </section>
  );
}
