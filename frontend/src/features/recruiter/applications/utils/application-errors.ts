import { apiError } from "@/features/recruiter/job-postings/utils/api-error";

// Server error codes for the screening actions and detail reads, in plain Recruiter language.
const messages: Record<string, string> = {
  APPLICATION_NOT_PENDING:
    "This application has already been decided. Refresh to see its current status.",
  INTAKE_NOT_OPEN: "This posting is no longer accepting applications.",
  INTERVIEW_CAPACITY_FULL:
    "All interview slots for this posting are already taken.",
  INTERVIEW_CAPACITY_NOT_FUNDED:
    "Interview capacity is not funded for this posting.",
  APPLICATION_NOT_FOUND: "This application could not be found.",
  JOB_POSTING_NOT_FOUND: "This job posting could not be found.",
};

const defaultFallback = "Something went wrong. Try again.";

// Known codes map to fixed copy; any other code shows the server's own message.
export function applicationErrorMessage(
  error: unknown,
  fallback: string = defaultFallback,
) {
  const { code, message } = apiError(error);
  if (code !== null && Object.hasOwn(messages, code)) return messages[code];
  return message ?? fallback;
}
