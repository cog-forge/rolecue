"use client";

import { PasswordVisibilityIcon } from "@/components/icons/password-visibility-icon";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState } from "react";
import type { FieldError, UseFormRegisterReturn } from "react-hook-form";

export function PasswordField({
  id,
  label,
  autoComplete,
  help,
  error,
  disabled,
  registration,
}: {
  id: string;
  label: string;
  autoComplete: "current-password" | "new-password";
  help?: string;
  error?: FieldError;
  disabled?: boolean;
  registration: UseFormRegisterReturn;
}) {
  const [visible, setVisible] = useState(false);
  const describedBy = error ? `${id}-error` : help ? `${id}-help` : undefined;
  return (
    <div className={`t-input-wrap space-y-1.5 ${error ? "is-error" : ""}`}>
      <label htmlFor={id} className="block text-sm font-medium">
        {label}
      </label>
      <div className="relative">
        <Input
          id={id}
          type={visible ? "text" : "password"}
          autoComplete={autoComplete}
          aria-invalid={!!error}
          aria-describedby={describedBy}
          disabled={disabled}
          className={`t-input h-10 pr-12 ${error ? "is-error" : ""}`}
          {...registration}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={() => setVisible((current) => !current)}
          aria-label={`${visible ? "Hide" : "Show"} ${label.toLowerCase()}`}
          aria-controls={id}
          aria-pressed={visible}
          disabled={disabled}
          className="absolute top-1/2 right-1 -translate-y-1/2 active:not-aria-[haspopup]:-translate-y-1/2 text-muted-foreground hover:text-foreground"
        >
          <PasswordVisibilityIcon visible={visible} />
        </Button>
      </div>
      {error ? (
        <p
          id={`${id}-error`}
          role="alert"
          className="t-error-msg text-xs text-destructive"
        >
          {error.message}
        </p>
      ) : (
        help && (
          <p id={`${id}-help`} className="text-xs text-muted-foreground">
            {help}
          </p>
        )
      )}
    </div>
  );
}
