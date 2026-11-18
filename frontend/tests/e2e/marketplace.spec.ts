import { expect, test } from "@playwright/test";
import { mockMarketplace } from "./fixtures/marketplace-api";
import {
  avatarId,
  listingId,
  nextVersion,
  version,
} from "../fixtures/marketplace";
for (const role of ["candidate", "recruiter"] as const) {
  test(`${role} browses, searches and reads a listing without accessing a model`, async ({
    page,
  }) => {
    const calls = await mockMarketplace(page, { role });
    await page.goto("/marketplace");
    await expect(
      page.getByRole("heading", { name: "Refund Bot" }),
    ).toBeVisible();
    await expect(page.getByText("Image preview unavailable")).toBeVisible();
    await page.getByRole("textbox", { name: "Search avatars" }).fill("Refund");
    await page.getByRole("button", { name: "Search", exact: true }).click();
    await expect(page).toHaveURL(/q=Refund/);
    await page.getByRole("combobox", { name: "Sort by" }).click();
    await page.getByRole("option", { name: "Price: low to high" }).click();
    await expect(page).toHaveURL(/sort=price_asc/);
    await page.getByRole("heading", { name: "Refund Bot" }).click();
    await expect(page).toHaveURL(new RegExp(`/marketplace/${listingId}$`));
    await expect(
      page.getByRole("button", { name: "Buy for 20 coins" }),
    ).toBeVisible();
    expect(
      calls.reads.some((path) => path.startsWith("/inventory/avatars/")),
    ).toBe(false);
  });
}
test("a lost purchase reply recovers the same receipt without charging again", async ({
  page,
}) => {
  const calls = await mockMarketplace(page, { lostReply: true });
  await page.goto(`/marketplace/${listingId}`);
  await page.getByRole("button", { name: "Buy for 20 coins" }).click();
  await page
    .getByRole("button", { name: "Confirm purchase · 20 coins" })
    .click();
  await expect(
    page.getByText(/We could not confirm the outcome/),
  ).toBeVisible();
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "This purchase was already processed" }),
  ).toBeVisible();
  const purchases = calls.writes.filter((x) => x.path.endsWith("/purchase"));
  expect(purchases).toHaveLength(2);
  expect(purchases[0]).toEqual(purchases[1]);
  expect(purchases[0].key).toMatch(/^mkt1:/);
  await page.getByRole("link", { name: "Open my collection" }).click();
  await expect(page.getByRole("heading", { name: "Refund Bot" })).toBeVisible();
});
test("changed prices require another explicit confirmation and a new intent", async ({
  page,
}) => {
  const calls = await mockMarketplace(page, {
    purchaseCode: "MARKETPLACE_PRICE_CHANGED",
  });
  await page.goto(`/marketplace/${listingId}`);
  await page.getByRole("button", { name: "Buy for 20 coins" }).click();
  await page
    .getByRole("button", { name: "Confirm purchase · 20 coins" })
    .click();
  await expect(page.getByText(/The price changed/)).toBeVisible();
  await page.getByRole("button", { name: "Reload listing" }).click();
  await page.getByRole("button", { name: "Buy for 30 coins" }).click();
  await page
    .getByRole("button", { name: "Confirm purchase · 30 coins" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Avatar added to your collection" }),
  ).toBeVisible();
  expect(calls.writes[0].body).toEqual({ price_coins: 20 });
  expect(calls.writes[1].body).toEqual({ price_coins: 30 });
  expect(calls.writes[0].key).not.toBe(calls.writes[1].key);
});
test("full inventory uses the existing capacity dialog and does not implicitly buy slots", async ({
  page,
}) => {
  const calls = await mockMarketplace(page, { full: true });
  await page.goto(`/marketplace/${listingId}`);
  await page.getByRole("button", { name: "Buy for 20 coins" }).click();
  await expect(
    page.getByRole("button", { name: "Confirm purchase · 20 coins" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Buy more slots" }).click();
  await expect(
    page.getByRole("dialog", { name: "Buy avatar slots" }),
  ).toBeVisible();
  expect(calls.writes).toHaveLength(0);
});
test("insufficient balance opens the existing top-up flow", async ({
  page,
}) => {
  const calls = await mockMarketplace(page, { balance: 5 });
  await page.goto(`/marketplace/${listingId}`);
  await page.getByRole("button", { name: "Buy for 20 coins" }).click();
  await expect(
    page.getByRole("button", { name: "Confirm purchase · 20 coins" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Add coins", exact: true }).click();
  await expect(page.getByRole("dialog").last()).toContainText("Starter");
  expect(calls.writes).toHaveLength(0);
});
for (const owned of [true, false])
  test(`owned=${owned} and own listings have no buy button`, async ({
    page,
  }) => {
    await mockMarketplace(page, { owned, own: !owned });
    await page.goto(`/marketplace/${listingId}`);
    await expect(
      page.getByRole("heading", { name: "Refund Bot" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: /Buy for/ })).toHaveCount(0);
  });
test("seller saves, submits and sees the frozen payload; unsaved changes are guarded", async ({
  page,
}) => {
  const calls = await mockMarketplace(page);
  await page.goto(`/marketplace/my-listings/new?avatar=${avatarId}`);
  await page.getByLabel("Listing title").fill("Refund Bot");
  await page.getByRole("link", { name: "My listings", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: "Discard unsaved changes?" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Keep editing" }).click();
  await page.getByLabel("Interviewer personality").fill("Calm and precise.");
  await page.getByLabel("Rights declaration").fill("I own commercial rights.");
  await page.getByLabel("Price in coins").fill("20");
  await page.getByRole("button", { name: "Save draft" }).click();
  await expect(page).toHaveURL(new RegExp(`/my-listings/${listingId}$`));
  await page
    .getByRole("button", { name: "Submit for review", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Submit for review", exact: true })
    .click();
  await expect(
    page.getByText("An administrator is reviewing your listing.", {
      exact: false,
    }),
  ).toBeVisible();
  await expect(page.getByRole("textbox")).toHaveCount(0);
  expect(calls.writes.at(-1)?.body).toEqual({
    expected_updated_at: nextVersion,
  });
});
test("admin reviews rights, approves the version and can suspend with a reason", async ({
  page,
}) => {
  const calls = await mockMarketplace(page, { role: "admin" });
  await page.goto("/admin/marketplace");
  await page.getByRole("heading", { name: "Refund Bot" }).click();
  await expect(
    page.getByText("I created this avatar and own its commercial rights."),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Approve listing", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Approve listing", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Suspend listing", exact: true }),
  ).toBeVisible();
  expect(calls.writes[0].body).toEqual({ expected_updated_at: version });
  await page
    .getByRole("button", { name: "Suspend listing", exact: true })
    .click();
  await page
    .getByLabel("Reason for the seller")
    .fill("License has been revoked.");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Suspend listing", exact: true })
    .click();
  await expect(page.getByText("Suspended", { exact: true })).toBeVisible();
  expect(calls.writes[1].body).toEqual({ reason: "License has been revoked." });
});
test("admin stale rejection cannot report success or submit twice", async ({
  page,
}) => {
  const calls = await mockMarketplace(page, {
    role: "admin",
    staleReview: true,
  });
  await page.goto(`/admin/marketplace/${listingId}`);
  await page
    .getByRole("button", { name: "Reject listing", exact: true })
    .click();
  await page
    .getByLabel("Reason for the seller")
    .fill("Please document the license.");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Reject listing", exact: true })
    .click();
  await expect(
    page.getByText("This listing changed", { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("dialog")
      .getByRole("button", { name: "Reject listing", exact: true }),
  ).toBeDisabled();
  expect(calls.writes).toHaveLength(1);
});
test("mobile catalog and keyboard confirmation remain usable with reduced motion", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockMarketplace(page);
  await page.goto("/marketplace");
  await expect(page.getByRole("heading", { name: "Refund Bot" })).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("heading", { name: "Refund Bot" }).click();
  await page.getByRole("button", { name: "Buy for 20 coins" }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
for (const failed of [false, true])
  test(`catalog uses shared ${failed ? "error" : "empty"} state`, async ({
    page,
  }) => {
    await mockMarketplace(page, { empty: !failed, failed });
    await page.goto("/marketplace");
    await expect(
      page.getByText(failed ? "Something went wrong" : "No avatars here yet", {
        exact: true,
      }),
    ).toBeVisible();
  });

for (const role of ["candidate", "admin"] as const)
  test(`${role} cannot enter the other marketplace workspace`, async ({
    page,
  }) => {
    const calls = await mockMarketplace(page, { role });
    await page.goto(role === "admin" ? "/marketplace" : "/admin/marketplace");
    await expect(page).toHaveURL(
      new RegExp(role === "admin" ? "/admin/dashboard$" : "/dashboard$"),
    );
    expect(
      calls.reads.some((path) =>
        role === "admin"
          ? path.startsWith("/marketplace/")
          : path.startsWith("/admin/marketplace/"),
      ),
    ).toBe(false);
  });
