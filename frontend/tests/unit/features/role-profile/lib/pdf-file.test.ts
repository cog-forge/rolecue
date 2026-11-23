import { describe, expect, it } from "vitest";
import { createTranslator } from "next-intl";
import { messagesFor } from "@tests/helpers/intl";
import { pdfFileError } from "@/features/role-profile/lib/pdf-file";

const uploadErrors = createTranslator({
  locale: "en",
  messages: messagesFor("en"),
  namespace: "candidate.roleProfiles.upload.errors",
});

describe("PDF upload guard", () => {
  it("rejects empty, oversized and non-PDF uploads", () => {
    expect(
      pdfFileError(
        new File([], "jd.pdf", { type: "application/pdf" }),
        uploadErrors,
      ),
    ).toContain("empty");
    expect(
      pdfFileError(
        new File([new Uint8Array(5 * 1024 * 1024 + 1)], "jd.pdf", {
          type: "application/pdf",
        }),
        uploadErrors,
      ),
    ).toContain("5 MB");
    expect(
      pdfFileError(
        new File(["JD"], "jd.txt", { type: "text/plain" }),
        uploadErrors,
      ),
    ).toContain("PDF");
    expect(
      pdfFileError(
        new File(["JD"], "jd.pdf", { type: "application/pdf" }),
        uploadErrors,
      ),
    ).toBeNull();
  });
});
