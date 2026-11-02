import { useId } from "react";
import { useTranslations } from "next-intl";
import { Controller, useWatch, type UseFormReturn } from "react-hook-form";
import { CalendarClock, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { lockDurations, type LockFormValues } from "../schemas/account-lock";
import { userTimeZone } from "@/lib/i18n/formatters";

export function AccountLockFields({
  form,
  disabled,
}: {
  form: UseFormReturn<LockFormValues>;
  disabled: boolean;
}) {
  const t = useTranslations("admin.accounts.fields");
  const id = useId();
  const [indefinite, duration] = useWatch({
    control: form.control,
    name: ["indefinite", "duration"],
  });
  const errors = form.formState.errors;
  const preset = lockDurations.find((option) => option.value === duration);
  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <label htmlFor={`${id}-reason`} className="text-sm font-medium">
          {t("reasonLabel")}
        </label>
        <Textarea
          id={`${id}-reason`}
          disabled={disabled}
          rows={3}
          maxLength={500}
          placeholder={t("reasonPlaceholder")}
          aria-invalid={!!errors.reason}
          aria-describedby={`${id}-reason-hint${errors.reason ? ` ${id}-reason-error` : ""}`}
          {...form.register("reason")}
        />
        <p id={`${id}-reason-hint`} className="text-xs text-muted-foreground">
          {t("reasonHint")}
        </p>
        {errors.reason && (
          <p
            id={`${id}-reason-error`}
            role="alert"
            className="text-xs text-destructive"
          >
            {errors.reason.message}
          </p>
        )}
      </div>
      <fieldset
        className="space-y-3"
        disabled={disabled || indefinite}
        aria-describedby={`${id}-duration-hint${!indefinite && errors.duration ? ` ${id}-duration-error` : ""}`}
      >
        <legend className="text-sm font-medium">{t("durationLegend")}</legend>
        <p id={`${id}-duration-hint`} className="text-xs text-muted-foreground">
          {t("durationHint")}
        </p>
        <Controller
          control={form.control}
          name="duration"
          render={({ field }) => (
            <div className="grid grid-cols-2 gap-2">
              {lockDurations.map((option, index) => (
                <Button
                  key={option.value}
                  type="button"
                  variant="outline"
                  ref={index === 0 ? field.ref : undefined}
                  aria-pressed={!indefinite && field.value === option.value}
                  disabled={disabled || indefinite}
                  onBlur={field.onBlur}
                  onClick={() => field.onChange(option.value)}
                  className="h-11 justify-between aria-pressed:border-workspace-accent aria-pressed:bg-workspace-accent/10 aria-pressed:text-workspace-accent"
                >
                  {t(`durations.${option.labelKey}`)}
                  {!indefinite && field.value === option.value && (
                    <Check className="size-4" aria-hidden />
                  )}
                </Button>
              ))}
              <Button
                type="button"
                variant="outline"
                aria-pressed={!indefinite && field.value === "custom"}
                disabled={disabled || indefinite}
                onBlur={field.onBlur}
                onClick={() => field.onChange("custom")}
                className="col-span-2 h-11 justify-start aria-pressed:border-workspace-accent aria-pressed:bg-workspace-accent/10 aria-pressed:text-workspace-accent"
              >
                <CalendarClock className="size-4" aria-hidden />
                {t("custom")}
                {!indefinite && field.value === "custom" && (
                  <Check className="ml-auto size-4" aria-hidden />
                )}
              </Button>
            </div>
          )}
        />
        {!indefinite && errors.duration && (
          <p
            id={`${id}-duration-error`}
            role="alert"
            className="text-xs text-destructive"
          >
            {errors.duration.message}
          </p>
        )}
      </fieldset>
      {duration === "custom" && (
        <div className="space-y-2">
          <label htmlFor={`${id}-expiry`} className="text-sm font-medium">
            {t("expiryLabel")}
          </label>
          <Input
            id={`${id}-expiry`}
            className="h-11"
            type="datetime-local"
            disabled={disabled || indefinite}
            aria-invalid={!indefinite && !!errors.expiresAt}
            aria-describedby={`${id}-expiry-hint${!indefinite && errors.expiresAt ? ` ${id}-expiry-error` : ""}`}
            {...form.register("expiresAt")}
          />
          <p id={`${id}-expiry-hint`} className="text-xs text-muted-foreground">
            {t("expiryHint", { timeZone: userTimeZone() })}
          </p>
          {!indefinite && errors.expiresAt && (
            <p
              id={`${id}-expiry-error`}
              role="alert"
              className="text-xs text-destructive"
            >
              {errors.expiresAt.message}
            </p>
          )}
        </div>
      )}
      <div className="space-y-2 border-t pt-4">
        <Controller
          control={form.control}
          name="indefinite"
          render={({ field }) => (
            <label
              htmlFor={`${id}-indefinite`}
              className="flex min-h-11 cursor-pointer items-center gap-3 rounded-lg border p-3 text-sm font-medium"
            >
              <Checkbox
                id={`${id}-indefinite`}
                checked={field.value}
                disabled={disabled}
                ref={field.ref}
                onBlur={field.onBlur}
                onCheckedChange={(value) => field.onChange(value === true)}
                aria-describedby={`${id}-indefinite-hint`}
              />
              {t("indefinite")}
            </label>
          )}
        />
        <p
          id={`${id}-indefinite-hint`}
          className="text-xs text-muted-foreground"
        >
          {t("indefiniteHint")}
        </p>
      </div>
      <p role="status" className="rounded-lg bg-muted/50 p-3 text-sm">
        {indefinite
          ? t("summaryIndefinite")
          : duration === "custom"
            ? t("summaryCustom")
            : preset
              ? t("summaryPreset", {
                  duration: t(`durations.${preset.labelKey}`),
                })
              : t("summaryNone")}
      </p>
    </div>
  );
}
