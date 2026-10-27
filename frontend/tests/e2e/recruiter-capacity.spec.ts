import { test, expect, type Page } from "@playwright/test";

const recruiter = {
  id: "22222222-2222-4222-8222-222222222222",
  full_name: "Rec Ruiter",
  email: "rec@example.com",
  image: null,
  role: "recruiter",
  email_verified: true,
  is_locked: false,
  onboarding_role_selected: true,
  onboarding_completed: true,
};
const postingId = "66666666-6666-4666-8666-666666666666";
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};

type Status = "open" | "intake_closed" | "closed";

function posting(status: Status, interviewSlot: number | null) {
  return {
    id: postingId,
    title: "Backend Engineer",
    status,
    reject_reason: null,
    raw_text: "We build reliable APIs in Go.",
    has_document: false,
    seniority_level: "mid",
    requirements: [],
    refinement_note: "",
    location: "Hanoi",
    employment_type: "Full-time",
    salary_min: null,
    salary_max: null,
    interview_duration: 30,
    interview_slot: interviewSlot,
    required_avatar: null,
    required_voice: null,
    core_question_count: 0,
    review_submitted: true,
    created_at: "2026-10-12T08:00:00Z",
  };
}

function capacity(over: Record<string, unknown> = {}) {
  return {
    interview_slot: null,
    funded: false,
    price_per_slot: 1,
    cv_approvals: 0,
    consumed: 0,
    available_to_approve: 0,
    refundable: 0,
    posting_status: "open",
    refunded: null,
    ...over,
  };
}

type Setup = {
  status?: Status;
  // Current funding; null or omitted means the posting is not funded yet.
  slot?: number | null;
  cvApprovals?: number;
  submitted?: number;
  refunded?: number;
  // Set to make the funding call fail with that error.
  fundError?: { status: number; code: string; message: string };
};

// Serves one posting's Recruiter screens the way Go does: capacity state is kept in memory so a
// reload shows the change, and every funding and close request is recorded.
async function mockCapacity(page: Page, setup: Setup = {}) {
  let status: Status = setup.status ?? "open";
  let slot: number | null = setup.slot ?? null;
  const cvApprovals = setup.cvApprovals ?? 0;
  const submitted = setup.submitted ?? 0;
  const funds: unknown[] = [];
  let closes = 0;

  const currentCapacity = () =>
    capacity({
      interview_slot: slot,
      funded: slot !== null,
      cv_approvals: cvApprovals,
      available_to_approve: slot === null ? 0 : slot - cvApprovals,
      refundable: slot ?? 0,
      posting_status: status,
      refunded: status === "closed" ? (setup.refunded ?? 0) : null,
    });

  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });

    if (url.pathname === "/auth/me") return ok(recruiter);
    if (url.pathname === `/job-postings/${postingId}` && method === "GET")
      return ok(posting(status, slot));
    if (url.pathname === `/job-postings/${postingId}/interview-capacity`) {
      if (method === "GET") return ok(currentCapacity());
      if (method === "POST") {
        const body = request.postDataJSON() as { slots: number };
        funds.push(body);
        if (setup.fundError)
          return route.fulfill({
            status: setup.fundError.status,
            headers,
            json: {
              success: false,
              error: {
                code: setup.fundError.code,
                message: setup.fundError.message,
              },
            },
          });
        slot = body.slots;
        return ok(currentCapacity());
      }
    }
    if (
      url.pathname === `/job-postings/${postingId}/close-intake` &&
      method === "POST"
    ) {
      closes += 1;
      status = "intake_closed";
      return ok({});
    }
    if (url.pathname === `/job-postings/${postingId}/applications`) {
      const items = Array.from({ length: submitted }, (_, i) => ({
        id: `77777777-7777-4777-8777-${String(i + 1).padStart(12, "0")}`,
        job_posting_id: postingId,
        job_posting_title: "Backend Engineer",
        status: "pending",
        submitted_at: "2026-10-20T08:00:00Z",
        candidate: {
          id: `88888888-8888-4888-8888-${String(i + 1).padStart(12, "0")}`,
          name: `Candidate ${i + 1}`,
          email: `cand${i + 1}@example.com`,
          image: null,
        },
        interview_eligible: false,
        interview_deadline: null,
        reject_reason: null,
        updated_at: "2026-10-20T08:00:00Z",
      }));
      return ok({
        items,
        pagination: { limit: 20, offset: 0, total: submitted },
      });
    }
    if (url.pathname === "/voices" || url.pathname === "/inventory/avatars")
      return ok({ items: [], pagination: { limit: 20, offset: 0, total: 0 } });
    return route.fulfill({
      status: 404,
      headers,
      json: {
        success: false,
        error: { code: "NOT_FOUND", message: "unmocked" },
      },
    });
  });
  return { funds, closes: () => closes };
}

