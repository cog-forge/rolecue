"use client";
import { QueryProvider } from "./query-provider";
import { Toaster } from "@/components/ui/sonner";
export function AppProviders({ children }: { children: React.ReactNode }) {
  return (
    <QueryProvider>
      {children}
      <Toaster />
    </QueryProvider>
  );
}
