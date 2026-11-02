"use client";

import { useId, useMemo } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import { useForm, useWatch } from "react-hook-form";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  createFundCapacitySchema,
  MAX_SLOTS,
  MIN_SLOTS,
  type FundCapacityInput,
  type FundCapacityValues,
} from "../schemas/capacity-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

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
  const fmt = useFormatters();
  const t = useTranslations("recruiter.capacity.fundForm");
  const validationT = useTranslations("recruiter.capacity.validation");
  const schema = useMemo(
    () => createFundCapacitySchema(validationT),
    [validationT],
  );
  const inputId = useId();
  const hintId = useId();
  const errorId = useId();
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<FundCapacityInput, unknown, FundCapacityValues>({
    resolver: zodResolver(schema),
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
      aria-label={t("label")}
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit((values) => onSubmit(values.slots))(event);
      }}
    >
      <div className="space-y-3">
        <label htmlFor={inputId} className="block text-sm font-medium">
          {t("slots")}
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
          {t("hint", {
            min: fmt.number(MIN_SLOTS),
            max: fmt.number(MAX_SLOTS),
            price: fmt.coins(pricePerSlot),
          })}
        </p>
        {slotError && (
          <p id={errorId} role="alert" className="text-xs text-destructive">
            {slotError}
          </p>
        )}
      </div>
      <p className="text-sm text-muted-foreground" aria-live="polite">
        {total === null
          ? t("totalEmpty")
          : t("total", { amount: fmt.coins(total) })}
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
        {pending ? t("funding") : t("submit")}
      </Button>
    </form>
  );
}
