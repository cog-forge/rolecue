"use client";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
export default function WorkspaceError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("common");
  return (
    <section role="alert" className="space-y-4">
      <h1 className="text-2xl font-semibold">{t("workspaceError.title")}</h1>
      <Button onClick={reset}>{t("globalError.retry")}</Button>
    </section>
  );
}
