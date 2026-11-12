import { expect, test, type Page } from "@playwright/test";
const postingId = "33333333-3333-4333-8333-333333333333",
  rid = "22222222-2222-4222-8222-222222222222",
  qid = "44444444-4444-4444-8444-444444444442",
  metric = "77777777-7777-4777-8777-777777777777";
const bankPath = `/job-postings/${postingId}/questions`;
const headers = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,PATCH,OPTIONS",
  "access-control-allow-headers": "Content-Type",
};
async function mock(
  page: Page,
  failure?: "QUESTION_STALE" | "VALIDATION_ERROR",
) {
  let questions = [
    {
      id: "44444444-4444-4444-8444-444444444441",
      content: "Explain channel ownership.",
      rubric_id: rid as string | null,
      expected_answer: "Key points." as string | null,
      grading_incomplete: false,
      grading_issues: [] as string[],
      revision: "a".repeat(64),
    },
    {
      id: qid,
      content: "Explain cancellation.",
      rubric_id: null as string | null,
      expected_answer: null as string | null,
      grading_incomplete: true,
      grading_issues: ["missing_rubric", "missing_expected_answer"],
      revision: "a".repeat(64),
    },
  ];
  let submitted = false,
    fail = failure;
  const writes: {
    previous_revision?: string;
    expected_answer?: string | null;
  }[] = [];
  const rubric = {
    id: rid,
    name: "Mechanism rubric",
    description: "Shared scoring standards",
    question_type: "concept",
    scope: "system",
    is_active: true,
    updated_at: "2026-11-11T00:00:00Z",
    created_at: "2026-11-11T00:00:00Z",
    criteria_count: 1,
    max_points: 3,
    is_own: false,
    can_manage: false,
    locked: true,
    criteria: [
      {
        id: metric,
        metric_id: metric,
        name: "Accuracy",
        max_points: 3,
        scoring_guide: "0 absent; 1 partial; 2 mostly correct; 3 complete.",
      },
    ],
  };
  const posting = () => ({
    id: postingId,
    title: "Backend Engineer",
    status: "pending",
    reject_reason: null,
    raw_text: "Backend role",
    has_document: true,
    seniority_level: "senior",
    requirements: [],
    refinement_note: "",
    review_submitted: submitted,
    location: null,
    employment_type: null,
    salary_min: null,
    salary_max: null,
    interview_duration: null,
    interview_slot: null,
    required_avatar: null,
    required_voice: null,
    core_question_count: questions.length,
    created_at: "2026-11-11T00:00:00Z",
  });
  const bank = () => ({
    questions,
    posting_status: "pending",
    editable: true,
    max_question_length: 1000,
    generation_size: 20,
    review_submitted: submitted,
    grading_ready: questions.every((q) => !q.grading_incomplete),
    grading_incomplete_count: questions.filter((q) => q.grading_incomplete)
      .length,
    max_expected_answer_length: 1000,
    max_expected_answer_words: 100,
    expected_answer_guidance_words: 80,
    max_bulk_questions: 100,
  });
  await page.route("http://localhost:8080/**", async (route) => {
    const req = route.request(),
      url = new URL(req.url()),
      path = url.pathname,
      method = req.method();
    const ok = (data: unknown) =>
      route.fulfill({ headers, json: { success: true, data } });
    if (method === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "/auth/me")
      return ok({
        id: rid,
        full_name: "Review recruiter",
        email: "review@example.com",
        image: null,
        role: "recruiter",
        email_verified: true,
        is_locked: false,
        onboarding_role_selected: true,
        onboarding_completed: true,
      });
    if (path === "/rubrics")
      return ok({
        items: url.searchParams.get("scope") === "own" ? [] : [rubric],
        pagination: { total: url.searchParams.get("scope") === "own" ? 0 : 1 },
      });
    if (path === `/rubrics/${rid}`) return ok(rubric);
    if (path === "/evaluation-metrics")
      return ok({ items: [{ id: metric, name: "Technical Accuracy" }] });
    if (path === `/job-postings/${postingId}/evaluation-weights`)
      return ok({
        configured: false,
        using_defaults: true,
        editable: true,
        posting_status: "pending",
        total: 100,
        weights: [
          { metric_id: metric, name: "Technical Accuracy", percentage: 100 },
        ],
      });
    if (path === `/job-postings/${postingId}/submit`) {
      submitted = true;
      return ok(posting());
    }
    if (path === `/job-postings/${postingId}`) return ok(posting());
    if (path === `/job-postings/${postingId}/interview-capacity`)
      return ok({
        interview_slot: null,
        funded: false,
        price_per_slot: 1,
        cv_approvals: 0,
        consumed: 0,
        available_to_approve: 0,
        refundable: 0,
        posting_status: "pending",
        refunded: null,
      });
    if (path === bankPath && method === "GET") return ok(bank());
    if (path === bankPath && method === "POST") {
      const body = req.postDataJSON() as { content: string };
      const question = {
        ...questions[1],
        id: "44444444-4444-4444-8444-444444444443",
        content: body.content,
        rubric_id: null,
        expected_answer: null,
        grading_incomplete: true,
        grading_issues: ["missing_rubric", "missing_expected_answer"],
        revision: "a".repeat(64),
      };
      questions = [...questions, question];
      return route.fulfill({
        status: 201,
        headers,
        json: { success: true, data: question },
      });
    }
    if (path === `${bankPath}/apply-rubric`) {
      const body = req.postDataJSON() as {
        rubric_id: string;
        question_ids: string[];
      };
      questions = questions.map((q) =>
        body.question_ids.includes(q.id) && q.rubric_id !== body.rubric_id
          ? {
              ...q,
              rubric_id: body.rubric_id,
              expected_answer: null,
              grading_incomplete: true,
              grading_issues: ["missing_expected_answer"],
              revision: "b".repeat(64),
            }
          : q,
      );
      return ok({
        questions: questions.filter((q) => body.question_ids.includes(q.id)),
      });
    }
    if (path.startsWith(bankPath + "/") && method === "PATCH") {
      const body = req.postDataJSON() as {
        rubric_id: string | null;
        expected_answer: string | null;
        previous_revision: string;
      };
      writes.push(body);
      if (fail) {
        const code = fail;
        fail = undefined;
        questions = questions.map((q) =>
          q.id === qid
            ? {
                ...q,
                expected_answer: "Changed elsewhere",
                revision: "c".repeat(64),
              }
            : q,
        );
        return route.fulfill({
          status: 409,
          headers,
          json: {
            success: false,
            error: { code, message: "private server message" },
          },
        });
      }
      const id = path.split("/").at(-1);
      questions = questions.map((q) =>
        q.id === id
          ? {
              ...q,
              ...body,
              grading_incomplete: !body.rubric_id || !body.expected_answer,
              grading_issues: [],
              revision: "d".repeat(64),
            }
          : q,
      );
      return ok(questions.find((q) => q.id === id));
    }
    return ok({ items: [], pagination: { total: 0 } });
  });
  return { writes };
}
for (const locale of ["en", "vi"] as const)
  test(`group, bulk apply, review, draft and submit in ${locale}`, async ({
    page,
  }) => {
    await page.setViewportSize(
      locale === "vi"
        ? { width: 390, height: 844 }
        : { width: 1440, height: 1000 },
    );
    await page
      .context()
      .addCookies([
        { name: "NEXT_LOCALE", value: locale, domain: "127.0.0.1", path: "/" },
      ]);
    await mock(page);
    await page.goto(`/recruiter/job-postings/${postingId}/questions`);
    const vi = locale === "vi";
    await expect(
      page.getByRole("button", {
        name: vi ? "Gửi duyệt" : "Submit for approval",
        exact: true,
      }),
    ).toBeDisabled();
    await page
      .getByRole("checkbox", {
        name: vi ? "Chọn câu 2" : "Select question 2",
        exact: true,
      })
      .check();
    await page
      .getByRole("button", {
        name: vi ? "Đổi barem" : "Change rubric",
        exact: true,
      })
      .click();
    await page.locator("#bulk-rubric").fill("Mechanism");
    await page.getByRole("option", { name: "Mechanism rubric" }).click();
    await page
      .getByRole("button", {
        name: vi ? "Áp dụng barem" : "Apply rubric",
        exact: true,
      })
      .click();
    await expect(
      page.getByText(
        vi
          ? "Đã áp barem. Hãy xem lại những đáp án kỳ vọng vừa bị xóa."
          : "Rubric applied. Review the expected answers that were cleared.",
      ),
    ).toBeVisible();
    await page
      .getByRole("button", {
        name: vi ? "Xem lại các câu vừa đổi" : "Review the updated questions",
      })
      .click();
    await page
      .getByRole("button", {
        name: vi ? "Xem tiêu chí của barem" : "View rubric criteria",
      })
      .click();
    await expect(
      page.getByRole("dialog").getByText("Technical Accuracy", { exact: true }),
    ).toBeVisible();
    await page
      .getByLabel(vi ? "Đáp án kỳ vọng" : "Expected answer", { exact: true })
      .last()
      .fill("Explain cancellation propagation and safe resource cleanup.");
    const save = page
      .getByRole("button", {
        name: vi ? "Lưu nháp" : "Save draft",
        exact: true,
      })
      .last();
    await expect(save).toBeDisabled();
    await page
      .getByRole("checkbox", {
        name: vi
          ? "Tôi đã đối chiếu đáp án với barem được chọn."
          : "I reviewed this answer against the selected rubric.",
      })
      .check();
    await save.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(
      page.getByLabel(vi ? "Đáp án kỳ vọng" : "Expected answer", {
        exact: true,
      }),
    ).toHaveValue(
      "Explain cancellation propagation and safe resource cleanup.",
    );
    await page
      .getByRole("button", {
        name: vi ? "Xem trong nhóm barem" : "View in rubric group",
      })
      .click();
    await expect(page.locator(`#question-${qid}`)).toHaveAttribute(
      "data-highlighted",
      "true",
    );
    await expect(page.locator(`#question-${qid}`)).toBeFocused();
    await expect(
      page.getByText(
        vi ? "Đã đủ cấu hình chấm điểm" : "Grading configuration is complete",
        { exact: true },
      ),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: test
        .info()
        .outputPath(`grading-${locale}-${vi ? "mobile" : "desktop"}.png`),
      fullPage: true,
    });
    await page
      .getByRole("button", {
        name: vi ? "Gửi duyệt" : "Submit for approval",
        exact: true,
      })
      .click();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByText("Technical Accuracy", { exact: true }),
    ).toBeVisible();
    await dialog
      .getByRole("button", {
        name: vi ? "Gửi duyệt" : "Submit for approval",
        exact: true,
      })
      .click();
    await expect(
      page.getByText(vi ? "Đang chờ duyệt" : "In the review queue", {
        exact: true,
      }),
    ).toBeVisible();
  });
