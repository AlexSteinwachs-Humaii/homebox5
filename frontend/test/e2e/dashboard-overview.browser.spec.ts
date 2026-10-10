import { expect, test } from "@playwright/test";
import { dashboardFixture, longName, locationName, tagName, itemId } from "./dashboard-fixture";

for (const width of [390, 834, 1440]) {
  for (const empty of [false, true]) {
    test(`overview ${width}px ${empty ? "empty" : "populated"}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 1000 });
      await dashboardFixture(page, empty);
      await expect(page.getByRole("searchbox", { name: "Search", exact: true })).toBeVisible();
      const sections = page.locator("section");
      const stats = sections.filter({ hasText: "Quick Statistics" });
      const recent = sections.filter({ hasText: "Recently Added" });
      expect((await recent.boundingBox())?.y).toBeLessThan(900);
      const locations = sections.filter({ hasText: "Storage Locations" });
      const tags = sections.filter({ hasText: "Tags" }).last();
      await expect(stats).toContainText(empty ? "0" : "9");
      if (empty) {
        await expect(recent).toContainText("No Items Found");
        await expect(locations).toContainText("No Locations Found");
        await expect(tags).toContainText("No Tags Found");
      } else {
        await expect(stats).toContainText("1,234.56");
        await expect(recent).toContainText(longName);
        await expect(recent).toContainText("1,234.56");
        await expect(recent.locator(`a[href='/item/${itemId}']`).first()).toBeVisible();
        await expect(locations.locator("a[href='/location/overview-location']")).toContainText(locationName);
        await expect(locations).toContainText("0");
        await expect(tags.locator("a[href='/tag/overview-tag']")).toContainText(tagName);
        if (width < 768) {
          await expect(recent.locator("table")).toHaveCount(0);
          await expect(recent).toContainText("Yes");
          await expect(recent).toContainText("No");
          await expect(recent.locator("img")).toHaveCount(1);
          const photo = recent.locator("img");
          await expect(photo).toHaveAttribute("src", /attachments\/original/);
          await expect(photo).toHaveCSS("filter", "none");
          await expect(photo).toHaveCSS("object-fit", "contain");
        } else {
          await expect(recent.locator("table")).toBeVisible();
          await expect(recent.locator("tbody tr").first()).toContainText("7");
        }
      }
      if (width < 1024) {
        await expect(page.locator("header svg[aria-hidden='true']")).toBeHidden();
      }
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const screenshot = testInfo.outputPath("dashboard.png");
      await page.screenshot({ path: screenshot, fullPage: true });
      await testInfo.attach("dashboard", {
        path: screenshot,
        contentType: "image/png",
      });
      expect(
        await recent
          .locator("table")
          .evaluateAll(tables => tables.every(table => table.scrollWidth <= table.parentElement!.clientWidth + 1))
      ).toBe(true);
      // Labels must wrap inside their own boxes, not just hide page overflow.
      expect(
        await page
          .locator("section h2, section dt, section a span")
          .evaluateAll(elements => elements.every(el => el.scrollWidth <= el.clientWidth + 1))
      ).toBe(true);
    });
  }
}
