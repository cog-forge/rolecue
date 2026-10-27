import { z } from "zod";

// Mirrors the Go KAN-80 Job Posting contract (snake_case).
export const postingStatuses = [
  "pending",
  "open",
  "intake_closed",
  "closed",
  "rejected",
] as const;
export const postingStatusSchema = z.enum(postingStatuses);
export type PostingStatus = z.infer<typeof postingStatusSchema>;

export const levels = ["intern", "junior", "mid", "senior", "lead"] as const;
export const levelSchema = z.enum(levels);
export type Level = z.infer<typeof levelSchema>;

export const requirementTargets = [
  "programming_language",
  "framework",
  "database",
  "tool",
  "technology",
  "other",
  "technologies",
  "domain_knowledge",
] as const;
export const requirementTypes = ["required", "preferred"] as const;

export const requirementSchema = z.object({
  name: z.string(),
  target: z.enum(requirementTargets),
  type: z.enum(requirementTypes),
});
export type Requirement = z.infer<typeof requirementSchema>;
export type RequirementTarget = Requirement["target"];

const refSchema = z.object({ id: z.uuid(), name: z.string().nullable() });

export const postingSchema = z.object({
  id: z.uuid(),
  title: z.string(),
  status: postingStatusSchema,
  reject_reason: z.string().nullable(),
  // The text read from the JD PDF (or the stored text of a posting created before PDF upload).
  raw_text: z.string(),
  // True when the JD is a PDF, opened through GET /job-postings/:id/document.
  has_document: z.boolean(),
  seniority_level: levelSchema,
  requirements: z.array(requirementSchema),
  refinement_note: z.string(),
  location: z.string().nullable(),
  employment_type: z.string().nullable(),
  salary_min: z.number().int().nullable(),
  salary_max: z.number().int().nullable(),
  interview_duration: z.number().int().nullable(),
  interview_slot: z.number().int().nullable(),
  required_avatar: refSchema.nullable(),
  required_voice: refSchema.nullable(),
  core_question_count: z.number().int().nonnegative(),
  // False while a pending posting is still being prepared and not yet in the Admin queue.
  review_submitted: z.boolean(),
  created_at: z.iso.datetime({ offset: true }),
});
export type JobPosting = z.infer<typeof postingSchema>;

export const summarySchema = z.object({
  id: z.uuid(),
  title: z.string(),
  status: postingStatusSchema,
  reject_reason: z.string().nullable(),
  location: z.string().nullable(),
  employment_type: z.string().nullable(),
  interview_slot: z.number().int().nullable(),
  core_question_count: z.number().int().nonnegative(),
  review_submitted: z.boolean(),
  created_at: z.iso.datetime({ offset: true }),
});
export type PostingSummary = z.infer<typeof summarySchema>;

export const suggestionSchema = z.object({
  title: z.string(),
  seniority_level: levelSchema.nullable(),
  requirements: z.array(requirementSchema),
});
export type Suggestion = z.infer<typeof suggestionSchema>;

// POST /job-postings/documents: the stored PDF and the text read from it.
export const documentSchema = z.object({
  document_id: z.uuid(),
  raw_text: z.string(),
});
export type UploadedDocument = z.infer<typeof documentSchema>;

// GET /job-postings/:id/document: a link to the PDF that expires after a few minutes.
export const documentLinkSchema = z.object({
  url: z.url({ protocol: /^https?$/ }),
  expires_at: z.iso.datetime({ offset: true }),
});

export const voiceSchema = z.object({ id: z.uuid(), name: z.string() });
export type Voice = z.infer<typeof voiceSchema>;

// GET /inventory/avatars: the Recruiter's own interviewer models.
export const avatarSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  personality: z.string(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
export type Avatar = z.infer<typeof avatarSchema>;

const envelope = <T extends z.ZodType>(data: T) =>
  z.object({ success: z.literal(true), data });
const page = <T extends z.ZodType>(item: T) =>
  z.object({
    items: z.array(item),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int(),
    }),
  });

export const postingEnvelopeSchema = envelope(postingSchema);
export const postingPageEnvelopeSchema = envelope(page(summarySchema));
export const suggestionEnvelopeSchema = envelope(suggestionSchema);
export const documentEnvelopeSchema = envelope(documentSchema);
export const documentLinkEnvelopeSchema = envelope(documentLinkSchema);
export const voicePageEnvelopeSchema = envelope(page(voiceSchema));
export const avatarPageEnvelopeSchema = envelope(page(avatarSchema));
export type PostingPage = z.infer<typeof postingPageEnvelopeSchema>["data"];

