import { expect, test, type Page } from "@playwright/test";
import type { CsvRequest } from "../../lib/api/types/csv-contracts.generated";

test.use({ serviceWorkers: "block" });
const id = "11111111-1111-4111-8111-111111111111";
const oldHistoryId = "22222222-2222-4222-8222-222222222222";

async function fixture(page: Page, baseURL: string, noCollection = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const requests: { path: string; body: CsvRequest }[] = [];
  const template = {
    id,
    name: "Template 雪",
    description: "Long description",
    defaultQuantity: 1,
    fields: [{ id, name: "Color", textValue: "Blue" }],
    defaultTags: [],
  };
  await page.route("**/api/v1/**", route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    if (path.endsWith("/csv-exports") && route.request().method() === "POST") {
      requests.push({ path, body: route.request().postDataJSON() });
      return route.fulfill({ status: 202, json: { id: "artifact", status: "pending" } });
    }
    if (path.endsWith("/csv-exports/artifact"))
      return route.fulfill({ json: { id: "artifact", source: requests.at(-1)?.body.source, status: "completed" } });
    if (path.endsWith("/csv-exports/artifact/download"))
      return route.fulfill({ status: 403, json: { message: "CSV access denied" } });
    if (path.endsWith("/csv-exports"))
      return route.fulfill({ json: { items: [{ id: oldHistoryId, source: "api-keys", status: "completed" }] } });
    if (path === "/status") return route.fulfill({ json: { build: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({
        json: {
          item: {
            id: "user",
            name: "Member",
            email: "me@example.com",
            defaultGroupId: noCollection ? null : "group",
            group: noCollection ? null : { id: "group", name: "Collection", currency: "USD" },
          },
        },
      });
    if (path === "/users/self/settings")
      return route.fulfill({ json: { item: { theme: "cinematic", language: "en" } } });
    if (path === "/groups/all")
      return route.fulfill({ json: noCollection ? [] : [{ id: "group", name: "Collection" }] });
    if (path === "/groups") return route.fulfill({ json: { id: "group", name: "Collection", currency: "USD" } });
    if (path === "/templates") return route.fulfill({ json: [template] });
    if (path === `/templates/${id}`) return route.fulfill({ json: template });
    if (path === `/entities/${id}`)
      return route.fulfill({
        json: {
          id,
          name: "Item",
          fields: [{ id, name: "Color", type: "text", textValue: "Blue" }],
          attachments: [
            { id: oldHistoryId, title: "Photo", type: "photo", mimeType: "image/png", path: "private/path" },
          ],
          tags: [],
          quantity: 1,
        },
      });
    if (path === "/groups/invitations")
      return route.fulfill({
        json: [
          { id, expiresAt: "2099-01-01", uses: 3, token: "never-export-token" },
          { id: oldHistoryId, expiresAt: "2020-01-01", uses: 2 },
        ],
      });
    if (path === "/groups/members")
      return route.fulfill({ json: [{ id: "user", name: "Member", email: "me@example.com" }] });
    if (path === "/notifiers")
      return route.fulfill({
        json: [{ id, name: "Notifier", url: "secret://credential", isActive: true, createdAt: "2026-01-01" }],
      });
    if (path === "/entity-types") return route.fulfill({ json: [{ id, name: "Item", isLocation: false }] });
    if (path === "/users/self/api-keys")
      return route.fulfill({ json: [{ id, name: "Key", createdAt: "2026-01-01", lastUsedAt: null, expiresAt: null }] });
    if (path === "/entities")
      return route.fulfill({
        json: {
          items: [
            {
              id,
              name: "Item",
              assetId: "000-001",
              fields: [{ id, name: "Color", type: "text", textValue: "Blue" }],
              attachments: [
                { id: oldHistoryId, title: "Photo", type: "photo", mimeType: "image/png", path: "private/path" },
              ],
              tags: [],
              quantity: 1,
            },
          ],
          total: 1,
        },
      });
    if (path === "/entities/tree")
      return route.fulfill({ json: [{ id: oldHistoryId, name: "Storage", children: [] }] });
    if (path === "/tags")
      return route.fulfill({ json: [{ id: oldHistoryId, name: "Snow", parentId: null, color: "", description: "" }] });
    return route.fulfill({ json: [] });
  });
  return requests;
}

test("collection picker exports matching membership records without switching or closing", async ({
  page,
  baseURL,
}) => {
  const requests = await fixture(page, baseURL!);
  await page.goto("/profile");
  await page.getByRole("combobox", { name: "Select Collection", exact: true }).click();
  const input = page.getByPlaceholder("Search collections");
  await expect(input).toBeVisible();
  const picker = page.locator("[role=dialog]");
  const action = picker;
  const trigger = action.getByRole("button", { name: "Export CSV", exact: true });
  await trigger.focus();
  await trigger.press("Enter");
  await action.getByRole("button", { name: "Generate CSV", exact: true }).press("Enter");
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body.source).toBe("collections");
  expect(requests[0]!.body.scope).toBe("private");
  await expect(input).toBeVisible();
  expect(page.url()).toContain("/profile");
});

test("labels export real records privately but not blank print positions or dimensions", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto("/reports/label-generator");
  const action = page.locator("[data-csv-export]");
  await action.getByRole("button", { name: "Export CSV", exact: true }).click();
  await action.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body).toMatchObject({
    source: "label-records",
    scope: "private",
    boundary: { kind: "explicit", recordIds: [id] },
  });
  expect(JSON.stringify(requests[0]!.body)).not.toMatch(/cardWidth|skipLabels|baseURL|qrcode/);
  await page.emulateMedia({ media: "print" });
  await expect(action).toBeHidden();
});

