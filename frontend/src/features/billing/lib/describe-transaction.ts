// Ledger rows carry the wording the API writes ("Practice interview <id>", "Deposit: Standard
// (100000 VND)"). People read what they did instead; the raw text stays available in details.

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

function formatVND(amount: number) {
  return `${String(amount).replace(/\B(?=(\d{3})+(?!\d))/g, ".")} ₫`;
}

function slots(count: string) {
  return `${count} ${count === "1" ? "slot" : "slots"}`;
}

// "Interview capacity of job posting <id>: <title>, 20 slots". Rows written before the title was
// added have no ": <title>" part and keep the generic wording.
const POSTING_SLOTS =
  "interview capacity of job posting(?:\\s+[0-9a-f-]{36})?(?::\\s*(.+?))?,\\s*(\\d+) slots?$";
const CAPACITY = new RegExp(`^${POSTING_SLOTS}`, "is");
const REFUND = new RegExp(`^Refund of unused ${POSTING_SLOTS}`, "is");

export function describeTransaction(
  description: string | null | undefined,
): TransactionDescription {
  const text = (description ?? "").trim();
  const deposit = /^Deposit:\s*(.+?)\s*\((\d+)\s*VND\)$/.exec(text);
  if (deposit) {
    return {
      kind: "topup",
      title: /\bpack$/i.test(deposit[1]) ? deposit[1] : `${deposit[1]} pack`,
      detail: formatVND(Number(deposit[2])),
    };
  }
  if (text.startsWith("Practice interview")) {
    return { kind: "practice_interview", title: "Practice interview" };
  }
  const refund = REFUND.exec(text);
  if (refund) {
    return {
      kind: "refund",
      title: refund[1]
        ? `Refund for unused slots: ${refund[1]}`
        : "Refund for unused interview slots",
      detail: slots(refund[2]),
    };
  }
  const capacity = CAPACITY.exec(text);
  if (capacity) {
    return {
      kind: "interview_slots",
      title: capacity[1]
        ? `Interview slots: ${capacity[1]}`
        : "Interview slots",
      detail: slots(capacity[2]),
    };
  }
  const avatar = /^Avatar inventory,\s*(\d+) slots?$/.exec(text);
  if (avatar) {
    return {
      kind: "avatar_storage",
      title: "Avatar storage",
      detail: slots(avatar[1]),
    };
  }
  return { kind: "other", title: text || "Coin transaction" };
}
