import { expect, test, type Page } from "@playwright/test";
import de from "../../locales/de.json" with { type: "json" };

const longName = "Storage " + "LongUnbrokenName".repeat(8);

async function inventoryFixture(page: Page, baseURL: string, theme: string) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const requests: URLSearchParams[] = [];
  const state = { fail: false, hold: false, waiting: false, release: () => {} };
  const location = { id: "room", name: longName, parent: null, children: [], tags: [], isLocation: true };
  await page.route("**/api/v1/**", async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    if (path === "/status") return route.fulfill({ json: { build: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({
        json: {
          item: {
            id: "user",
            name: "Member",
            defaultGroupId: "group",
            group: { id: "group", name: "Collection", currency: "USD" },
          },
        },
      });
    if (path === "/users/self/settings")
      return route.fulfill({ json: { item: { theme, displayLegacyHeader: false } } });
    if (path === "/groups/all") return route.fulfill({ json: [{ id: "group", name: "Collection" }] });
    if (path === "/groups") return route.fulfill({ json: { id: "group", name: "Collection", currency: "USD" } });
    if (path === "/tags") return route.fulfill({ json: [{ id: "tag", name: longName, color: "", parentId: null }] });
    if (path === "/entities/tree") return route.fulfill({ json: [location] });
    if (path === "/entities/fields") return route.fulfill({ json: ["Manufacturer", "Color"] });
    if (path === "/entities/fields/values") return route.fulfill({ json: ["Acme", "Blue"] });
    if (path === "/entities") {
      if (url.searchParams.get("isLocation") === "true")
        return route.fulfill({ json: { items: [location], total: 1, page: 1, pageSize: 10 } });
      requests.push(url.searchParams);
      if (state.hold)
        await new Promise<void>(resolve => {
          state.waiting = true;
          state.release = resolve;
        });
      if (state.fail) return route.fulfill({ status: 500, json: { message: "Search failed" } });
      return route.fulfill({ json: { items: [], total: 0, page: 1, pageSize: 10 } });
    }
    return route.fulfill({ json: [] });
  });
  return { requests, state };
}

for (const theme of ["cinematic", "light", "homebox"]) {
  for (const width of [390, 1440]) {
    test(`${theme} ${width}px search filters retain labels, URL hydration and enabled states`, async ({
      page,
      baseURL,
    }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      const { requests } = await inventoryFixture(page, baseURL!, theme);
      await page.goto(
        "/items?q=lamp&loc=room&tag=tag&archived=true&onlyWithoutPhoto=true&fieldSelector=true&fields=Manufacturer%3DAcme&fields=Color%3DBlue"
      );
      const search = page.locator("[data-inventory-search]").getByRole("searchbox", { name: "Search", exact: true });
      await expect(search).toHaveValue("lamp");
      await expect(page.getByRole("button", { name: "Locations (1)", exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Tags (1)", exact: true })).toBeVisible();
      await expect(page.getByRole("combobox", { name: "Field", exact: true }).first()).toContainText("Manufacturer");
      await search.focus();
      await expect.poll(() => search.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
      await page.locator("[data-inventory-search]").getByRole("button", { name: "Search", exact: true }).click();
      await expect.poll(() => requests.at(-1)?.getAll("fields")).toEqual(["Manufacturer=Acme", "Color=Blue"]);
      expect(requests.at(-1)?.getAll("parentIds")).toEqual(["room"]);
      expect(requests.at(-1)?.getAll("tags")).toEqual(["tag"]);
      expect(requests.at(-1)?.get("includeArchived")).toBe("true");
      await page.getByRole("button", { name: "Options", exact: true }).click();
      const archived = page.getByRole("switch", { name: "Include Archived Items", exact: true });
      await expect(archived).toBeEnabled();
      await expect(archived).toBeChecked();
      await expect(page.locator("#inventory-without-photo")).toBeChecked();
      await page.locator("#inventory-with-photo").focus();
      await page.keyboard.press("Space");
      await expect(page.locator("#inventory-without-photo")).not.toBeChecked();
      await expect.poll(() => new URL(page.url()).searchParams.get("onlyWithPhoto")).toBe("true");
      await page.keyboard.press("Escape");
      await page.getByRole("button", { name: "Tags (1)", exact: true }).click();
      await expect(page.getByRole("checkbox", { name: longName, exact: true })).toBeChecked();
      await page.getByRole("checkbox", { name: longName, exact: true }).click();
      await expect.poll(() => new URL(page.url()).searchParams.has("tag")).toBe(false);
      await page.getByRole("searchbox", { name: /^Tags:/ }).fill("not-a-tag");
      await expect(page.getByRole("status").filter({ hasText: "No Results Found" })).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: testInfo.outputPath("filters.png"), fullPage: true });
      await page.keyboard.press("Escape");
      await page.reload();
      await expect(search).toHaveValue("lamp");
      await expect(page.getByRole("button", { name: "Locations (1)", exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Tags", exact: true })).toBeVisible();
      await page.getByRole("button", { name: "Options", exact: true }).click();
      await page.locator("#inventory-fields").click();
      await page.keyboard.press("Escape");
      await expect(page.getByRole("combobox", { name: "Field", exact: true })).toHaveCount(0);
      await page.locator("[data-inventory-search]").getByRole("button", { name: "Search", exact: true }).click();
      await expect.poll(() => requests.at(-1)?.getAll("fields")).toEqual([]);
    });
  }
}

test("query back/forward and loading, empty, invalid asset ID and API failure remain distinct", async ({
  page,
  baseURL,
}) => {
  const { state } = await inventoryFixture(page, baseURL!, "cinematic");
  await page.goto("/items?q=lamp");
  const search = page.locator("[data-inventory-search]").getByRole("searchbox", { name: "Search", exact: true });
  await expect(page.getByText("No Items Found", { exact: true })).toBeVisible();
  await search.fill("chair");
  await expect.poll(() => new URL(page.url()).searchParams.get("q")).toBe("chair");
  await page.goBack();
  await expect(search).toHaveValue("lamp");
  await page.goForward();
  await expect(search).toHaveValue("chair");
  await search.fill("");
  await expect.poll(() => new URL(page.url()).searchParams.has("q")).toBe(false);
  state.hold = true;
  await search.fill("#invalid");
  await expect(page.locator("#inventory-asset-query")).toContainText("Invalid Asset ID");
  await expect(page.getByRole("status").filter({ hasText: "Loading" })).toBeVisible();
  await expect(page.getByText("No Items Found", { exact: true })).not.toBeVisible();
  await expect.poll(() => state.waiting).toBe(true);
  state.fail = true;
  state.hold = false;
  state.release();
  await expect(page.locator("section [role=alert]")).toContainText("Failed to search items");
  await expect(page.getByText("No Items Found", { exact: true })).not.toBeVisible();
});

test.describe("translated search controls", () => {
  test.use({ locale: "de-DE", viewport: { width: 390, height: 900 } });
  test("German labels and long selector names fit narrow popovers", async ({ page, baseURL }, testInfo) => {
    await inventoryFixture(page, baseURL!, "cinematic");
    await page.goto("/items");
    const panel = page.locator("[data-inventory-search]");
    await expect(panel.getByRole("searchbox", { name: de.global.search, exact: true })).toBeVisible();
    await panel.getByRole("button", { name: de.items.options, exact: true }).click();
    await expect(page.getByRole("switch", { name: de.items.only_without_photo, exact: true })).toBeVisible();
    const bounds = await page.locator("[data-reka-popper-content-wrapper]").last().boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
    await page.screenshot({ path: testInfo.outputPath("translated-options.png"), fullPage: true });
    await page.keyboard.press("Escape");
    await panel.getByRole("button", { name: de.global.locations, exact: true }).click();
    await expect(page.getByRole("checkbox", { name: longName })).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath("long-location.png"), fullPage: true });
  });
});