test("item edit draft fields export without form submission; parent search stays open", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/item/${id}/edit`);
  await page.getByRole("switch", { name: "Advanced", exact: true }).click();
  // Export fields by finding their list section, not an arbitrary first selector.
  const customFields = page
    .getByRole("heading", { name: "Custom Fields", exact: true })
    .locator("xpath=ancestor::div[contains(@class, 'hb-panel')][1]");
  const action = customFields.locator("[data-csv-export]").first();
  await action.getByRole("button", { name: "Export CSV", exact: true }).click();
  await action.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body).toMatchObject({
    source: "draft-fields",
    scope: "private",
    snapshot: [{ name: "Color", type: "text", value: "Blue" }],
  });
  expect(page.url()).toContain(`/item/${id}/edit`);
});

test("parent-item picker captures complete search and exclusions without changing selection", async ({
  page,
  baseURL,
}) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/item/${id}/edit`);
  await page.getByRole("combobox", { name: "Parent Item", exact: true }).click();
  const picker = page.getByRole("dialog");
  const search = picker.getByRole("combobox");
  await search.fill("Item");
  await picker.getByRole("button", { name: "Export CSV", exact: true }).click();
  await picker.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body).toMatchObject({
    source: "entities",
    filters: { search: "Item" },
    boundary: { kind: "eligible", excludeIds: [id] },
  });
  await expect(search).toBeVisible();
  await expect(search).toHaveValue("Item");
  expect(page.url()).toContain("/edit");
});

test("location picker exports hierarchy matches without selecting a location", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/item/${id}/edit`);
  await page.getByRole("combobox", { name: "Parent Location", exact: true }).click();
  const picker = page.getByRole("dialog");
  const search = picker.getByRole("combobox");
  await search.fill("Storage");
  await picker.getByRole("button", { name: "Export CSV", exact: true }).click();
  await picker.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body).toMatchObject({
    source: "locations",
    boundary: { kind: "explicit", recordIds: [oldHistoryId] },
    columns: [
      { id: "name", label: "Name" },
      { id: "path", label: "Hierarchy path" },
    ],
  });
  await expect(search).toBeVisible();
});

test("entity-type picker exposes an accessible non-option export", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/item/${id}/edit`);
  await page.getByRole("combobox", { name: "Entity Type", exact: true }).click();
  await page.getByRole("button", { name: "Export CSV", exact: true }).last().click();
  await page.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body.source).toBe("entity-types");
  await expect(page.getByRole("option", { name: "Item", exact: true })).toBeVisible();
});

test("template selector exports matching records without applying defaults or closing", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto("/templates");
  await page.locator("#shell-create-trigger").click();
  await page.getByRole("menuitem", { name: /^Item/ }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("combobox", { name: "", exact: true }).click();
  const search = page.getByPlaceholder("Search templates…", { exact: true });
  await search.fill("Template");
  const picker = page.locator("[role=dialog]").filter({ has: search });
  await picker.getByRole("button", { name: "Export CSV", exact: true }).click();
  await picker.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body).toMatchObject({ source: "templates", boundary: { kind: "explicit", recordIds: [id] } });
  await expect(search).toBeVisible();
});

test("tag selector excludes synthetic create choices and leaves the picker open", async ({ page, baseURL }) => {
  const requests = await fixture(page, baseURL!);
  await page.goto(`/item/${id}/edit`);
  const search = page.getByRole("combobox", { name: "Tags", exact: true });
  await search.fill("Snow");
  const inputRoot = search.locator("xpath=ancestor::div[contains(@class,'flex-col')][1]");
  await inputRoot.getByRole("button", { name: "Export CSV", exact: true }).click();
  await inputRoot.getByRole("button", { name: "Generate CSV", exact: true }).click();
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.body).toMatchObject({
    source: "tags",
    boundary: { kind: "explicit", recordIds: [oldHistoryId] },
  });
  await expect(page.getByRole("option", { name: "Snow", exact: true })).toBeVisible();
  await expect(search).toHaveValue("Snow");
});
