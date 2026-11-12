"use client";

// Adapted from Kokonut UI's Action Search Bar (MIT), with controlled selection,
// keyboard navigation, and RoleCue primitives in place of the demo actions.
// https://github.com/kokonut-labs/kokonutui/blob/94d9004fab6ea169617625dd67932f8f6df3a85a/components/kokonutui/action-search-bar.tsx
// License: frontend/docs/LICENSE-kokonut-ui.txt
import { useId, useRef, useState, type ReactNode } from "react";
import { Check, ChevronsUpDown, Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverAnchor,
  PopoverContent,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

export type ActionSearchOption = {
  value: string;
  label: string;
  description?: string;
  keywords?: string[];
  icon?: ReactNode;
  disabled?: boolean;
};
type ActionSearchBarProps = {
  id?: string;
  options: ActionSearchOption[];
  value: string;
  onValueChange: (value: string) => void;
  placeholder: string;
  searchPlaceholder: string;
  emptyMessage: string;
  label: string;
  unavailableLabel?: string;
  disabled?: boolean;
  className?: string;
};
const searchable = (value: string) =>
  value
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .replace(/đ/gi, "d")
    .toLowerCase();

export function ActionSearchBar({
  id,
  options,
  value,
  onValueChange,
  placeholder,
  searchPlaceholder,
  emptyMessage,
  label,
  unavailableLabel,
  disabled = false,
  className,
}: ActionSearchBarProps) {
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const listId = `${inputId}-options`;
  const input = useRef<HTMLInputElement>(null);
  const anchor = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeValue, setActiveValue] = useState<string | null>(null);
  const selected = options.find((option) => option.value === value);
  const matching = options.filter((option) =>
    searchable(
      [option.label, option.description, ...(option.keywords ?? [])].join(" "),
    ).includes(searchable(query.trim())),
  );
  const enabled = matching.filter((option) => !option.disabled);
  const active =
    enabled.find((option) => option.value === activeValue) ?? enabled[0];
  const activeIndex = active ? matching.indexOf(active) : -1;
  const isOpen = open && !disabled;
  const optionId = (index: number) => `${listId}-${index}`;
  const showOptions = () => {
    setQuery("");
    setActiveValue(value);
    setOpen(true);
  };
  const choose = (option: ActionSearchOption) => {
    if (option.disabled) return;
    onValueChange(option.value);
    input.current?.focus({ preventScroll: true });
    setOpen(false);
  };
  const move = (direction: 1 | -1) => {
    if (!isOpen) {
      showOptions();
      return;
    }
    if (!enabled.length) return;
    const index = enabled.findIndex((option) => option.value === active?.value);
    const next = enabled[(index + direction + enabled.length) % enabled.length];
    setActiveValue(next.value);
    document
      .getElementById(optionId(matching.indexOf(next)))
      ?.scrollIntoView?.({ block: "nearest" });
  };
  return (
    <Popover open={isOpen} onOpenChange={setOpen}>
      <PopoverAnchor asChild>
        <div ref={anchor} className={cn("relative min-w-0", className)}>
          <Search
            aria-hidden
            className="pointer-events-none absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            ref={input}
            id={inputId}
            role="combobox"
            aria-label={label}
            aria-autocomplete="list"
            aria-haspopup="listbox"
            aria-expanded={isOpen}
            aria-controls={isOpen ? listId : undefined}
            aria-activedescendant={
              isOpen && activeIndex >= 0 ? optionId(activeIndex) : undefined
            }
            autoComplete="off"
            disabled={disabled}
            placeholder={isOpen ? searchPlaceholder : placeholder}
            value={
              isOpen
                ? query
                : (selected?.label ??
                  (value ? (unavailableLabel ?? placeholder) : ""))
            }
            className="h-11 bg-card pr-11 pl-10"
            onFocus={showOptions}
            onClick={() => {
              if (!isOpen) showOptions();
            }}
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveValue(null);
              setOpen(true);
            }}
            onKeyDown={(event) => {
              if (event.nativeEvent.isComposing) return;
              if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                event.preventDefault();
                move(event.key === "ArrowDown" ? 1 : -1);
              } else if (event.key === "Enter" && isOpen) {
                event.preventDefault();
                if (active) choose(active);
              } else if (event.key === "Escape" && isOpen) {
                event.preventDefault();
                event.stopPropagation();
                setOpen(false);
              } else if (event.key === "Tab") setOpen(false);
            }}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            tabIndex={-1}
            aria-label={label}
            aria-expanded={isOpen}
            disabled={disabled}
            className="absolute top-0 right-0 size-11 text-muted-foreground"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              if (isOpen) setOpen(false);
              else {
                input.current?.focus();
                showOptions();
              }
            }}
          >
            <ChevronsUpDown aria-hidden className="size-4" />
          </Button>
        </div>
      </PopoverAnchor>
      <PopoverContent
        align="start"
        className="w-(--radix-popper-anchor-width) p-1 motion-reduce:animate-none"
        onOpenAutoFocus={(event) => event.preventDefault()}
        onCloseAutoFocus={(event) => event.preventDefault()}
        onInteractOutside={(event) => {
          if (
            event.target instanceof Node &&
            anchor.current?.contains(event.target)
          )
            event.preventDefault();
        }}
        onEscapeKeyDown={(event) => event.stopPropagation()}
      >
        <div
          role="listbox"
          id={listId}
          aria-label={label}
          className="max-h-[min(18rem,var(--radix-popover-content-available-height))] overflow-y-auto overscroll-contain"
        >
          {matching.map((option, index) => (
            <button
              key={option.value}
              id={optionId(index)}
              type="button"
              role="option"
              tabIndex={-1}
              aria-selected={option.value === value}
              disabled={option.disabled}
              className={cn(
                "flex min-h-11 w-full items-center gap-3 rounded-sm px-3 py-2 text-left text-sm transition-colors disabled:pointer-events-none disabled:opacity-50 motion-reduce:transition-none",
                active?.value === option.value &&
                  "bg-accent text-accent-foreground",
              )}
              onMouseDown={(event) => event.preventDefault()}
              onPointerMove={(event) => {
                if (event.pointerType === "mouse") setActiveValue(option.value);
              }}
              onClick={() => choose(option)}
            >
              {option.icon && (
                <span aria-hidden className="shrink-0 text-rolecue-brand">
                  {option.icon}
                </span>
              )}
              <span className="min-w-0 flex-1">
                <span className="block wrap-anywhere">{option.label}</span>
                {option.description && (
                  <span className="mt-0.5 block text-xs text-muted-foreground wrap-anywhere">
                    {option.description}
                  </span>
                )}
              </span>
              {option.value === value && (
                <Check
                  aria-hidden
                  className="size-4 shrink-0 text-rolecue-brand"
                />
              )}
            </button>
          ))}
        </div>
        {matching.length === 0 && (
          <p
            role="status"
            className="px-3 py-5 text-center text-sm text-muted-foreground"
          >
            {emptyMessage}
          </p>
        )}
      </PopoverContent>
    </Popover>
  );
}
