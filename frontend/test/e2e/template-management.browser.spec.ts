import { expect, test, type Page } from "@playwright/test";

const rootId = "11111111-1111-1111-1111-111111111111";
const childId = "22222222-2222-2222-2222-222222222222";
const longName = "Deep " + "UnbrokenOrganizationName".repeat(10);

async function fixture(page: Page, baseURL: string, theme: string, failed = false, empty = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const tags = empty
    ? []
    : [
        { id: rootId, name: "Root tag", color: "#ffffff", icon: "palette-outline", parentId: null },
        { id: childId, name: longName, color: "#000000", icon: "palette-outline", parentId: rootId },
      ];
  await page.route("**/api/v1/**", route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
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
    if (path === "/tags" || path === "/entities/tree") {
      if (failed) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
      return route.fulfill({
        json:
          path === "/tags"
            ? tags
            : empty
              ? []
              : [
                  {
                    id: rootId,
                    name: "Root location",
                    type: "location",
                    children: [
                      {
                        id: childId,
                        name: longName,
                        type: "location",
                        children: [
                          {
                            id: "33333333-3333-3333-3333-333333333333",
                            name: "Stored item",
                            type: "item",
                            children: [],
                          },
                        ],
                      },
                    ],
                  },
                ],
      });
    }
    return route.fulfill({ json: [] });
  });
}

const templateId = "55555555-5555-5555-5555-555555555555";
const fieldId = "66666666-6666-6666-6666-666666666666";
const nil = "00000000-0000-0000-0000-000000000000";
function templateRecord() {
  return {
    id: templateId,
    name: longName,
    description: "Reusable defaults",
    notes: "Retained notes",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    defaultName: "Default item",
    defaultDescription: "Default description",
    defaultQuantity: 3,
    defaultInsured: true,
    defaultManufacturer: "Manufacturer",
    defaultModelNumber: "Model",
    defaultLifetimeWarranty: true,
    defaultWarrantyDetails: "Retained warranty details",
    defaultLocation: { id: rootId, name: "Root location" },
    defaultTags: [{ id: rootId, name: "Root tag" }],
    includeWarrantyFields: true,
    includePurchaseFields: true,
    includeSoldFields: true,
    fields: [{ id: fieldId, name: "Custom default", type: "text", textValue: "Retained value" }],
  };
}