test("stale draft requires reload and explicit reapply, with unsaved dialog protection", async ({
  page,
}) => {
  const api = await mock(page, "QUESTION_STALE");
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);
  await page
    .getByRole("button", { name: "Review grading for question 2" })
    .click();
  await page.locator(`#grading-${qid}-answer`).fill("Draft key points.");
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "Leave unsaved changes?" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Keep editing", exact: true }).click();
  await page
    .getByRole("button", { name: "Save draft", exact: true })
    .last()
    .click();
  await expect(
    page.getByText("This question changed in another tab."),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Reload current question", exact: true })
    .click();
  await expect(
    page.getByText("Changed elsewhere", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Keep my draft against this version" })
    .click();
  await expect(page.locator(`#grading-${qid}-answer`)).toHaveValue(
    "Draft key points.",
  );
  await page
    .getByRole("button", { name: "Save draft", exact: true })
    .last()
    .click();
  await expect(page.getByText("Draft saved.", { exact: true })).toBeVisible();
  expect(api.writes.at(-1)?.previous_revision).toBe("c".repeat(64));
});
test("criteria remain keyboard accessible with reduced motion on a small phone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 375, height: 812 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mock(page);
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);
  const trigger = page.getByRole("button", { name: "View rubric criteria" });
  await trigger.focus();
  await page.keyboard.press("Enter");
  await expect(
    page.getByText("Technical Accuracy", { exact: true }),
  ).toBeVisible();
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
  expect(
    await page
      .locator(".t-acc-panel")
      .first()
      .evaluate((el) => getComputedStyle(el).transitionDuration),
  ).toBe("0s");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

for (const width of [1440, 390])
  test(`new question stays in one workspace through rubric assignment and save at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await mock(page);
    await page.goto(`/recruiter/job-postings/${postingId}/questions`);
    await page
      .getByRole("button", { name: "Add a question", exact: true })
      .click();
    const originalDialog = await page.getByRole("dialog").elementHandle();
    await page
      .getByRole("textbox", { name: "Add a question", exact: true })
      .fill("Explain bounded worker pools.");
    await page
      .getByRole("button", { name: "Add question", exact: true })
      .click();
    const editor = page.getByRole("dialog", {
      name: "Review grading for question 3",
    });
    await expect(editor).toBeVisible();
    const newId = "44444444-4444-4444-8444-444444444443";
    const picker = editor.locator(`#grading-${newId}-rubric`);
    await picker.click();
    await picker.fill("no matching rubric");
    await expect(
      page.getByText("No matching rubrics.", { exact: true }),
    ).toBeVisible();
    await picker.press("Escape");
    await expect(editor).toBeVisible();
    await expect(picker).toHaveAttribute("aria-expanded", "false");
    await picker.fill("Mechanism");
    await picker.press("Enter");
    await editor
      .getByLabel("Expected answer", { exact: true })
      .fill("Discuss bounded concurrency and cancellation.");
    await editor
      .getByRole("checkbox", {
        name: "I reviewed this answer against the selected rubric.",
      })
      .check();
    await editor
      .getByRole("button", { name: "Save draft", exact: true })
      .click();
    await expect(
      editor.getByText("Draft saved.", { exact: true }),
    ).toBeVisible();
    expect(await originalDialog?.evaluate((el) => el.isConnected)).toBe(true);
    await expect(
      editor.getByLabel("Expected answer", { exact: true }),
    ).toHaveValue("Discuss bounded concurrency and cancellation.");
    await editor.getByRole("button", { name: "View in rubric group" }).click();
    const row = page.locator(`#question-${newId}`);
    await expect(row).toBeFocused();
    await expect(row).toHaveAttribute("data-highlighted", "true");
    await expect(
      page
        .getByRole("region", { name: "Mechanism rubric" })
        .getByText("Explain bounded worker pools.", { exact: true }),
    ).toBeVisible();
  });

for (const reduced of [false, true])
  test(`posting tabs prefetch data and preserve drafts with reduced motion ${reduced}`, async ({
    page,
  }) => {
    await page.setViewportSize(
      reduced ? { width: 390, height: 844 } : { width: 1440, height: 1000 },
    );
    await page.emulateMedia({
      reducedMotion: reduced ? "reduce" : "no-preference",
    });
    await page.addInitScript(() => {
      if (document.startViewTransition) {
        document.startViewTransition = new Proxy(document.startViewTransition, {
          apply(target, receiver, args) {
            document.documentElement.dataset.testTransitions = String(
              Number(document.documentElement.dataset.testTransitions ?? 0) + 1,
            );
            return Reflect.apply(target, receiver, args);
          },
        });
      }
    });
    await mock(page);
    let prefetched = false;
    page.on("response", (response) => {
      if (
        response
          .url()
          .endsWith(`/job-postings/${postingId}/evaluation-weights`) &&
        response.ok()
      )
        prefetched = true;
    });
    await page.goto(`/recruiter/job-postings/${postingId}/questions`);
    await expect(
      page.getByRole("heading", { name: "Question bank", exact: true }),
    ).toBeVisible();
    await expect.poll(() => prefetched).toBe(true);
    expect(
      await page
        .locator("[data-posting-panel]")
        .evaluate((el) => getComputedStyle(el).viewTransitionName),
    ).toBe("none");
    const nav = page.getByRole("navigation", { name: "Posting sections" });
    await nav.getByRole("link", { name: "Evaluation", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Evaluation criteria", exact: true }),
    ).toBeVisible();
    await expect(
      nav.getByRole("link", { name: "Evaluation", exact: true }),
    ).toHaveAttribute("aria-current", "page");
    await expect
      .poll(() =>
        page.evaluate(
          () =>
            document.documentElement.dataset.postingTransition ?? "finished",
        ),
      )
      .toBe("finished");
    expect(
      await page
        .locator("[data-posting-panel]")
        .evaluate((el) => getComputedStyle(el).viewTransitionName),
    ).toBe("none");
    expect(
      await page.evaluate(() =>
        Number(document.documentElement.dataset.testTransitions ?? 0),
      ),
    ).toBe(reduced ? 0 : 1);
    const indicator = nav.locator(".t-tabs-pill");
    const line = await indicator.boundingBox();
    const link = await nav
      .getByRole("link", { name: "Evaluation", exact: true })
      .boundingBox();
    expect(Math.abs(line!.x - link!.x)).toBeLessThan(1);
    expect(Math.abs(line!.width - link!.width)).toBeLessThan(1);
    if (reduced)
      expect(
        await indicator.evaluate(
          (el) => getComputedStyle(el).transitionDuration,
        ),
      ).toBe("0s");
    await page.getByLabel("Technical Accuracy", { exact: true }).fill("90");
    await nav.getByRole("link", { name: "Questions", exact: true }).click();
    const guard = page.getByRole("dialog", { name: "Leave without saving?" });
    await expect(guard).toBeVisible();
    await guard.getByRole("button", { name: "Stay on this page" }).click();
    await expect(
      page.getByLabel("Technical Accuracy", { exact: true }),
    ).toHaveValue("90");
    await expect(
      nav.getByRole("link", { name: "Evaluation", exact: true }),
    ).toHaveAttribute("aria-current", "page");
    await page.getByLabel("Technical Accuracy", { exact: true }).fill("100");
    await expect(
      page.getByRole("button", { name: "Save criteria", exact: true }),
    ).toBeDisabled();
    await nav.getByRole("link", { name: "Overview", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Backend Engineer", exact: true }),
    ).toBeVisible();
    await nav.getByRole("link", { name: "Questions", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Question bank", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  });

test("a later tab click wins while another tab is still warming", async ({
  page,
}) => {
  await mock(page);
  await page.route(
    `http://localhost:8080/job-postings/${postingId}/evaluation-weights`,
    async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 650));
      await route.fulfill({
        headers,
        json: {
          success: true,
          data: {
            configured: false,
            using_defaults: true,
            editable: true,
            posting_status: "pending",
            total: 100,
            weights: [
              {
                metric_id: metric,
                name: "Technical Accuracy",
                percentage: 100,
              },
            ],
          },
        },
      });
    },
  );
  await page.goto(`/recruiter/job-postings/${postingId}/questions`);
  const nav = page.getByRole("navigation", { name: "Posting sections" });
  await nav.getByRole("link", { name: "Evaluation", exact: true }).click();
  await expect(nav).toHaveAttribute("aria-busy", "true");
  await nav.getByRole("link", { name: "Overview", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Backend Engineer", exact: true }),
  ).toBeVisible();
  await expect(nav).not.toHaveAttribute("aria-busy", "true");
  await expect(page).toHaveURL(new RegExp(`/job-postings/${postingId}$`));
});
