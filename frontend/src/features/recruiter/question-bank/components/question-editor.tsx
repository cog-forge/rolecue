"use client";

import { useEffect } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  questionFormSchema,
  questionLength,
  type QuestionValues,
} from "../schemas/question-bank-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

// One form for adding and editing a question. The draft stays when the request fails, because
// the parent keeps the editor mounted and only closes it on success.
export function QuestionEditor({
  id,
  label,
  initial = "",
  maxLength,
  pending,
  error,
  submitLabel,
  pendingLabel,
  onSubmit,
  onCancel,
  onDirtyChange,
}: {
  id: string;
  label: string;
  initial?: string;
  maxLength: number;
  pending: boolean;
  error: string | null;
  submitLabel: string;
  pendingLabel: string;
  onSubmit: (content: string) => Promise<boolean>;
  onCancel?: () => void;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const fmt = useFormatters();
  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors, isDirty, isValid },
  } = useForm<QuestionValues>({
    resolver: zodResolver(questionFormSchema(maxLength)),
    defaultValues: { content: initial },
    mode: "onChange",
  });
  useEffect(() => {
    onDirtyChange?.(isDirty);
  }, [isDirty, onDirtyChange]);
  const content = useWatch({ control, name: "content" });
  const fieldError = errors.content?.message;
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;

  return (
    <form
      noValidate
      aria-label={label}
      className="space-y-2"
      onSubmit={handleSubmit(async (values) => {
        if (await onSubmit(values.content.trim())) reset({ content: initial });
      })}
    >
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <Textarea
        id={id}
        rows={3}
        className="bg-card"
        disabled={pending}
        aria-invalid={!!fieldError}
        aria-describedby={[hintId, fieldError ? errorId : ""]
          .filter(Boolean)
          .join(" ")}
        {...register("content")}
      />
      <p
        id={hintId}
        className="flex justify-between gap-3 text-xs text-muted-foreground"
      >
        <span>Asked by the AI interviewer. Candidates never see the bank.</span>
        <span className="tabular-nums">
          {fmt.number(questionLength(content ?? ""))}/{fmt.number(maxLength)}
        </span>
      </p>
      {fieldError && (
        <p id={errorId} role="alert" className="text-xs text-destructive">
          {fieldError}
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <Button
          type="submit"
          className="h-10 rounded-full px-5"
          disabled={pending || !isDirty || !isValid}
          aria-busy={pending}
        >
          {pending && (
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
          )}
          {pending ? pendingLabel : submitLabel}
        </Button>
        {onCancel && (
          <Button
            type="button"
            variant="outline"
            className="h-10 rounded-full px-5"
            disabled={pending}
            onClick={onCancel}
          >
            Cancel
          </Button>
        )}
      </div>
    </form>
  );
}
