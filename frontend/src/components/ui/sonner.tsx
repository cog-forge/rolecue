"use client";

import { Toaster as Sonner, type ToasterProps } from "sonner";

function Toaster(props: ToasterProps) {
  return (
    <Sonner
      theme="light"
      toastOptions={{
        classNames: {
          toast: "border border-border bg-background text-foreground",
        },
      }}
      {...props}
    />
  );
}

export { Toaster };
