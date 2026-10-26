"use client";

import { useId } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { useForm, useWatch } from "react-hook-form";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  fundCapacitySchema,
  MAX_SLOTS,
  MIN_SLOTS,
  type FundCapacityInput,
  type FundCapacityValues,
} from "../schemas/capacity-schema";

// Buys N interview slots in one purchase. The total is only a preview; the server charges it.
export function FundCapacityForm({
  pricePerSlot,
  pending,
  error,
  onSubmit,
}: {
  pricePerSlot: number;
  pending: boolean;
  error: string | null;
  onSubmit: (slots: number) => void;
}) {
  const inputId = useId();
  const hintId = useId();
  const errorId = useId();
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<FundCapacityInput, unknown, FundCapacityValues>({
    resolver: zodResolver(fundCapacitySchema),
    defaultValues: { slots: "" },
    mode: "onTouched",
  });
  const typed = useWatch({ control, name: "slots" }) ?? "";
  const count = /^\d+$/.test(typed.trim()) ? Number(typed.trim()) : null;
  // Null while the count is not a valid whole number in range; the server still decides.
  const total =
    count !== null && count >= MIN_SLOTS && count <= MAX_SLOTS
      ? count * pricePerSlot
      : null;
  const slotError = errors.slots?.message;

  return (
    <form
      noValidate
      aria-label="Fund interview slots"
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit((values) => onSubmit(values.slots))(event);
      }}
    >
      <div className="space-y-2">
        <label htmlFor={inputId} className="text-sm font-medium">
          Interview slots to fund
        </label>
        <Input
          id={inputId}
          type="number"
          inputMode="numeric"
          min={MIN_SLOTS}
          max={MAX_SLOTS}
          step={1}
          className="h-11 max-w-40 tabular-nums"
          disabled={pending}
          aria-invalid={!!slotError}
          aria-describedby={[hintId, slotError ? errorId : ""]
            .filter(Boolean)
            .join(" ")}
          {...register("slots")}
        />
        <p id={hintId} className="text-xs text-muted-foreground">
          {MIN_SLOTS} to {MAX_SLOTS.toLocaleString("en")} slots. Each slot costs{" "}
          {pricePerSlot} {pricePerSlot === 1 ? "coin" : "coins"}.
        </p>
        {slotError && (
          <p id={errorId} role="alert" className="text-xs text-destructive">
            {slotError}
          </p>
        )}
      </div>
      <p className="text-sm text-muted-foreground" aria-live="polite">
        {total === null
          ? "Enter a slot count to see the total."
          : `Total: ${total.toLocaleString("en")} ${total === 1 ? "coin" : "coins"} from your wallet.`}
      </p>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <Button
        type="submit"
        className="h-11"
        disabled={pending}
        aria-busy={pending}
      >
        {pending && (
          <LoaderCircle
            className="size-4 animate-spin motion-reduce:animate-none"
            aria-hidden
          />
        )}
        {pending ? "Funding…" : "Fund interview slots"}
      </Button>
    </form>
  );
}
