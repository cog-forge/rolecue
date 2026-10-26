"use client";

import { Controller, useWatch, type UseFormReturn } from "react-hook-form";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { categoryLabels, type RoleReview } from "../api/contract";

export function RequirementDetails({
  form,
  index,
  disabled,
  customTarget,
}: {
  form: UseFormReturn<RoleReview>;
  index: number;
  disabled: boolean;
  customTarget: boolean;
}) {
  const prefix = `requirement-${index}`;
  const error = form.formState.errors.requirements?.[index];
  const kind = useWatch({
    control: form.control,
    name: `requirements.${index}.kind`,
  });
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <div className="space-y-2">
        <label
          htmlFor={`${prefix}-kind`}
          className="text-xs text-muted-foreground"
        >
          Kind
        </label>
        <Controller
          control={form.control}
          name={`requirements.${index}.kind`}
          render={({ field }) => (
            <Select
              value={field.value}
              disabled={disabled}
              onValueChange={(
                value: RoleReview["requirements"][number]["kind"],
              ) => {
                field.onChange(value);
                form.setValue(
                  `requirements.${index}.category`,
                  value === "skill" ? "other" : null,
                  { shouldDirty: true, shouldValidate: true },
                );
              }}
            >
              <SelectTrigger
                id={`${prefix}-kind`}
                ref={field.ref}
                onBlur={field.onBlur}
                className="w-full data-[size=default]:h-11"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem value="skill">Technical skill</SelectItem>
                <SelectItem value="technology">Technology</SelectItem>
                <SelectItem value="domain_knowledge">
                  Domain knowledge
                </SelectItem>
              </SelectContent>
            </Select>
          )}
        />
      </div>
      {kind === "skill" && (
        <div className="space-y-2">
          <label
            htmlFor={`${prefix}-category`}
            className="text-xs text-muted-foreground"
          >
            Skill category
          </label>
          <Controller
            control={form.control}
            name={`requirements.${index}.category`}
            render={({ field }) => (
              <Select
                value={field.value ?? undefined}
                disabled={disabled}
                onValueChange={field.onChange}
              >
                <SelectTrigger
                  id={`${prefix}-category`}
                  ref={field.ref}
                  onBlur={field.onBlur}
                  className="w-full data-[size=default]:h-11"
                  aria-invalid={Boolean(error?.category)}
                >
                  <SelectValue placeholder="Select a category" />
                </SelectTrigger>
                <SelectContent position="popper" align="start">
                  {Object.entries(categoryLabels).map(([value, label]) => (
                    <SelectItem key={value} value={value}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
      )}
      <div className="space-y-2">
        <label
          htmlFor={`${prefix}-type`}
          className="text-xs text-muted-foreground"
        >
          Priority
        </label>
        <Controller
          control={form.control}
          name={`requirements.${index}.type`}
          render={({ field }) => (
            <Select
              value={field.value ?? "unspecified"}
              disabled={disabled}
              onValueChange={(value) =>
                field.onChange(value === "unspecified" ? null : value)
              }
            >
              <SelectTrigger
                id={`${prefix}-type`}
                ref={field.ref}
                onBlur={field.onBlur}
                className="w-full data-[size=default]:h-11"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem value="unspecified">Unspecified</SelectItem>
                <SelectItem value="required">Required</SelectItem>
                <SelectItem value="preferred">Preferred</SelectItem>
              </SelectContent>
            </Select>
          )}
        />
      </div>
      {customTarget && (
        <div className="space-y-2 sm:col-span-2 xl:col-span-3">
          <label
            htmlFor={`${prefix}-saved-target`}
            className="text-xs text-muted-foreground"
          >
            Saved target
          </label>
          <Input
            id={`${prefix}-saved-target`}
            {...form.register(`requirements.${index}.target`)}
            className="h-11"
            aria-invalid={Boolean(error?.target)}
          />
          <p className="text-xs text-muted-foreground">
            This role has a target saved in the earlier format. It stays as
            written until you change it.
          </p>
        </div>
      )}
    </div>
  );
}
