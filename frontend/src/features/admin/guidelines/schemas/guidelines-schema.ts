import { z } from "zod";

// Mirrors api/internal/features/interview/domain.BehaviorGuideline and the Admin response of
// GET/PUT /admin/interview-guidelines (KAN-276). Every field is a closed vocabulary: nothing the Admin
// types ever becomes prompt text.
export const tones = ["neutral", "friendly", "formal", "challenging"] as const;
export const questionLengths = ["standard", "concise"] as const;
export const followUpFocuses = ["standard", "vague_only", "thorough"] as const;
export const avoidTopics = [
  "compensation",
  "personal_life",
  "politics_religion",
  "former_employers",
  "location_nationality",
] as const;
export const MAX_FOLLOW_UPS = 3;

export type Tone = (typeof tones)[number];
export type QuestionLength = (typeof questionLengths)[number];
export type FollowUpFocus = (typeof followUpFocuses)[number];
export type AvoidTopic = (typeof avoidTopics)[number];

export const guidelineValuesSchema = z.object({
  tone: z.enum(tones),
  question_length: z.enum(questionLengths),
  // null = use the configured limit; a number can only lower it.
  follow_up_max: z.number().int().min(0).max(MAX_FOLLOW_UPS).nullable(),
  follow_up_focus: z.enum(followUpFocuses),
  avoid_topics: z.array(z.enum(avoidTopics)).max(avoidTopics.length),
});
export type GuidelineValues = z.infer<typeof guidelineValuesSchema>;

export const guidelinesSchema = z.object({
  // 0 means nothing was saved yet and `values` are the defaults.
  version: z.number().int().nonnegative(),
  values: guidelineValuesSchema,
  defaults: guidelineValuesSchema,
  effective_follow_up_ceiling: z.number().int().nonnegative(),
  updated_at: z.iso.datetime({ offset: true }).nullable(),
  updated_by_name: z.string(),
});
export type Guidelines = z.infer<typeof guidelinesSchema>;

export const guidelinesEnvelopeSchema = z.object({
  success: z.literal(true),
  data: guidelinesSchema,
});

// What the form edits. The follow-up maximum is a radio choice, so it is a string here.
export const followUpMaxChoices = ["default", "0", "1", "2", "3"] as const;
export type FollowUpMaxChoice = (typeof followUpMaxChoices)[number];

export const guidelineFormSchema = z.object({
  tone: z.enum(tones),
  question_length: z.enum(questionLengths),
  follow_up_max: z.enum(followUpMaxChoices),
  follow_up_focus: z.enum(followUpFocuses),
  avoid_topics: z.array(z.enum(avoidTopics)),
});
export type GuidelineFormValues = z.infer<typeof guidelineFormSchema>;

export function toFormValues(values: GuidelineValues): GuidelineFormValues {
  return {
    tone: values.tone,
    question_length: values.question_length,
    follow_up_max:
      values.follow_up_max === null
        ? "default"
        : (String(values.follow_up_max) as FollowUpMaxChoice),
    follow_up_focus: values.follow_up_focus,
    // The catalog order, so two equal selections compare equal.
    avoid_topics: avoidTopics.filter((topic) =>
      values.avoid_topics.includes(topic),
    ),
  };
}

export function toRequestValues(form: GuidelineFormValues): GuidelineValues {
  return {
    tone: form.tone,
    question_length: form.question_length,
    follow_up_max:
      form.follow_up_max === "default" ? null : Number(form.follow_up_max),
    follow_up_focus: form.follow_up_focus,
    avoid_topics: avoidTopics.filter((topic) =>
      form.avoid_topics.includes(topic),
    ),
  };
}

// The follow-ups per question the interviewer will really ask: the server's ceiling, lowered by the
// guideline, never raised by it.
export function effectiveFollowUps(
  choice: FollowUpMaxChoice,
  ceiling: number,
): number {
  return choice === "default" ? ceiling : Math.min(Number(choice), ceiling);
}