async function openPosting(page: Page) {
  await page.goto(`/recruiter/job-postings/${postingId}`);
  await expect(
    page.getByRole("heading", { level: 1, name: "Backend Engineer" }),
  ).toBeVisible();
}

test("an open unfunded posting is funded for the typed slots and then shows the funded state", async ({
  page,
}) => {
  const api = await mockCapacity(page, { submitted: 7 });
  await openPosting(page);

  const form = page.getByRole("form", { name: "Fund interview slots" });
  await form.getByLabel("Interview slots to fund").fill("5");
  await expect(
    form.getByText("Total: 5 coins from your wallet."),
  ).toBeVisible();
  await form.getByRole("button", { name: "Fund interview slots" }).click();

  await expect(page.getByText("Funded slots", { exact: true })).toBeVisible();
  await expect(page.getByText("Still open for approval")).toBeVisible();
  await expect(page.getByLabel("Interview slots to fund")).toHaveCount(0);
  expect(api.funds).toEqual([{ slots: 5 }]);
});

test("the funded state shows slots apart from the submitted CV count", async ({
  page,
}) => {
  await mockCapacity(page, { slot: 5, cvApprovals: 2, submitted: 7 });
  await openPosting(page);

  await expect(page.getByText("Funded slots", { exact: true })).toBeVisible();
  await expect(page.getByText("CVs approved", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Submitted CVs do not use interview slots."),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "CVs submitted", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Fund interview slots" }),
  ).toHaveCount(0);
});

test("a funding the wallet cannot cover shows the plain message and keeps the form", async ({
  page,
}) => {
  const api = await mockCapacity(page, {
    fundError: {
      status: 422,
      code: "INSUFFICIENT_BALANCE",
      message: "insufficient balance",
    },
  });
  await openPosting(page);

  const form = page.getByRole("form", { name: "Fund interview slots" });
  await form.getByLabel("Interview slots to fund").fill("50");
  await form.getByRole("button", { name: "Fund interview slots" }).click();

  await expect(
    page.getByText(
      "Your wallet does not have enough coins for this many slots. Nothing was charged.",
    ),
  ).toBeVisible();
  await expect(form.getByLabel("Interview slots to fund")).toHaveValue("50");
  await expect(
    page.getByRole("button", { name: "Fund interview slots" }),
  ).toBeVisible();
  expect(api.funds).toEqual([{ slots: 50 }]);
});

test("cancelling Close Intake leaves the posting open and calls nothing", async ({
  page,
}) => {
  const api = await mockCapacity(page, { slot: 5 });
  await openPosting(page);

  await page.getByRole("button", { name: "Close intake", exact: true }).click();
  const dialog = page.getByRole("dialog", {
    name: "Close intake for this posting?",
  });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Cancel" }).click();

  await expect(dialog).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Close intake", exact: true }),
  ).toBeVisible();
  expect(api.closes()).toBe(0);
});

test("confirming Close Intake calls the endpoint once and removes the control", async ({
  page,
}) => {
  const api = await mockCapacity(page, { slot: 5 });
  await openPosting(page);

  await page.getByRole("button", { name: "Close intake", exact: true }).click();
  const dialog = page.getByRole("dialog", {
    name: "Close intake for this posting?",
  });
  await dialog
    .getByRole("button", { name: "Close intake", exact: true })
    .click();

  await expect(dialog).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Close intake", exact: true }),
  ).toHaveCount(0);
  expect(api.closes()).toBe(1);
});

test("a closed posting shows the refunded coins as a read-only note with no actions", async ({
  page,
}) => {
  await mockCapacity(page, { status: "closed", slot: 5, refunded: 3 });
  await openPosting(page);

  await expect(
    page.getByText(
      "3 coins were returned to your wallet when this posting closed.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Close intake", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Fund interview slots" }),
  ).toHaveCount(0);
});