// Limits match the Go validation; lengths count Unicode code points like Go counts runes.
export const TEXT_MIN = 100;
export const TEXT_MAX = 20_000;
export const PDF_MAX_BYTES = 5 * 1024 * 1024;
export const TITLE_MAX = 200;
export const DETAIL_MAX = 200;
export const NAME_MAX = 200;
export const REQUIREMENTS_MAX = 100;
export const REFINEMENT_NOTE_MAX = 2_000;
const INT32_MAX = 2_147_483_647;

export const runeLength = (value: string) => Array.from(value).length;
const trimmedLength = (value: string) => runeLength(value.trim());

const optionalDetail = (label: string) =>
  z
    .string()
    .refine(
      (value) => trimmedLength(value) <= DETAIL_MAX,
      `Keep the ${label} to ${DETAIL_MAX} characters or fewer.`,
    );

// Whole numbers typed as text; blank means "not set".
const optionalWhole = (message: string, min: number) =>
  z.string().refine((value) => {
    const trimmed = value.trim();
    if (trimmed === "") return true;
    if (!/^\d+$/.test(trimmed)) return false;
    const number = Number(trimmed);
    return number >= min && number <= INT32_MAX;
  }, message);

export const postingFormSchema = z
  .object({
    // document_id is a newly uploaded PDF ("" keeps the posting's current one); raw_text and
    // file_name come from that upload and are shown, never typed. The server reads the PDF again.
    document_id: z.string(),
    file_name: z.string(),
    raw_text: z
      .string()
      .refine(
        (value) => trimmedLength(value) > 0,
        "Upload the job description as a PDF.",
      ),
    title: z
      .string()
      .refine(
        (value) => trimmedLength(value) > 0,
        "Enter a title for the posting.",
      )
      .refine(
        (value) => trimmedLength(value) <= TITLE_MAX,
        `Keep the title to ${TITLE_MAX} characters or fewer.`,
      ),
    seniority_level: z.union([levelSchema, z.literal("")]),
    requirements: z
      .array(
        z.object({
          name: z
            .string()
            .refine(
              (value) => trimmedLength(value) > 0,
              "Name the requirement.",
            )
            .refine(
              (value) => trimmedLength(value) <= NAME_MAX,
              `Keep the name to ${NAME_MAX} characters or fewer.`,
            ),
          target: z.enum(requirementTargets),
          type: z.enum(requirementTypes),
        }),
      )
      .min(
        1,
        "Add at least one requirement. The question bank is built from them.",
      )
      .max(
        REQUIREMENTS_MAX,
        `Keep to ${REQUIREMENTS_MAX} requirements or fewer.`,
      ),
    refinement_note: z
      .string()
      .refine(
        (value) => trimmedLength(value) <= REFINEMENT_NOTE_MAX,
        `Keep the refinement note to ${REFINEMENT_NOTE_MAX} characters or fewer.`,
      ),
    confirmed: z.boolean(),
    location: optionalDetail("location"),
    employment_type: optionalDetail("employment type"),
    salary_min: optionalWhole(
      "Enter the minimum salary as a whole number, 0 or more.",
      0,
    ),
    salary_max: optionalWhole(
      "Enter the maximum salary as a whole number, 0 or more.",
      0,
    ),
    interview_duration: optionalWhole(
      "Enter the interview length as a whole number of minutes, 1 or more.",
      1,
    ),
    required_avatar_id: z
      .string()
      .min(1, "Choose the interviewer model candidates will meet."),
    required_voice_id: z.string().min(1, "Choose the interviewer voice."),
  })
  .superRefine((values, ctx) => {
    if (values.seniority_level === "")
      ctx.addIssue({
        code: "custom",
        path: ["seniority_level"],
        message: "Choose the seniority of the role.",
      });
    const seen = new Set<string>();
    values.requirements.forEach((row, index) => {
      const key = `${row.name.trim().toLowerCase()}\u0000${row.target}`;
      if (row.name.trim() && seen.has(key))
        ctx.addIssue({
          code: "custom",
          path: ["requirements", index, "name"],
          message: "This requirement is already in the list.",
        });
      seen.add(key);
    });
    const min = values.salary_min.trim();
    const max = values.salary_max.trim();
    if (/^\d+$/.test(min) && /^\d+$/.test(max) && Number(min) > Number(max))
      ctx.addIssue({
        code: "custom",
        path: ["salary_max"],
        message: "The maximum salary must be at least the minimum.",
      });
  });
export type PostingValues = z.infer<typeof postingFormSchema>;

