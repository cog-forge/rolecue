"use client";
import { QueryProvider } from "./query-provider";
import { GooeyToaster } from "goey-toast";
import { useReducedMotion } from "motion/react";
import { WorkspacePreferencesProvider } from "./workspace-provider";
export function AppProviders({ children }: { children: React.ReactNode }) {
  const reducedMotion = useReducedMotion();
  return (
    <QueryProvider>
      <WorkspacePreferencesProvider>
        {children}
        <GooeyToaster
          position="bottom-right"
          theme="dark"
          preset="smooth"
          spring={!reducedMotion}
          duration={5000}
          closeButton
          showTimestamp={false}
          visibleToasts={3}
        />
      </WorkspacePreferencesProvider>
    </QueryProvider>
  );
}
