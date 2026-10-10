import { expect, type Page } from "@playwright/test";

export const longName = "Inventory identification " + "VeryLongUnbrokenName".repeat(6);
export const locationName = "Storage " + "LongLocationName".repeat(6);
export const tagName = "Tag " + "LongTagName".repeat(6);
export const itemId = "overview-item";
const photoId = "overview-photo";
const tag = {
  id: "overview-tag",
  name: tagName,
  color: "#e09020",
  icon: "",
  parentId: null,
};
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

export async function dashboardFixture(page: Page, empty: boolean, theme = "cinematic") {
  // Response fixtures test presentation without modifying the shared demo inventory.
  await page.route("**/api/v1/entities?*", route => {
    const locations = new URL(route.request().url()).searchParams.get("isLocation") === "true";
    const items = empty
      ? []
      : locations
        ? [
            {
              ...record,
              id: "overview-location",
              name: locationName,
              itemCount: 0,
            },
          ]
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
              parent: { id: "overview-location", name: locationName },
            },
          ];
    return route.fulfill({
      json: { items, page: 1, pageSize: 5, total: items.length },
    });
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
  await page.route("**/api/v1/tags", route =>
    route.fulfill({
      json: empty ? [] : [tag, { ...tag, id: "overview-default-tag", name: "Uncolored tag", color: "" }],
    })
  );
  await page.route("**/api/v1/entities/overview-photo/attachments/original?*", route =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: "<svg xmlns='http://www.w3.org/2000/svg' width='120' height='80'><rect width='120' height='80' fill='blue'/></svg>",
    })
  );
  await page.route("**/api/v1/users/self/settings", route =>
    route.fulfill({
      json: {
        item: { displayLegacyHeader: false, theme },
      },
    })
  );
  const detail = {
    ...record,
    fields: [],
    attachments: [],
    children: [],
    assetId: "000001",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    purchaseTime: "0001-01-01",
    soldTime: "0001-01-01",
    warrantyExpires: "0001-01-01",
  };
  await page.route("**/api/v1/entities/overview-item", route => route.fulfill({ json: detail }));
  await page.route("**/api/v1/entities/overview-location", route =>
    route.fulfill({ json: { ...detail, id: "overview-location", name: locationName, isLocation: true } })
  );
  await page.route("**/api/v1/tags/overview-tag", route =>
    route.fulfill({ json: { ...tag, description: "", createdAt: detail.createdAt, updatedAt: detail.updatedAt } })
  );
  await page.goto("/");
  await page.waitForLoadState("networkidle");
  await page.locator("input[type='text']").fill("demo@example.com");
  await page.locator("input[type='password']").fill("demodemo");
  await page.locator("button[type='submit']").click();
  await expect(page).toHaveURL("/home");
}
