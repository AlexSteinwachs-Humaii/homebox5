import { expect, test } from "@playwright/test";
import { fixture } from "./remaining-surfaces-fixture";

import { contrast } from "./remaining-surfaces-contrast";

// API/camera fixtures must not be bypassed by the production PWA service worker.
test.use({ serviceWorkers: "block" });

for (const width of [390, 834, 1440]) {
  for (const theme of ["light", "cinematic"]) {
    test(`${theme}/${width}: confirmations, validation, focus return and measured contrast`, async ({
      page,
      baseURL,
    }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await fixture(page, baseURL!, theme);
      const mutations: string[] = [];
      await page.route("**/api/v1/actions/**", route => {
        mutations.push(route.request().url());
        return route.fulfill({
          status: 500,
          json: { message: "Fixture failure" },
        });
      });
      await page.goto("/collection/tools");
      const ensure = page.getByRole("button", {
        name: "Ensure Asset IDs",
        exact: true,
      });
      await ensure.focus();
      const toolContrast = await contrast(ensure);
      await ensure.press("Enter");
      const confirmation = page.getByRole("alertdialog");
      await expect(confirmation).toBeVisible();
      await expect(confirmation.getByRole("button", { name: "Cancel", exact: true })).toBeFocused();
      const measurements = {
        tool: toolContrast,
        confirmation: await contrast(confirmation.locator("p")),
      };
      await page.keyboard.press("Escape");
      await expect(confirmation).toHaveCount(0);
      await expect(ensure).toBeFocused();
      expect(mutations).toHaveLength(0);
      await ensure.press("Enter");
      await confirmation.getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.locator("[data-sonner-toast]").first()).toBeVisible();
      expect(mutations).toHaveLength(1);
      await expect(ensure).toBeFocused();
      expect(await page.locator("main").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      await page.screenshot({ path: testInfo.outputPath(`tools-${theme}-${width}.png`), fullPage: true });
      await testInfo.attach("computed-contrast", {
        body: JSON.stringify(measurements, null, 2),
        contentType: "application/json",
      });
      await page.goto("/no-collections");
      const create = page.getByRole("button", {
        name: "Create a new collection",
        exact: true,
      });
      await create.focus();
      await create.press("Enter");
      const dialog = page.getByRole("dialog");
      await expect(dialog.locator("input").first()).toBeFocused();
      expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      await page.keyboard.press("Escape");
      await expect(create).toBeFocused();
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      expect(await page.evaluate(() => JSON.parse(localStorage.getItem("homebox/preferences/location")!).theme)).toBe(
        theme
      );
    });

    test(`${theme}/${width}: asset failures/empty results and legacy shortcut redirects`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 900 });
      await fixture(page, baseURL!, theme);
      let mode = "empty";
      await page.route("**/api/v1/assets/**", route =>
        route.fulfill({
          status: mode === "failure" ? 500 : 200,
          json: mode === "failure" ? { message: "Fixture failure" } : { items: [], total: 0 },
        })
      );
      await page.goto("/a/000-001");
      await expect(page).toHaveURL(/\/home\/?$/);
      await expect(page.locator("[data-sonner-toast]").first()).toContainText(/not found/i);
      mode = "failure";
      await page.goto("/assets/000-001");
      await expect(page).toHaveURL(/\/home\/?$/);
      await expect(page.locator("[data-sonner-toast]").first()).toContainText(/failed/i);
      await page.route("**/api/v1/tags/legacy*", route =>
        route.fulfill({ json: { id: "legacy", name: "Legacy tag" } })
      );
      await page.goto("/label/legacy");
      await expect(page).toHaveURL(/\/tag\/legacy\/?$/);
    });
  }
}
