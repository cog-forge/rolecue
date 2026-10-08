"use client";

import { useId, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Search } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { workspaceNavigation } from "@/config/navigation";
import type { ProductRole } from "@/config/permissions";
import { cn } from "@/lib/utils";

function SearchPages({
  role,
  onNavigate,
}: {
  role: ProductRole;
  onNavigate: () => void;
}) {
  const [query, setQuery] = useState("");
  const [selection, setSelection] = useState(0);
  const router = useRouter();
  const id = useId();
  const items = workspaceNavigation[role]
    .flatMap((group) => group.items)
    .filter((item) =>
      item.label.toLowerCase().includes(query.trim().toLowerCase()),
    );
  return (
    <>
      <div className="relative">
        <Search
          aria-hidden
          className="absolute top-3 left-3 size-5 text-muted-foreground"
        />
        <Input
          autoFocus
          aria-label="Search workspace pages"
          placeholder="Where would you like to go?"
          role="combobox"
          aria-expanded
          aria-controls={`${id}-results`}
          aria-autocomplete="list"
          aria-activedescendant={
            items.length ? `${id}-${selection}` : undefined
          }
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setSelection(0);
          }}
          className="h-11 pr-3 pl-10"
          onKeyDown={(event) => {
            if (event.key === "ArrowDown" || event.key === "ArrowUp") {
              event.preventDefault();
              const next = items.length
                ? (selection +
                    (event.key === "ArrowDown" ? 1 : items.length - 1)) %
                  items.length
                : 0;
              setSelection(next);
              document
                .getElementById(`${id}-${next}`)
                ?.scrollIntoView({ block: "nearest" });
            }
            if (event.key === "Enter") {
              event.preventDefault();
              if (items[selection]) {
                router.push(items[selection].href);
                onNavigate();
              }
            }
          }}
        />
      </div>
      <ul
        id={`${id}-results`}
        role="listbox"
        aria-label="Workspace pages"
        className="max-h-[min(50dvh,360px)] space-y-1 overflow-y-auto"
      >
        {items.map((item, index) => (
          <li key={item.href} role="presentation">
            <Button
              asChild
              variant="ghost"
              className={cn(
                "h-11 w-full justify-start gap-3",
                selection === index && "bg-accent",
              )}
            >
              <Link
                id={`${id}-${index}`}
                href={item.href}
                role="option"
                aria-selected={selection === index}
                onFocus={() => setSelection(index)}
                onClick={onNavigate}
              >
                <item.icon className="size-5" aria-hidden />
                {item.label}
              </Link>
            </Button>
          </li>
        ))}
      </ul>
      {!items.length && (
        <p
          role="status"
          className="py-8 text-center text-sm text-muted-foreground"
        >
          No matching pages.
        </p>
      )}
      <p className="text-xs text-muted-foreground">
        Use ↑ ↓ to choose a page, Enter to open, Esc to close.
      </p>
    </>
  );
}

export function WorkspaceSearch({
  role,
  open,
  onOpenChange,
  onNavigate,
  returnFocus,
}: {
  role: ProductRole;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onNavigate: () => void;
  returnFocus: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        overlayClassName="z-95 motion-reduce:animate-none"
        className="z-100 rounded-2xl motion-reduce:animate-none"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          returnFocus();
        }}
      >
        <DialogHeader>
          <DialogTitle>Find a page</DialogTitle>
          <DialogDescription>
            Quick navigation within your workspace.
          </DialogDescription>
        </DialogHeader>
        <SearchPages role={role} onNavigate={onNavigate} />
      </DialogContent>
    </Dialog>
  );
}
