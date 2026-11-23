import { expect, test, type Page } from "@playwright/test";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";

// The room opens on the room saved in the setup of its session (KAN-231 follow-up). An in-room
// change is a choice for this visit only and is never written back.
const room = `/interviews/${roomId}/room`;
const stage = (page: Page) =>
  page.getByRole("region", { name: "Interview room", exact: true });
const backdrop = (page: Page) =>
  page.locator('[data-slot="studio-backdrop"] img');

test("a practice room opens on its saved room and a change stays in the room", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, { savedRoom: "balcony" });
  await page.goto(room);
  await expect(page.getByText("Practice room")).toBeVisible({
    timeout: 15_000,
  });
  await expect(stage(page)).toHaveAttribute("data-background", "balcony");
  await expect(backdrop(page)).toHaveAttribute("src", /balcony\.webp/);
  expect(calls.setupReads).toHaveLength(1);
  expect(calls.setupReads[0]).toMatch(/^\/interview-setups\/practice\//);

  await page.getByRole("button", { name: "Choose background" }).click();
  await page.getByRole("radio", { name: "Office", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(stage(page)).toHaveAttribute("data-background", "office");
  expect(calls.writes).toEqual([]);
});

test("a recruitment room opens on the room saved for its application", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, {
    recruitment: true,
    savedRoom: "meeting-3d",
  });
  await page.goto(room);
  await expect(stage(page)).toHaveAttribute("data-background", "meeting-3d", {
    timeout: 15_000,
  });
  expect(calls.setupReads[0]).toMatch(/^\/interview-setups\/recruitment\//);
});

test("the default room stays when the saved room is unknown or missing", async ({
  page,
}) => {
  await mockRoomPreview(page, { savedRoom: "moon-base" });
  await page.goto(room);
  await expect(stage(page)).toHaveAttribute("data-background", "cafe-3d", {
    timeout: 15_000,
  });
  await page.unrouteAll({ behavior: "ignoreErrors" });

  await mockRoomPreview(page, { savedRoom: null });
  await page.goto(room);
  await expect(stage(page)).toHaveAttribute("data-background", "cafe-3d", {
    timeout: 15_000,
  });
});