export const emptyPostingValues: PostingValues = {
  document_id: "",
  file_name: "",
  raw_text: "",
  title: "",
  seniority_level: "",
  requirements: [],
  refinement_note: "",
  confirmed: false,
  location: "",
  employment_type: "",
  salary_min: "",
  salary_max: "",
  interview_duration: "",
  required_avatar_id: "",
  required_voice_id: "",
};

const numberText = (value: number | null) =>
  value === null ? "" : String(value);

export function postingValues(posting: JobPosting): PostingValues {
  return {
    document_id: "",
    file_name: "",
    raw_text: posting.raw_text,
    title: posting.title,
    seniority_level: posting.seniority_level,
    requirements: posting.requirements.map((row) => ({ ...row })),
    refinement_note: posting.refinement_note,
    // Asked again only if the Recruiter changes the seniority or requirements.
    confirmed: false,
    location: posting.location ?? "",
    employment_type: posting.employment_type ?? "",
    salary_min: numberText(posting.salary_min),
    salary_max: numberText(posting.salary_max),
    interview_duration: numberText(posting.interview_duration),
    required_avatar_id: posting.required_avatar?.id ?? "",
    required_voice_id: posting.required_voice?.id ?? "",
  };
}

const detail = (value: string) => value.trim() || null;
const whole = (value: string) =>
  value.trim() === "" ? null : Number(value.trim());
const requirementRows = (rows: PostingValues["requirements"]) =>
  rows.map((row) => ({
    name: row.name.trim(),
    target: row.target,
    type: row.type,
  }));

// Body for POST /job-postings. Only the fields the Go DTO accepts; it rejects unknown ones.
export function createBody(values: PostingValues) {
  return {
    title: values.title.trim(),
    document_id: values.document_id,
    // Validation already rejected a blank seniority; parsing narrows the type without a cast.
    seniority_level: levelSchema.parse(values.seniority_level),
    requirements: requirementRows(values.requirements),
    refinement_note: values.refinement_note.trim(),
    location: detail(values.location),
    employment_type: detail(values.employment_type),
    salary_min: whole(values.salary_min),
    salary_max: whole(values.salary_max),
    interview_duration: whole(values.interview_duration),
    required_avatar_id: values.required_avatar_id,
    required_voice_id: values.required_voice_id,
  };
}

export type CreatePostingBody = ReturnType<typeof createBody>;
export type PostingPatch = Partial<CreatePostingBody>;

const sameRequirements = (a: Requirement[], b: Requirement[]) =>
  a.length === b.length &&
  a.every(
    (row, index) =>
      row.name === b[index]?.name &&
      row.target === b[index]?.target &&
      row.type === b[index]?.type,
  );

// Body for PATCH /job-postings/:id: only what changed, so a locked field that was not touched is
// never sent. The PDF is only sent when a new one was uploaded (possible only after a rejection).
export function postingPatch(
  values: PostingValues,
  posting: JobPosting,
): PostingPatch {
  const next = createBody(values);
  const patch: PostingPatch = {};
  if (next.title !== posting.title) patch.title = next.title;
  if (next.document_id) patch.document_id = next.document_id;
  if (next.seniority_level !== posting.seniority_level)
    patch.seniority_level = next.seniority_level;
  if (!sameRequirements(next.requirements, posting.requirements))
    patch.requirements = next.requirements;
  if (next.refinement_note !== posting.refinement_note)
    patch.refinement_note = next.refinement_note;
  if (next.location !== posting.location) patch.location = next.location;
  if (next.employment_type !== posting.employment_type)
    patch.employment_type = next.employment_type;
  if (next.salary_min !== posting.salary_min)
    patch.salary_min = next.salary_min;
  if (next.salary_max !== posting.salary_max)
    patch.salary_max = next.salary_max;
  if (next.interview_duration !== posting.interview_duration)
    patch.interview_duration = next.interview_duration;
  if (next.required_avatar_id !== (posting.required_avatar?.id ?? ""))
    patch.required_avatar_id = next.required_avatar_id;
  if (next.required_voice_id !== (posting.required_voice?.id ?? ""))
    patch.required_voice_id = next.required_voice_id;
  return patch;
}

// What the editor may change, following the backend locks.
export function postingLocks(posting: JobPosting) {
  const editable =
    posting.status === "pending" || posting.status === "rejected";
  return {
    editable,
    textEditable: posting.status === "rejected",
    // While pending, the question bank was generated from these; changing them needs the bank
    // deleted first. A rejection clears the bank on the next change.
    requirementsLocked:
      posting.status === "pending" && posting.core_question_count > 0,
  };
}
