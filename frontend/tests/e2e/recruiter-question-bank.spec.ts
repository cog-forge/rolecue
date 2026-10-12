import { test, expect, type Page } from "@playwright/test";

const recruiter = {
  id: "22222222-2222-4222-8222-222222222222",
  full_name: "Rec Ruiter",
  email: "rec@example.com",
  image: null,
  role: "recruiter",
  email_verified: true,
  is_locked: false,
};
const postingId = "33333333-3333-4333-8333-333333333333";
const bankPath = `/job-postings/${postingId}/questions`;

async function mockBank(page: Page, options: { editable?: boolean } = {}) {
  const editable = options.editable ?? true;
  let questions: { id: string; content: string }[] = [];
  let next = 1;
  const bodies: unknown[] = [];
  const id = () => `44444444-4444-4444-8444-${String(next++).padStart(12, "0")}`;
  const bank = () => ({
    questions,
    posting_status: editable ? "pending" : "open",
    editable,
    max_question_length: 1000,
    generation_size: 2,
  });
  await page.route("http://localhost:8080/**", async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const headers = {
      "access-control-allow-origin": "http://127.0.0.1:3100",
      "access-control-allow-credentials": "true",
      "access-control-allow-methods": "GET,POST,PATCH,DELETE,OPTIONS",
      "access-control-allow-headers": "Content-Type",
    };
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (url.pathname === "/auth/me")
      return route.fulfill({ headers, json: { success: true, data: recruiter } });
    if (url.pathname === bankPath && method === "GET")
      return route.fulfill({ headers, json: { success: true, data: bank() } });
    if (url.pathname === `${bankPath}/generate` && method === "POST") {
      questions = [
        { id: id(), content: "Explain Go channels." },
        { id: id(), content: "How do you index a table?" },
      ];
      return route.fulfill({ status: 201, headers, json: { success: true, data: { questions } } });
    }
    if (url.pathname === bankPath && method === "POST") {
      const body = route.request().postDataJSON() as { content: string };
      bodies.push(body);
      const question = { id: id(), content: body.content };
      questions = [...questions, question];
      return route.fulfill({ status: 201, headers, json: { success: true, data: question } });
    }
    const match = url.pathname.match(new RegExp(`^${bankPath}/(.+)$`));
    if (match && method === "DELETE") {
      questions = questions.filter((q) => q.id !== match[1]);
      return route.fulfill({ status: 204, headers });
    }
    return route.fulfill({
      status: 404,
      headers,
      json: { success: false, error: { code: "NOT_FOUND", message: "unmocked" } },
    });
  });
  return { bodies };
}

test("recruiter generates the bank, adds and deletes a question", async ({ page }) => {
  const api = await mockBank(page);
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);

  await expect(page.getByRole("heading", { level: 1, name: "Question bank" })).toBeVisible();
  await page.getByRole("button", { name: "Generate questions" }).click();
  await expect(page.getByText("Explain Go channels.")).toBeVisible();
  await expect(page.getByText("2 questions")).toBeVisible();

  const form = page.getByRole("form", { name: "Add a question" });
  await form.getByLabel("Add a question").fill("What is a goroutine leak?");
  await form.getByRole("button", { name: "Add question" }).click();
  await expect(page.getByText("3 questions")).toBeVisible();
  expect(api.bodies).toEqual([{ content: "What is a goroutine leak?" }]);

  await page.getByRole("button", { name: "Delete question 1" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete question" }).click();
  await expect(page.getByText("Explain Go channels.")).toBeHidden();
  await expect(page.getByText("2 questions")).toBeVisible();
});

test("a posting that left review shows a read-only bank", async ({ page }) => {
  await mockBank(page, { editable: false });
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);
  await expect(page.getByText(/so the question bank is read-only/)).toBeVisible();
  await expect(page.getByRole("form", { name: "Add a question" })).toHaveCount(0);
});
