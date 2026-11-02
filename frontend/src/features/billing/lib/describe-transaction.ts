import { createTranslator, type useTranslations } from "next-intl";
import candidate from "@/lib/i18n/messages/en/candidate.json";
import { formatVnd } from "@/lib/i18n/formatters";

// Ledger rows carry the wording the API writes ("Practice interview <id>", "Deposit: Standard
// (100000 VND)"). People read what they did instead; the raw text stays available in details.
// The API wording is matched as stored (English); the labels shown come from the catalog.

export type TransactionKind =
  | "topup"
  | "practice_interview"
  | "interview_slots"
  | "avatar_storage"
  | "refund"
  | "other";

export type TransactionDescription = {
  kind: TransactionKind;
  title: string;
  detail?: string;
};

export type DescribeTranslator = ReturnType<
  typeof useTranslations<"candidate.billing.describe">
>;

// VND keeps Vietnamese conventions in every language, so only the translator varies.
export type DescribeFormatters = { vnd: (amount: number) => string };

// Callers that render in the active language pass their own translator
// (`useTranslations("candidate.billing.describe")`) and `useFormatters()`; without them the labels
// are English.
const englishTranslator: DescribeTranslator = createTranslator({
  locale: "en",
  messages: { candidate },
  namespace: "candidate.billing.describe",
});
const defaultFormatters: DescribeFormatters = { vnd: formatVnd };

function slots(count: string, t: DescribeTranslator) {
  return t("slotCount", { count: Number(count) });
}

function avatarSlots(count: string, t: DescribeTranslator) {
  return t("avatarSlotCount", { count: Number(count) });
}

// "Interview capacity of job posting <id>: <title>, 20 slots". Rows written before the title was
// added have no ": <title>" part and keep the generic wording.
const POSTING_SLOTS =
  "interview capacity of job posting(?:\\s+[0-9a-f-]{36})?(?::\\s*(.+?))?,\\s*(\\d+) slots?$";
const CAPACITY = new RegExp(`^${POSTING_SLOTS}`, "is");
const REFUND = new RegExp(`^Refund of unused ${POSTING_SLOTS}`, "is");

export function describeTransaction(
  description: string | null | undefined,
  t: DescribeTranslator = englishTranslator,
  fmt: DescribeFormatters = defaultFormatters,
): TransactionDescription {
  const text = (description ?? "").trim();
  const deposit = /^Deposit:\s*(.+?)\s*\((\d+)\s*VND\)$/.exec(text);
  if (deposit) {
    return {
      kind: "topup",
      title: /\bpack$/i.test(deposit[1])
        ? deposit[1]
        : t("pack", { name: deposit[1] }),
      detail: fmt.vnd(Number(deposit[2])),
    };
  }
  if (text.startsWith("Practice interview")) {
    return { kind: "practice_interview", title: t("practice") };
  }
  const refund = REFUND.exec(text);
  if (refund) {
    return {
      kind: "refund",
      title: refund[1] ? t("refundNamed", { title: refund[1] }) : t("refund"),
      detail: slots(refund[2], t),
    };
  }
  const capacity = CAPACITY.exec(text);
  if (capacity) {
    return {
      kind: "interview_slots",
      title: capacity[1] ? t("slotsNamed", { title: capacity[1] }) : t("slots"),
      detail: slots(capacity[2], t),
    };
  }
  const avatar = /^Avatar inventory,\s*(\d+) slots?$/.exec(text);
  if (avatar) {
    return {
      kind: "avatar_storage",
      title: t("avatar"),
      detail: avatarSlots(avatar[1], t),
    };
  }
  return { kind: "other", title: text || t("other") };
}
