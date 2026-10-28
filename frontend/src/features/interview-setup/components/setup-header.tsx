import type { ReactNode } from "react";

// One heading per step, in the workspace's header grammar (accent context line, title, one line
// of help). The wizard navigation above it says where the step sits.
export function SetupHeader({
  context,
  title,
  description,
}: {
  context: string;
  title: string;
  description: ReactNode;
}) {
  return (
    <header className="space-y-3">
      <p className="text-xs font-semibold tracking-widest text-workspace-accent uppercase">
        {context}
      </p>
      <h1 className="text-3xl font-semibold tracking-tight text-balance [overflow-wrap:anywhere] sm:text-4xl">
        {title}
      </h1>
      <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
        {description}
      </p>
    </header>
  );
}
