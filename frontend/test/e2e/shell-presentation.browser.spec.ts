import { expect, test } from "@playwright/test";

// Uses the same demo account as the existing login suite; no inventory mutations.
test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/users/self/settings", async route => {
    if (route.request().method() !== "GET") {
      await route.fulfill({ json: { item: route.request().postDataJSON() } });
      return;
    }
    const response = await route.fetch();
    const body = await response.json();
    body.item = { ...body.item, displayLegacyHeader: false, theme: "cinematic" };
    await route.fulfill({ response, json: body });
  });
  await page.goto("/");
  // Demo credentials are initialized on mount; wait before replacing them.
  await page.waitForLoadState("networkidle");
  await page.fill("input[type='text']", "demo@example.com");
  await page.fill("input[type='password']", "demodemo");
  await page.click("button[type='submit']");
  await expect(page).toHaveURL("/home");
});

test("shell destinations, search and keyboard quick menu", async ({ page }) => {
  const home = page.locator('[data-sidebar="menu-button"][href="/home"]');
  await expect(home).toHaveAttribute("aria-current", "page");
  for (const path of [
    "/locations",
    "/tags",
    "/items",
    "/templates",
    "/maintenance",
    "/profile",
    "/collection/members",
  ]) {
    await expect(page.locator(`[data-sidebar='menu-button'][href='${path}']`).first()).toBeVisible();
  }
  await page.getByRole("button", { name: "Toggle sidebar", exact: true }).click();
  await expect(page.locator('[data-collapsible="icon"]')).toBeVisible();
  await page.getByRole("button", { name: "Toggle sidebar", exact: true }).click();
  const search = page.getByRole("searchbox", { name: "Search" });
  await search.fill("shell regression");
  await search.press("Enter");
  await expect(page).toHaveURL(/\/items\?q=shell%20regression/);
  const trigger = page.getByRole("button", { name: "Toggle sidebar", exact: true });
  await trigger.focus();
  await page.keyboard.press("Control+Backquote");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAccessibleName(/number keys/i);
  await page.keyboard.press("Tab");
  await expect.poll(() => dialog.evaluate(el => el.contains(document.activeElement))).toBe(true);
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(trigger).toBeFocused();
  await page.keyboard.press("Control+Backquote");
  await page.getByRole("option", { name: "Location 2" }).click();
  const create = page.getByRole("dialog").filter({ has: page.getByRole("heading") });
  await expect(create.getByRole("heading")).toHaveText(/Create\s*Location/);
  await page.keyboard.press("Escape");
  await expect(create).toBeHidden();
});

test("narrow shell and shared dialog keep long labels within viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "Toggle sidebar", exact: true }).click();
  const selector = page.getByRole("combobox", { name: "Select Collection" });
  await selector.click();
  await page.getByText("Create New Collection", { exact: true }).click();
  const dialog = page.getByRole("dialog").filter({ has: page.getByRole("heading") });
  await expect(dialog).toBeVisible();
  // Emulate a long translated title without changing domain data.
  await dialog.getByRole("heading").evaluate(el => {
    el.textContent = "Eine neue gemeinsame Sammlung mit einem sehr langen übersetzten Namen erstellen";
  });
  const bounds = await dialog.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
  await expect.poll(() => dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
});

test("legacy light header preserves scanner, shortcuts and sign-out", async ({ page }) => {
  // Read-only settings fixture; do not overwrite the shared demo account's settings.
  await page.route("**/api/v1/users/self/settings", async route => {
    if (route.request().method() !== "GET") {
      await route.fulfill({ json: { item: route.request().postDataJSON() } });
      return;
    }
    const response = await route.fetch();
    const body = await response.json();
    body.item.displayLegacyHeader = true;
    body.item.theme = "homebox";
    await route.fulfill({ response, json: body });
  });
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "homebox");
  await expect(page.getByRole("searchbox")).toBeHidden();
  // Target the legacy sidebar control, not the modern header's hidden button
  // while the saved settings snapshot is still being applied.
  await page.locator("[data-sidebar='sidebar']").getByRole("button", { name: "Scanner", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await page.keyboard.press("Control+b");
  await expect(page.locator('[data-collapsible="icon"]')).toBeVisible();
  await page.getByTestId("logout-button").click();
  await expect(page).toHaveURL("/");
});
