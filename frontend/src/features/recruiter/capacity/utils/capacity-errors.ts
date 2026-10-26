import { apiError } from "@/features/recruiter/job-postings/utils/api-error";

// Server codes from the capacity and close-intake calls, in plain Recruiter language. This is the
// only place these codes are turned into copy.
const messages: Record<string, string> = {
  INSUFFICIENT_BALANCE:
    "Your wallet does not have enough coins for this many slots. Nothing was charged.",
  INTERVIEW_CAPACITY_ALREADY_FUNDED:
    "Interview slots are already funded for this posting. The current count is shown below.",
  INTAKE_NOT_OPEN:
    "Intake is no longer open for this posting, so this cannot be changed.",
};

const defaultFallback = "Something went wrong. Try again.";

// Known codes map to fixed copy; any other code shows the server's own message.
export function capacityErrorMessage(
  error: unknown,
  fallback: string = defaultFallback,
) {
  const { code, message } = apiError(error);
  if (code !== null && Object.hasOwn(messages, code)) return messages[code];
  return message ?? fallback;
}
