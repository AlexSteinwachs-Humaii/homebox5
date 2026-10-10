import { expect, test, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import type { CsvRequest } from "../../lib/api/types/csv-contracts.generated";

// Fixture routing must see NetworkOnly CSV requests rather than a service worker.
test.use({ serviceWorkers: "block" });

const itemId = "11111111-1111-4111-8111-111111111111";
const roomId = "22222222-2222-4222-8222-222222222222";
const tagId = "33333333-3333-4333-8333-333333333333";
const record = {
  id: itemId,
  name: "Lamp 灯",
  description: "Full description",
  quantity: 2,
  insured: true,
  archived: false,
  assetId: "123-456",
  purchasePrice: 12.5,
  tags: [],
  parent: null,
  imageId: null,
  thumbnailId: null,
  fields: [],
  attachments: [],
  children: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

async function fixture(page: Page, baseURL: string, empty = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  await page.addInitScript(() =>
    localStorage.setItem(
      "homebox/preferences/location",
      JSON.stringify({
        itemDisplayView: "table",
        theme: "cinematic",
        itemsPerTablePage: 12,
        tableHeaders: [
          { value: "purchasePrice", enabled: true },
          { value: "name", enabled: true },
          { value: "quantity", enabled: false },
          { value: "assetId", enabled: false },
          { value: "insured", enabled: false },
          { value: "location", enabled: false },
          { value: "archived", enabled: false },
          { value: "createdAt", enabled: false },
          { value: "updatedAt", enabled: false },
        ],
      })
    )
  );
  const requests: CsvRequest[] = [];
  const location = {
    ...record,
    id: roomId,
    name: "Room",
    isLocation: true,
    itemCount: 99,
    children: [],
  };
  const tag = {
    id: tagId,
    name: "Tag",
    parentId: null,
    color: "",
    description: "",
    createdAt: record.createdAt,
    updatedAt: record.updatedAt,
  };
  const rows = empty ? [] : [record];
  await page.route("**/api/v1/**", async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    if (path === "/group/csv-exports" && route.request().method() === "POST") {
      requests.push(route.request().postDataJSON());
      return route.fulfill({
        status: 202,
        json: { id: "artifact", status: "pending" },
      });
    }
    if (path === "/group/csv-exports/artifact")
      return route.fulfill({
        json: {
          id: "artifact",
          source: "entities",
          status: "completed",
          expiresAt: "2026-12-01T00:00:00Z",
        },
      });
    if (path === "/group/csv-exports/artifact/download")
      return route.fulfill({
        contentType: "text/csv",
        body: "Name\nLamp 灯\nNot on current page\n",
      });
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
      return route.fulfill({
        json: { item: { theme: "cinematic", displayLegacyHeader: false } },
      });
    if (path === "/groups/all") return route.fulfill({ json: [{ id: "group", name: "Collection" }] });
    if (path === "/groups")
      return route.fulfill({
        json: { id: "group", name: "Collection", currency: "USD" },
      });
    if (path === "/groups/statistics")
      return route.fulfill({
        json: {
          totalItems: 99,
          totalLocations: 1,
          totalTags: 1,
          totalItemPrice: 12.5,
        },
      });
    if (path === "/tags") return route.fulfill({ json: empty ? [] : [tag] });
    if (path === `/tags/${tagId}`) return route.fulfill({ json: tag });
    if (path === `/entities/${itemId}`) return route.fulfill({ json: record });
    if (path === `/entities/${roomId}`) return route.fulfill({ json: location });
    if (path === "/entities/tree")
      return route.fulfill({
        json: empty
          ? []
          : [
              {
                id: roomId,
                name: "Room",
                type: "location",
                children: [{ id: itemId, name: "Lamp 灯", type: "item", children: [] }],
              },
            ],
      });
    if (path === "/entities")
      return route.fulfill({
        json: {
          items: url.searchParams.get("isLocation") === "true" ? (empty ? [] : [location]) : rows,
          total: empty ? 0 : 99,
          page: 3,
          pageSize: 12,
        },
      });
    if (path === "/assets/123456")
      return route.fulfill({
        json: { items: rows, total: 99, page: 1, pageSize: 50 },
      });
    return route.fulfill({ json: [] });
  });
  return requests;
}

async function generate(page: Page, index = 0) {
  const exportControl = page.locator("[data-csv-export]").nth(index);
  await exportControl.getByRole("button", { name: "Export CSV", exact: true }).click();
  await exportControl.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect(exportControl.getByRole("button", { name: "Download CSV", exact: true })).toBeVisible();
  return exportControl;
}

test("table export captures query and ordered columns without changing selection, URL, page or theme", async ({
  page,
  baseURL,
}) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/items?q=Lamp&tag=${tagId}&loc=${roomId}&archived=true&orderBy=updatedAt&page=3`);
  const selection = page.getByRole("checkbox", {
    name: "Select Row: Lamp 灯",
    exact: true,
  });
  await selection.check();
  const beforeURL = page.url();
  const beforePrefs = await page.evaluate(() => localStorage.getItem("homebox/preferences/location"));
  const control = await generate(page);
  expect(requests).toHaveLength(1);
  expect(requests[0]!.columns.map(c => c.id)).toEqual(["purchasePrice", "name"]);
  expect(requests[0]!.filters).toMatchObject({
    search: "Lamp",
    tagIds: [tagId],
    parentIds: [roomId],
    includeArchived: true,
  });
  expect(requests[0]!.sort).toEqual([{ column: "updatedAt", direction: "desc" }]);
  expect(requests[0]!.boundary).toEqual({ kind: "all" });
  expect(JSON.stringify(requests[0])).not.toMatch(/pageSize|pageIndex|recordIds/);
  await expect(selection).toBeChecked();
  expect(page.url()).toBe(beforeURL);
  expect(await page.evaluate(() => localStorage.getItem("homebox/preferences/location"))).toBe(beforePrefs);
  const download = page.waitForEvent("download");
  await control.getByRole("button", { name: "Download CSV" }).click();
  const saved = await download;
  expect(saved.suggestedFilename()).toBe("entities.csv");
  expect(await readFile((await saved.path())!, "utf8")).toBe("Name\nLamp 灯\nNot on current page\n");
  expect(requests).toHaveLength(1);
});

for (const empty of [false, true]) {
  test(`dashboard sections have separate boundaries (${empty ? "empty" : "populated"})`, async ({ page, baseURL }) => {
    const requests = await fixture(page, baseURL!, empty);
    await page.goto("/home");
    await expect(page.locator("[data-csv-export]")).toHaveCount(3);
    for (let i = 0; i < 3; i++) await generate(page, i);
    expect(requests.map(r => [r.source, r.boundary])).toEqual([
      ["entities", { kind: "recent", limit: 5 }],
      ["locations", { kind: "roots" }],
      ["tags", { kind: "all" }],
    ]);
  });
}

for (const [path, source, boundary, filters] of [
  ["/locations?showItems=false", "locations", "hierarchy", { showItems: false }],
  ["/tags", "tags", "hierarchy", { showItems: false }],
  [`/item/${itemId}`, "entities", "children", { entityId: itemId }],
  [`/location/${roomId}`, "entities", "children", { entityId: roomId }],
  [`/tag/${tagId}`, "entities", "all", { tagIds: [tagId] }],
  ["/assets/123456", "asset-results", "all", { assetId: 123456 }],
] as const) {
  test(`export descriptor for ${path}`, async ({ page, baseURL }) => {
    const requests = await fixture(page, baseURL!);
    await page.goto(path);
    await generate(page);
    expect(requests[0]!.source).toBe(source);
    expect(requests[0]!.boundary.kind).toBe(boundary);
    expect(requests[0]!.filters).toMatchObject(filters);
    if (boundary === "hierarchy") expect(requests[0]!.columns.map(c => c.id)).toEqual(["name", "parentId", "path"]);
    if (path.startsWith("/location/")) {
      await generate(page, 1);
      expect(requests[1]).toMatchObject({
        source: "locations",
        boundary: { kind: "children" },
        filters: { entityId: roomId },
      });
    }
  });
}

test("mobile card export captures displayed fields, not table preferences", async ({ page, baseURL }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const requests = await fixture(page, baseURL!);
  await page.goto("/home");
  await generate(page);
  expect(requests[0]!.columns.map(c => c.id)).toEqual([
    "name",
    "location",
    "insured",
    "archived",
    "quantity",
    "purchasePrice",
    "tags",
    "path",
  ]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("mobile-export.png"), fullPage: true });
});

test("embedded table captures its effective local sort and export controls return focus", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/tag/${tagId}`);
  const priceHeader = page
    .getByRole("columnheader")
    .filter({ has: page.getByRole("button", { name: "Purchase Price", exact: true }) });
  await priceHeader.getByRole("button").click();
  await priceHeader.getByRole("button").click();
  await expect(priceHeader).toHaveAttribute("aria-sort", "descending");
  const beforeURL = page.url();
  const control = page.locator("[data-csv-export]").first();
  const trigger = control.getByRole("button", { name: "Export CSV", exact: true });
  await trigger.focus();
  await page.keyboard.press("Enter");
  await control.getByRole("button", { name: "Generate CSV" }).click();
  await expect(control.getByRole("button", { name: "Download CSV" })).toBeVisible();
  expect(requests[0]!.sort).toEqual([{ column: "purchasePrice", direction: "desc" }]);
  expect(page.url()).toBe(beforeURL);
  await control.getByRole("button", { name: "Close", exact: true }).click();
  await expect(trigger).toBeFocused();
});
