"use client";

import { useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { authClient } from "@/lib/auth/client";
import { routes } from "@/config/routes";
import { gooeyToast } from "goey-toast";

// Discards cached account data and leaves the workspace once the session has ended.
export function useEndLocalSession() {
  const cache = useQueryClient();
  const router = useRouter();
  return useCallback(async () => {
    await cache.cancelQueries();
    cache.clear();
    router.replace(routes.login);
    router.refresh();
  }, [cache, router]);
}

export function useSignOut() {
  const t = useTranslations("common.shell.signedOut");
  const endLocalSession = useEndLocalSession();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState(false);
  async function signOut() {
    if (pending) return;
    setPending(true);
    setError(false);
    try {
      const result = await authClient.signOut();
      if (result.error) throw new Error("Sign out failed");
      gooeyToast.success(t("title"), {
        description: t("description"),
      });
      await endLocalSession();
    } catch {
      setError(true);
      gooeyToast.error(t("failedTitle"), {
        description: t("failedDescription"),
      });
    } finally {
      setPending(false);
    }
  }
  return { signOut, pending, error };
}
