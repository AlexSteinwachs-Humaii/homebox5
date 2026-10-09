import { expect, test, type Page } from "@playwright/test";

const longName = "Inventory identification " + "VeryLongUnbrokenName".repeat(6);
const locationName = "Storage " + "LongLocationName".repeat(6);
const tagName = "Tag " + "LongTagName".repeat(6);
const itemId = "overview-item";
const photoId = "overview-photo";
const tag = { id: "overview-tag", name: tagName, color: "", icon: "", parentId: null };
const record = {
  id: itemId,
  name: longName,
  quantity: 7,
  insured: true,
  purchasePrice: 1234.56,
  description: "",
  tags: [tag],
  parent: null,
  imageId: null,
  thumbnailId: null,
};

async function fixture(page: Page, empty: boolean) {
  // Response fixtures test presentation without modifying the shared demo inventory.
  await page.route("**/api/v1/entities?*", route => {
    const locations = new URL(route.request().url()).searchParams.get("isLocation") === "true";
    const items = empty
      ? []
      : locations
        ? [{ ...record, id: "overview-location", name: locationName, itemCount: 0 }]
        : [
            record,
            {
              ...record,
              id: photoId,
              name: "Real photo record",
              quantity: 2,
              insured: false,
              purchasePrice: 0,
              imageId: "original",
            },
          ];
    return route.fulfill({ json: { items, page: 1, pageSize: 5, total: items.length } });
  });
  await page.route("**/api/v1/groups/statistics", route =>
    route.fulfill({
      json: {
        totalItemPrice: empty ? 0 : 1234.56,
        totalItems: empty ? 0 : 9,
        totalLocations: empty ? 0 : 1,
        totalTags: empty ? 0 : 1,
      },
    })
  );
  await page.route("**/api/v1/tags", route => route.fulfill({ json: empty ? [] : [tag] }));
  await page.route("**/api/v1/entities/overview-photo/attachments/original?*", route =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: "<svg xmlns='http://www.w3.org/2000/svg' width='120' height='80'><rect width='120' height='80' fill='blue'/></svg>",
    })
  );
  await page.route("**/api/v1/users/self/settings", route =>
    route.fulfill({
      json: {
        item: { displayLegacyHeader: false, theme: "cinematic" },
      },
    })
  );
  await page.goto("/");
  await page.waitForLoadState("networkidle");
  await page.locator("input[type='text']").fill("demo@example.com");
  await page.locator("input[type='password']").fill("demodemo");
  await page.locator("button[type='submit']").click();
  await expect(page).toHaveURL("/home");
}

for (const width of [390, 834, 1440]) {
  for (const empty of [false, true]) {
    test(`overview ${width}px ${empty ? "empty" : "populated"}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 1000 });
      await fixture(page, empty);
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
      await testInfo.attach("dashboard", { path: screenshot, contentType: "image/png" });
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
