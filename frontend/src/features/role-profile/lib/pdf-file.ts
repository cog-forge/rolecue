import { maxPDFBytes } from "../schemas/upload-schema";

export type PdfErrorTranslator = (
  key: "notPdf" | "empty" | "tooLarge",
) => string;
export function pdfFileError(file: File, t: PdfErrorTranslator) {
  if (
    !file.name.toLowerCase().endsWith(".pdf") ||
    (file.type && file.type !== "application/pdf")
  )
    return t("notPdf");
  if (file.size === 0) return t("empty");
  if (file.size > maxPDFBytes) return t("tooLarge");
  return null;
}
