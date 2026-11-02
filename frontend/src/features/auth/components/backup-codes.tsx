"use client";

import { useState } from "react";
import { Download } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { CopyButton } from "./copy-button";

export function BackupCodes({
  codes,
  onDone,
}: {
  codes: readonly string[];
  onDone: () => void;
}) {
  const t = useTranslations("auth.security.backupCodes");
  const [saved, setSaved] = useState(false);
  const [message, setMessage] = useState("");
  function download() {
    const url = URL.createObjectURL(
      new Blob([t("fileHeader") + "\n\n" + codes.join("\n")], {
        type: "text/plain",
      }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "rolecue-backup-codes.txt";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
    setMessage(t("downloaded"));
  }
  return (
    <div className="space-y-4">
      <ul
        aria-label={t("list")}
        className="grid grid-cols-2 gap-x-6 gap-y-2 rounded-xl bg-muted px-5 py-4 font-mono text-sm tracking-wider tabular-nums"
      >
        {codes.map((code) => (
          <li key={code} className="wrap-anywhere">
            {code}
          </li>
        ))}
      </ul>
      <div className="grid grid-cols-2 gap-2">
        <CopyButton value={codes.join("\n")} label={t("copy")}>
          {t("copy")}
        </CopyButton>
        <Button variant="outline" className="min-h-11" onClick={download}>
          <Download aria-hidden />
          {t("download")}
        </Button>
      </div>
      {message && (
        <p role="status" className="text-sm text-muted-foreground">
          {message}
        </p>
      )}
      <div className="flex min-h-11 items-center gap-3">
        <Checkbox
          id="backup-codes-saved"
          checked={saved}
          onCheckedChange={(checked) => setSaved(checked === true)}
        />
        <label htmlFor="backup-codes-saved" className="text-sm">
          {t("saved")}
        </label>
      </div>
      <Button className="min-h-11 w-full" disabled={!saved} onClick={onDone}>
        {t("done")}
      </Button>
    </div>
  );
}