for (const theme of ["cinematic", "light", "homebox"]) {
  for (const width of [390, 1440]) {
    test(`templates ${theme} ${width}: duplicate, edit defaults, failure, confirmation and refresh`, async ({
      page,
      baseURL,
    }, testInfo) => {
      await page.setViewportSize({ width, height: 844 });
      await fixture(page, baseURL!, theme);
      const record = templateRecord();
      let list = [record];
      let duplicate: Record<string, unknown> = {};
      let updated: Record<string, unknown> = {};
      let failSave = true;
      let failDelete = true;
      let deletions = 0;
      await page.route("**/api/v1/templates**", route => {
        const request = route.request();
        const path = new URL(request.url()).pathname;
        if (request.method() === "POST") {
          duplicate = request.postDataJSON();
          list = [record, { ...record, id: childId, name: String(duplicate.name) }];
          return route.fulfill({ json: list[1] });
        }
        if (request.method() === "PUT") {
          updated = request.postDataJSON();
          if (failSave) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
          record.name = String(updated.name);
          return route.fulfill({ json: record });
        }
        if (request.method() === "DELETE") {
          deletions++;
          if (failDelete) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
          list = [];
          return route.fulfill({ status: 204 });
        }
        return route.fulfill({ json: path.endsWith("/templates") ? list : record });
      });
      await page.goto("/templates");
      await expect(page.getByRole("heading", { name: longName, exact: true })).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const copy = page.getByRole("button", { name: "Duplicate template" });
      await copy.focus();
      await page.keyboard.press("Enter");
      await expect(page.getByRole("heading", { name: `${longName} (Copy)`, exact: true })).toBeVisible();
      expect(duplicate).toMatchObject({
        defaultLocationId: rootId,
        defaultTagIds: [rootId],
        defaultQuantity: 3,
        defaultInsured: true,
        defaultLifetimeWarranty: true,
        defaultWarrantyDetails: record.defaultWarrantyDetails,
        notes: record.notes,
        includeWarrantyFields: true,
        includePurchaseFields: true,
        includeSoldFields: true,
        fields: [{ id: nil, name: "Custom default", type: "text", textValue: "Retained value" }],
      });
      await page.getByRole("link", { name: "Edit template", exact: true }).first().click();
      await expect(page.getByRole("heading", { name: longName, exact: true })).toBeVisible();
      await expect(page.getByText("Root location", { exact: true })).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.getByRole("button", { name: "Edit", exact: true }).click();
      const dialog = page.getByRole("dialog", { name: "Edit Template" });
      await expect(dialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue("Default item");
      await expect(dialog.getByRole("spinbutton", { name: "Quantity", exact: true })).toHaveValue("3");
      await expect(dialog.getByRole("switch", { name: "Lifetime Warranty", exact: true })).toBeChecked();
      await expect(dialog.getByRole("textbox", { name: "Default Value", exact: true })).toHaveValue("Retained value");
      await dialog.getByRole("textbox", { name: "Template Name", exact: true }).fill("Updated template");
      await dialog.getByRole("button", { name: "Update", exact: true }).click();
      await expect(page.getByText("Failed to update template", { exact: true })).toBeVisible();
      await expect(dialog).toBeVisible();
      expect(updated).toMatchObject({
        defaultLocationId: rootId,
        defaultTagIds: [rootId],
        defaultQuantity: 3,
        defaultInsured: true,
        defaultLifetimeWarranty: true,
        defaultWarrantyDetails: record.defaultWarrantyDetails,
        notes: record.notes,
        includeWarrantyFields: true,
        includePurchaseFields: true,
        includeSoldFields: true,
        fields: record.fields,
      });
      await page.screenshot({ path: testInfo.outputPath("template-editor.png"), fullPage: true });
      failSave = false;
      await dialog.getByRole("button", { name: "Update", exact: true }).click();
      await expect(dialog).toHaveCount(0);
      await expect(page.getByRole("heading", { name: "Updated template", exact: true })).toBeVisible();
      await page.getByRole("button", { name: "Delete", exact: true }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Cancel", exact: true }).click();
      expect(deletions).toBe(0);
      await page.getByRole("button", { name: "Delete", exact: true }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.getByText("Failed to delete template", { exact: true })).toBeVisible();
      failDelete = false;
      await page.getByRole("button", { name: "Delete", exact: true }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page).toHaveURL(/\/templates$/);
      await expect(page.getByRole("status")).toContainText("No templates");
      expect(deletions).toBe(2);
    });
  }
}

test("template list failed load has retry, then empty create and retained failed form", async ({ page, baseURL }) => {
  await page.setViewportSize({ width: 390, height: 700 });
  await fixture(page, baseURL!, "cinematic");
  let failLoad = true;
  let failCreate = true;
  let saved: Record<string, unknown> = {};
  let records: Record<string, unknown>[] = [];
  let releaseCreate: () => void = () => {};
  const createGate = new Promise<void>(resolve => {
    releaseCreate = resolve;
  });
  await page.route("**/api/v1/templates", async route => {
    if (route.request().method() === "POST") {
      saved = route.request().postDataJSON();
      await createGate;
      if (failCreate) return route.fulfill({ status: 400, json: { message: "Invalid template" } });
      records = [{ ...saved, id: templateId }];
      return route.fulfill({ json: records[0] });
    }
    return failLoad
      ? route.fulfill({ status: 500, json: { message: "Unavailable" } })
      : route.fulfill({ json: records });
  });
  await page.goto("/templates");
  await expect(page.getByRole("alert")).toContainText("Failed to load template details");
  await expect(page.getByRole("status")).toHaveCount(0);
  failLoad = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await page.getByRole("status").getByRole("button", { name: "Create Template", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Create Template" });
  await dialog.getByRole("textbox", { name: "Template Name", exact: true }).fill("New template");
  await dialog.getByRole("textbox", { name: "Item Name", exact: true }).fill("New default");
  await dialog.getByRole("combobox", { name: "Parent Location" }).click();
  await page.getByRole("option", { name: "Root location", exact: true }).click();
  await dialog.getByRole("combobox", { name: "Tags", exact: true }).click();
  await page.getByRole("option", { name: "Root tag", exact: true }).click();
  await page.keyboard.press("Escape");
  await dialog.getByRole("switch", { name: "Lifetime Warranty", exact: true }).click();
  await dialog.getByRole("button", { name: "Add", exact: true }).click();
  await dialog.getByRole("textbox", { name: "Field Name", exact: true }).fill("Custom default");
  await dialog.getByRole("textbox", { name: "Default Value", exact: true }).fill("Custom value");
  await dialog.getByRole("button", { name: "Create", exact: true }).press("Enter");
  await expect(dialog.getByRole("button", { name: "Loading…", exact: true })).toBeDisabled();
  await expect(dialog.locator("form")).toHaveAttribute("aria-busy", "true");
  releaseCreate();
  await expect(page.getByText("Failed to create template", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("textbox", { name: "Template Name", exact: true })).toHaveValue("New template");
  expect(saved).toMatchObject({
    defaultName: "New default",
    defaultLocationId: rootId,
    defaultTagIds: [rootId],
    defaultLifetimeWarranty: true,
    fields: [{ id: nil, name: "Custom default", type: "text", textValue: "Custom value" }],
  });
  failCreate = false;
  await dialog.getByRole("button", { name: "Create", exact: true }).press("Enter");
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "New template", exact: true })).toBeVisible();
});

test("existing item template selector searches, toggles, restores and sends unchanged defaults", async ({
  page,
  baseURL,
}) => {
  await fixture(page, baseURL!, "cinematic");
  const record = { ...templateRecord(), name: "Edited reusable template" };
  const typeId = "77777777-7777-7777-7777-777777777777";
  let itemRequest: Record<string, unknown> = {};
  await page.route("**/api/v1/entity-types", route =>
    route.fulfill({ json: [{ id: typeId, name: "global.item", isLocation: false }] })
  );
  await page.route("**/api/v1/entities?**", route =>
    route.fulfill({ json: { items: [{ id: rootId, name: "Root location" }], total: 1 } })
  );
  await page.route("**/api/v1/templates**", route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/create-item")) {
      itemRequest = route.request().postDataJSON();
      return route.fulfill({ status: 500, json: { message: "Unavailable" } });
    }
    return route.fulfill({ json: path.endsWith("/templates") ? [record] : record });
  });
  await page.goto("/templates");
  await expect(page.getByRole("heading", { name: record.name, exact: true })).toBeVisible();
  await page.locator("#shell-create-trigger").click();
  await page.getByRole("menuitem", { name: /^Item/ }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("combobox", { name: "Entity Type" })).toContainText("Item");
  const selector = dialog.getByRole("combobox", { name: "", exact: true });
  await selector.click();
  await page.getByPlaceholder("Search templates…", { exact: true }).fill("Edited");
  await page.getByRole("option", { name: /Edited reusable template/ }).click();
  await expect(dialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue(record.defaultName);
  await expect(dialog.getByRole("spinbutton", { name: "Item Quantity", exact: true })).toHaveValue("3");
  await dialog.getByRole("button", { name: "Show defaults", exact: true }).click();
  await expect(dialog.getByText("Custom default:", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Retained value", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Lifetime Warranty:", { exact: true })).toBeVisible();
  await expect(dialog.getByRole("combobox", { name: "Parent Location" })).toContainText("Root location");
  await selector.click();
  await page.getByRole("option", { name: /Edited reusable template/ }).click();
  await expect(dialog.getByRole("spinbutton", { name: "Item Quantity", exact: true })).toHaveValue("1");
  await selector.click();
  await page.getByRole("option", { name: /Edited reusable template/ }).click();
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  await expect.poll(() => itemRequest.quantity).toBe(3);
  expect(itemRequest).toEqual({
    name: record.defaultName,
    description: record.defaultDescription,
    parentId: rootId,
    tagIds: [rootId],
    quantity: 3,
    entityTypeId: typeId,
  });
  await page.reload();
  await expect(page.getByRole("heading", { name: record.name, exact: true })).toBeVisible();
  await page.locator("#shell-create-trigger").click();
  await page.getByRole("menuitem", { name: /^Item/ }).click();
  await expect(dialog.getByRole("spinbutton", { name: "Item Quantity", exact: true })).toHaveValue("3");
  await expect(
    dialog.getByRole("heading", { name: "Using template: Edited reusable template", exact: true })
  ).toBeVisible();
});
