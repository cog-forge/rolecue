"use client";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { authClient } from "@/lib/auth/client";
import { routes } from "@/config/routes";
import { gooeyToast } from "goey-toast";

export function useSignOut() {
  const cache = useQueryClient();
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState(false);
  async function signOut() {
    if (pending) return;
    setPending(true);
    setError(false);
    try {
      const result = await authClient.signOut();
      if (result.error) throw new Error("Sign out failed");
      await cache.cancelQueries();
      cache.clear();
      gooeyToast.success("Signed out", {
        description: "You have safely signed out of RoleCue.",
      });
      router.replace(routes.login);
      router.refresh();
    } catch {
      setError(true);
      gooeyToast.error("Sign-out failed", {
        description: "You are still signed in. Please try again.",
      });
    } finally {
      setPending(false);
    }
  }
  return { signOut, pending, error };
}
