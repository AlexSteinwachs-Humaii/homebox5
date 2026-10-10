import { expect, test } from "@playwright/test";

// Presentation/action regression fixtures. No shared collection data is mutated.
for (const theme of ["cinematic", "homebox", "light"]) {
  for (const width of [390, 1440]) {
    test(`${theme} ${width}px item and location detail`, async ({ page, baseURL }, testInfo) => {
      await page.setViewportSize({ width, height: 1000 });
      await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL! }]);
      const name = "Record " + "LongUnbrokenName".repeat(15);
      const record = {
        id: "detail-item",
        name,
        quantity: 0,
        serialNumber: "",
        modelNumber: "",
        manufacturer: "",
        insured: false,
        archived: false,
        purchasePrice: 0,
        soldPrice: 0,
        description: "Original description",
        notes: "Original notes",
        fields: [{ name: "CustomField".repeat(12), textValue: "StoredValue".repeat(20) }],
        attachments: [],
        children: [],
        tags: [],
        parent: null,
        assetId: "000001",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
        purchaseTime: "0001-01-01",
        soldTime: "0001-01-01",
        warrantyExpires: "0001-01-01",
      };
      const mutations: string[] = [];
      await page.route("**/api/v1/**", route => {
        const request = route.request();
        const path = new URL(request.url()).pathname.replace("/api/v1", "");
        if (path === "/status")
          return route.fulfill({
            json: {
              build: { version: "v1.0.0", commit: "fixture" },
              latest: { version: "v1.0.0" },
              allowRegistration: true,
            },
          });
        if (path === "/users/self")
          return route.fulfill({
            json: {
              item: {
                id: "detail-user",
                name: "Member",
                defaultGroupId: "detail-group",
                group: { id: "detail-group", name: "Collection", currency: "USD" },
              },
            },
          });
        if (path === "/users/self/settings")
          return route.fulfill({ json: { item: { theme, displayLegacyHeader: false } } });
        if (path === "/groups/all") return route.fulfill({ json: [{ id: "detail-group", name: "Collection" }] });
        if (path === "/groups")
          return route.fulfill({ json: { id: "detail-group", name: "Collection", currency: "USD" } });
        if (path === "/entities/detail-item" && request.method() === "PATCH") {
          mutations.push(request.method());
          Object.assign(record, request.postDataJSON());
          return route.fulfill({ json: record });
        }
        if (request.method() === "DELETE") mutations.push("DELETE");
        if (path === "/entities/detail-item") return route.fulfill({ json: record });
        if (path === "/entities/detail-location")
          return route.fulfill({
            json: {
              ...record,
              id: "detail-location",
              isLocation: true,
              parent: { id: "parent-location", name: "Parent " + name },
              children: [{ id: "child-location", name: "Child " + name, itemCount: 0 }],
            },
          });
        if (path.endsWith("/path"))
          return route.fulfill({
            json: [
              { id: "parent-location", name: "Parent " + name, type: "location" },
              { id: record.id, name, type: "item" },
            ],
          });
        if (path === "/entities") return route.fulfill({ json: { items: [], page: 1, pageSize: 10, total: 0 } });
        return route.fulfill({ json: [] });
      });
      await page.goto("/item/detail-item");
      await expect(page.locator("h1")).toHaveText(name);
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await expect(page.locator("dl").filter({ hasText: "Original notes" })).toContainText("Original notes");
      await expect(page.locator("dl").filter({ hasText: "Original notes" })).toContainText("StoredValue".repeat(20));
      const increase = page.getByRole("button", { name: "Increase quantity" });
      await increase.focus();
      await expect.poll(() => increase.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
      await page.screenshot({ path: testInfo.outputPath("item-detail.png"), fullPage: true });
      await page.keyboard.press("Enter");
      await expect.poll(() => record.quantity).toBe(1);
      await page.getByRole("button", { name: "Decrease quantity" }).click();
      await expect.poll(() => record.quantity).toBe(0);
      await page.getByRole("button", { name: "Decrease quantity" }).click();
      await expect(page.getByText("Quantity cannot be negative")).toBeVisible();
      expect(mutations).toEqual(["PATCH", "PATCH"]);
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await expect(page.getByRole("link", { name: "Edit", exact: true })).toHaveAttribute(
        "href",
        "/item/detail-item/edit"
      );
      await page.getByRole("button", { name: "More actions" }).click();
      await expect(page.getByRole("menuitem", { name: "Duplicate", exact: true })).toBeVisible();
      await page.getByRole("menuitem", { name: "Delete", exact: true }).click();
      await page.getByRole("button", { name: "Cancel", exact: true }).click();
      expect(mutations).not.toContain("DELETE");
      await page.goto("/location/detail-location");
      await expect(page.locator("h1")).toContainText(name);
      // The shell hides horizontal overflow, so scrollWidth alone cannot catch clipped heading text.
      await expect
        .poll(() =>
          page.locator("h1").evaluate(el => {
            const range = document.createRange();
            range.selectNodeContents(el);
            const bounds = el.getBoundingClientRect();
            return [...range.getClientRects()].every(
              rect => rect.right <= bounds.right + 1 && rect.left >= bounds.left - 1
            );
          })
        )
        .toBe(true);
      await expect(page.locator("dl").filter({ hasText: "Original notes" })).toContainText("Original notes");
      await expect(page.getByRole("link", { name: "Parent " + name, exact: true })).toHaveAttribute(
        "href",
        "/location/parent-location"
      );
      await expect(page.getByRole("link", { name: "Child " + name, exact: false })).toBeVisible();
      await expect(page.getByRole("button", { name: "Create Item", exact: true })).toBeVisible();
      await page.getByRole("button", { name: "Delete", exact: true }).click();
      await page.getByRole("button", { name: "Cancel", exact: true }).click();
      expect(mutations).not.toContain("DELETE");
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: testInfo.outputPath("location-detail.png"), fullPage: true });
      await page.getByRole("button", { name: "Edit", exact: true }).click();
      await expect(page).toHaveURL(/\/location\/detail-location\/edit$/);
    });
  }
}
