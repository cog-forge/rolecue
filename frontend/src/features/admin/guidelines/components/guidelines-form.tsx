"use client";

import { Controller, useForm, useWatch } from "react-hook-form";
import { useTranslations } from "next-intl";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  ArrowUpRight,
  BriefcaseBusiness,
  Check,
  CircleAlert,
  Crosshair,
  Info,
  LoaderCircle,
  MessageCircle,
  Minus,
  RotateCcw,
  Save,
  ShieldCheck,
  Smile,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  avoidTopics,
  effectiveFollowUps,
  followUpFocuses,
  followUpMaxChoices,
  guidelineFormSchema,
  questionLengths,
  tones,
  toFormValues,
  toRequestValues,
  type Guidelines,
  type GuidelineValues,
} from "../schemas/guidelines-schema";
import { ChoiceCard } from "./choice-card";
import { GuidelineField } from "./guideline-field";

const actionClass = "h-11 rounded-full px-5";
const toneIcons = {
  neutral: Minus,
  friendly: Smile,
  formal: BriefcaseBusiness,
  challenging: Crosshair,
};

export function GuidelinesForm({
  guidelines,
  saving,
  error,
  onSave,
  onRequestReset,
}: {
  guidelines: Guidelines;
  saving: boolean;
  error: string | null;
  onSave: (values: GuidelineValues) => void;
  onRequestReset: () => void;
}) {
  const t = useTranslations("admin.interviewGuidelines");
  const tFields = useTranslations("admin.interviewGuidelines.fields");
  const form = useForm({
    resolver: zodResolver(guidelineFormSchema),
    defaultValues: toFormValues(guidelines.values),
  });
  const dirty = form.formState.isDirty;
  const values = useWatch({ control: form.control });
  const maxChoice = values.follow_up_max ?? "default";
  const tone = values.tone ?? guidelines.values.tone;
  const focus = values.follow_up_focus ?? guidelines.values.follow_up_focus;
  const length = values.question_length ?? guidelines.values.question_length;
  const topics = values.avoid_topics ?? [];
  const effective = effectiveFollowUps(
    maxChoice,
    guidelines.effective_follow_up_ceiling,
  );
  const capped =
    maxChoice !== "default" &&
    Number(maxChoice) > guidelines.effective_follow_up_ceiling;

  return (
    <form
      noValidate
      aria-busy={saving}
      onSubmit={form.handleSubmit((data) => {
        if (!saving) onSave(toRequestValues(data));
      })}
      className="space-y-5"
    >
      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_17rem]">
        <div className="divide-y rounded-2xl border bg-card">
          <section
            aria-labelledby="style-heading"
            className="space-y-6 p-5 sm:p-7"
          >
            <div className="flex items-start gap-3">
              <span
                aria-hidden
                className="pt-0.5 font-mono text-xs text-muted-foreground"
              >
                01
              </span>
              <div>
                <h2
                  id="style-heading"
                  className="text-lg font-semibold tracking-tight"
                >
                  {t("sections.style.title")}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("sections.style.description")}
                </p>
              </div>
            </div>
            <GuidelineField name="tone" legend={tFields("tone.label")}>
              <Controller
                control={form.control}
                name="tone"
                render={({ field }) => (
                  <RadioGroup
                    aria-labelledby="tone-legend"
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={saving}
                    className="grid-cols-2 gap-2"
                  >
                    {tones.map((value) => {
                      const Icon = toneIcons[value];
                      return (
                        <ChoiceCard
                          key={value}
                          htmlFor={`tone-${value}`}
                          title={tFields(`tone.options.${value}.label`)}
                          icon={<Icon className="size-4" />}
                          control={
                            <RadioGroupItem
                              id={`tone-${value}`}
                              value={value}
                            />
                          }
                        />
                      );
                    })}
                  </RadioGroup>
                )}
              />
              <p className="min-h-4 text-xs text-muted-foreground">
                {tFields(`tone.options.${tone}.description`)}
              </p>
            </GuidelineField>
            <GuidelineField
              name="focus"
              legend={tFields("followUpFocus.label")}
            >
              <Controller
                control={form.control}
                name="follow_up_focus"
                render={({ field }) => (
                  <RadioGroup
                    aria-labelledby="focus-legend"
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={saving}
                    className="gap-2 sm:grid-cols-3"
                  >
                    {followUpFocuses.map((value) => (
                      <ChoiceCard
                        key={value}
                        htmlFor={`focus-${value}`}
                        title={tFields(`followUpFocus.options.${value}.label`)}
                        control={
                          <RadioGroupItem id={`focus-${value}`} value={value} />
                        }
                      />
                    ))}
                  </RadioGroup>
                )}
              />
              <p className="min-h-4 text-xs leading-relaxed text-muted-foreground">
                {tFields(`followUpFocus.options.${focus}.description`)}
              </p>
            </GuidelineField>
          </section>

          <section
            aria-labelledby="limits-heading"
            className="space-y-6 p-5 sm:p-7"
          >
            <div className="flex items-start gap-3">
              <span
                aria-hidden
                className="pt-0.5 font-mono text-xs text-muted-foreground"
              >
                02
              </span>
              <div>
                <h2
                  id="limits-heading"
                  className="text-lg font-semibold tracking-tight"
                >
                  {t("sections.limits.title")}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("sections.limits.description")}
                </p>
              </div>
            </div>
            <GuidelineField
              name="length"
              legend={tFields("questionLength.label")}
            >
              <Controller
                control={form.control}
                name="question_length"
                render={({ field }) => (
                  <RadioGroup
                    aria-labelledby="length-legend"
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={saving}
                    className="grid-cols-2 gap-2"
                  >
                    {questionLengths.map((value) => (
                      <ChoiceCard
                        key={value}
                        htmlFor={`length-${value}`}
                        title={tFields(`questionLength.options.${value}.label`)}
                        description={tFields(
                          `questionLength.options.${value}.description`,
                        )}
                        control={
                          <RadioGroupItem
                            id={`length-${value}`}
                            value={value}
                          />
                        }
                      />
                    ))}
                  </RadioGroup>
                )}
              />
            </GuidelineField>
            <GuidelineField name="max" legend={tFields("followUpMax.label")}>
              <Controller
                control={form.control}
                name="follow_up_max"
                render={({ field }) => (
                  <RadioGroup
                    aria-labelledby="max-legend"
                    aria-describedby="max-effective"
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={saving}
                    className="grid-cols-[minmax(0,2fr)_repeat(4,minmax(0,1fr))] gap-1.5"
                  >
                    {followUpMaxChoices.map((value) => (
                      <label
                        key={value}
                        htmlFor={`max-${value}`}
                        className="relative flex min-h-11 cursor-pointer items-center justify-center rounded-lg border px-1 text-center text-sm font-medium transition-[background-color,border-color,color] duration-[var(--duration-quick)] has-[[data-state=checked]]:border-foreground has-[[data-state=checked]]:bg-foreground has-[[data-state=checked]]:text-background has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-ring/50 has-disabled:cursor-not-allowed has-disabled:opacity-60 hover:bg-muted motion-reduce:transition-none"
                      >
                        <RadioGroupItem
                          id={`max-${value}`}
                          aria-label={tFields(`followUpMax.options.${value}`)}
                          value={value}
                          className="absolute inset-0 size-full opacity-0 after:hidden"
                        />
                        <span className="pointer-events-none">
                          {value === "0"
                            ? "0"
                            : tFields(`followUpMax.options.${value}`)}
                        </span>
                      </label>
                    ))}
                  </RadioGroup>
                )}
              />
              <p
                id="max-effective"
                aria-live="polite"
                className="text-xs leading-relaxed text-muted-foreground"
              >
                {tFields("followUpMax.effective", { count: effective })}
                {capped &&
                  ` ${tFields("followUpMax.capped", { ceiling: guidelines.effective_follow_up_ceiling, chosen: maxChoice })}`}
              </p>
            </GuidelineField>
          </section>

          <section
            aria-labelledby="topics-heading"
            className="space-y-5 p-5 sm:p-7"
          >
            <div className="flex items-start gap-3">
              <span
                aria-hidden
                className="pt-0.5 font-mono text-xs text-muted-foreground"
              >
                03
              </span>
              <div className="min-w-0 flex-1">
                <h2
                  id="topics-heading"
                  className="text-lg font-semibold tracking-tight"
                >
                  {tFields("avoidTopics.label")}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {tFields("avoidTopics.help")}
                </p>
              </div>
              <span
                aria-hidden
                className="rounded-md bg-muted px-2 py-1 font-mono text-xs tabular-nums"
              >
                {topics.length}/{avoidTopics.length}
              </span>
            </div>
            <Controller
              control={form.control}
              name="avoid_topics"
              render={({ field }) => (
                <div
                  role="group"
                  aria-labelledby="topics-heading"
                  className="grid gap-2 sm:grid-cols-2"
                >
                  {avoidTopics.map((topic) => (
                    <ChoiceCard
                      key={topic}
                      htmlFor={`topic-${topic}`}
                      title={tFields(`avoidTopics.options.${topic}`)}
                      className="py-2.5"
                      control={
                        <Checkbox
                          id={`topic-${topic}`}
                          checked={field.value.includes(topic)}
                          disabled={saving}
                          onCheckedChange={(checked) =>
                            field.onChange(
                              avoidTopics.filter((item) =>
                                item === topic
                                  ? checked === true
                                  : field.value.includes(item),
                              ),
                            )
                          }
                        />
                      }
                    />
                  ))}
                </div>
              )}
            />
          </section>
        </div>

        <aside
          aria-label={t("summary.title")}
          className="space-y-4 xl:sticky xl:top-6"
        >
          <Card className="hidden gap-0 rounded-2xl border py-0 shadow-none ring-0 xl:flex">
            <CardContent className="space-y-5 p-5">
              <div className="flex items-center justify-between gap-2">
                <h2 className="text-sm font-semibold">{t("summary.title")}</h2>
                <MessageCircle
                  className="size-4 text-workspace-accent"
                  aria-hidden
                />
              </div>
              <div className="rounded-xl bg-muted/60 p-4">
                <div className="flex items-baseline gap-2">
                  <span className="text-5xl font-semibold tracking-tighter tabular-nums">
                    {effective}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {t("summary.perQuestion")}
                  </span>
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  {t("summary.ceiling", {
                    count: guidelines.effective_follow_up_ceiling,
                  })}
                </p>
              </div>
              <dl className="space-y-3 text-sm">
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">
                    {tFields("tone.label")}
                  </dt>
                  <dd className="text-right font-medium">
                    {tFields(`tone.options.${tone}.label`)}
                  </dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">
                    {tFields("followUpFocus.label")}
                  </dt>
                  <dd className="text-right font-medium">
                    {tFields(`followUpFocus.options.${focus}.label`)}
                  </dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">
                    {tFields("questionLength.label")}
                  </dt>
                  <dd className="text-right font-medium">
                    {t("summary.characters", {
                      count: length === "concise" ? 150 : 300,
                    })}
                  </dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">
                    {tFields("avoidTopics.label")}
                  </dt>
                  <dd className="text-right font-medium">
                    {t("summary.topics", { count: topics.length })}
                  </dd>
                </div>
              </dl>
              <Separator />
              <p className="flex items-start gap-2 text-xs leading-relaxed text-muted-foreground">
                <ArrowUpRight
                  className="mt-0.5 size-3.5 shrink-0"
                  aria-hidden
                />
                {t("summary.appliesTo")}
              </p>
            </CardContent>
          </Card>
          <Accordion type="single" collapsible className="px-1">
            <AccordionItem value="scope">
              <AccordionTrigger className="min-h-11 items-center gap-2 text-xs hover:no-underline">
                <Info
                  className="size-3.5 shrink-0 text-muted-foreground"
                  aria-hidden
                />
                <span className="flex-1">{t("promptNote.title")}</span>
              </AccordionTrigger>
              <AccordionContent className="space-y-3 pt-2 text-xs leading-relaxed text-muted-foreground">
                <p>{t("promptNote.body")}</p>
                <p className="flex items-start gap-2">
                  <ShieldCheck
                    className="mt-0.5 size-3.5 shrink-0"
                    aria-hidden
                  />
                  {t("summary.protected")}
                </p>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </aside>
      </div>

      {error && (
        <p
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-destructive/30 bg-card p-4 text-sm"
        >
          <CircleAlert
            className="mt-0.5 size-4 shrink-0 text-destructive"
            aria-hidden
          />
          {error}
        </p>
      )}

      <div className="sticky bottom-3 z-10 flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-background p-3 shadow-[0_4px_20px_-8px_rgb(0_0_0/0.18)] sm:bottom-4 sm:px-4">
        <div
          className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground"
          aria-live="polite"
        >
          {dirty ? (
            <span
              aria-hidden
              className="size-1.5 rounded-full bg-workspace-accent"
            />
          ) : (
            <Check className="size-3.5" aria-hidden />
          )}
          {dirty ? t("status.dirty") : t("status.clean")}
        </div>
        <div className="flex items-center gap-1 sm:gap-2">
          <Button
            type="button"
            variant="ghost"
            className="h-11 rounded-full px-3 text-xs"
            disabled={saving}
            onClick={onRequestReset}
          >
            <RotateCcw className="size-3.5" aria-hidden />
            <span className="sr-only sm:not-sr-only">{t("actions.reset")}</span>
          </Button>
          <Button
            type="button"
            variant="ghost"
            className="h-11 rounded-full px-3 text-xs"
            disabled={!dirty || saving}
            onClick={() => form.reset(toFormValues(guidelines.values))}
          >
            {t("actions.discard")}
          </Button>
          <Button
            type="submit"
            className={`${actionClass} text-xs`}
            disabled={!dirty || saving}
          >
            <span
              className="t-icon-swap"
              data-state={saving ? "b" : "a"}
              aria-hidden
            >
              <Save className="t-icon size-3.5" data-icon="a" />
              <LoaderCircle
                className="t-icon size-3.5 animate-spin motion-reduce:animate-none"
                data-icon="b"
              />
            </span>
            {saving ? t("actions.saving") : t("actions.save")}
          </Button>
        </div>
      </div>
    </form>
  );
}
