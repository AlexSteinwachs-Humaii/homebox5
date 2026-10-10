import { expect, test } from "@playwright/test";

for (const theme of ["cinematic", "homebox", "light"]) {
  for (const width of [390, 1440]) {
    test(`${theme} ${width}px entity forms preserve saves and keyboard controls`, async ({
      page,
      baseURL,
    }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL! }]);
      const types = [
        { id: "item-type", name: "Item", isLocation: false },
        { id: "location-type", name: "Location", isLocation: true },
      ];
      const location = { id: "room", name: "Room", entityType: types[1], parent: null, children: [], tags: [] };
      const record = {
        id: "record",
        name: "Original record",
        entityType: types[0],
        quantity: 2,
        parent: location,
        location,
        description: "Description",
        notes: "Notes",
        assetId: "000001",
        attachments: [],
        children: [],
        tags: [],
        fields: [{ name: "Custom name", textValue: "Stored value" }],
        purchasePrice: 12,
        soldPrice: 0,
        insured: false,
        archived: false,
        syncChildEntityLocations: false,
        purchaseDate: "2026-04-18",
        warrantyExpires: "",
        soldDate: "",
      };
      const payloads: Record<string, unknown>[] = [];
      let failSave = true;
      const creations: Record<string, unknown>[] = [];
      let failCreate = true;
      const locationSaves: Record<string, unknown>[] = [];
      await page.route("**/api/v1/**", route => {
        const request = route.request();
        const url = new URL(request.url());
        const path = url.pathname.replace("/api/v1", "");
        if (path === "/status")
          return route.fulfill({ json: { build: { version: "v1.0.0" }, latest: { version: "v1.0.0" } } });
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
          return route.fulfill({ json: { item: { theme, displayLegacyHeader: false, editorAdvancedView: true } } });
        if (path === "/groups/all") return route.fulfill({ json: [{ id: "group", name: "Collection" }] });
        if (path === "/groups") return route.fulfill({ json: { id: "group", name: "Collection", currency: "USD" } });
        if (path === "/entity-types") return route.fulfill({ json: types });
        if (request.method() === "PUT" && path === "/entities/record") {
          payloads.push(request.postDataJSON());
          if (failSave) return route.fulfill({ status: 400, json: { message: "Save failed" } });
          Object.assign(record, request.postDataJSON());
          return route.fulfill({ json: record });
        }
        if (request.method() === "POST" && path === "/entities") {
          creations.push(request.postDataJSON());
          if (failCreate) return route.fulfill({ status: 400, json: { message: "Create failed" } });
          return route.fulfill({ json: { ...record, ...request.postDataJSON(), id: "created" } });
        }
        if (path === "/entities/record") return route.fulfill({ json: record });
        if (request.method() === "PUT" && path === "/entities/room") {
          locationSaves.push(request.postDataJSON());
          return route.fulfill({ json: { ...record, ...location, ...request.postDataJSON() } });
        }
        if (path === "/entities/room") return route.fulfill({ json: { ...record, ...location } });
        if (path === "/entities/tree") return route.fulfill({ json: [] });
        if (path === "/entities")
          return route.fulfill({ json: { items: [location], total: 1, page: 1, pageSize: 10 } });
        if (path.endsWith("/path")) return route.fulfill({ json: [] });
        return route.fulfill({ json: [] });
      });
      await page.goto("/item/record/edit");
      await expect(page.getByRole("textbox", { name: "Name", exact: true }).first()).toHaveValue("Original record");
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await page.getByRole("textbox", { name: "Name", exact: true }).first().fill("Changed record");
      await page.getByLabel("Preview", { exact: true }).first().focus();
      await page.keyboard.press("Space");
      await expect(page.getByLabel("Preview", { exact: true }).first()).toBeChecked();
      await expect(page.getByLabel("Purchase Date", { exact: true })).toBeVisible();
      const save = page.getByRole("button", { name: "Save", exact: true });
      await save.click();
      await expect(page.getByText("Failed to save item", { exact: false })).toBeVisible();
      await expect(page.getByRole("textbox", { name: "Name", exact: true }).first()).toHaveValue("Changed record");
      expect(payloads[0]).toMatchObject({
        name: "Changed record",
        parentId: "room",
        quantity: 2,
        purchaseDate: "2026-04-18",
        entityTypeId: "item-type",
        tagIds: [],
      });
      await page.getByRole("combobox", { name: "Entity Type", exact: true }).click();
      await page.getByRole("option", { name: "Location", exact: true }).click();
      await save.click();
      const confirmation = page.getByRole("alertdialog");
      await expect(confirmation).toBeVisible();
      await confirmation.getByRole("button", { name: "Cancel", exact: true }).press("Enter");
      await expect(confirmation).toBeHidden();
      await expect(save).toBeFocused();
      expect(payloads).toHaveLength(1);
      await page.getByRole("combobox", { name: "Entity Type", exact: true }).click();
      await page.getByRole("option", { name: "Item", exact: true }).click();
      failSave = false;
      await page.keyboard.press("Control+Shift+s");
      await expect.poll(() => payloads.length).toBe(2);
      await expect(page).toHaveURL(/\/edit$/);
      await expect(save).toBeEnabled();
      await page.getByLabel("Advanced", { exact: true }).click();
      await expect(page.getByText("Custom Fields", { exact: true })).toBeHidden();
      await page.getByLabel("Advanced", { exact: true }).click();
      await expect(page.getByText("Custom Fields", { exact: true })).toBeVisible();
      await expect(page.getByRole("textbox", { name: /^Description/ })).toBeVisible();
      for (const control of await page.locator("main input, main textarea").all()) {
        if (!(await control.isVisible())) continue;
        const box = await control.boundingBox();
        expect(box!.x).toBeGreaterThanOrEqual(0);
        expect(box!.x + box!.width).toBeLessThanOrEqual(width);
      }
      await page.screenshot({ path: testInfo.outputPath("item-editor.png"), fullPage: true });
      await save.click();
      await expect(page).toHaveURL(/\/item\/record$/);
      await page.goto("/location/room/edit");
      await expect(page.getByRole("textbox", { name: "Name", exact: true }).first()).toHaveValue("Room");
      await page.screenshot({ path: testInfo.outputPath("location-editor.png"), fullPage: true });
      await page.keyboard.press("Control+Shift+s");
      await expect.poll(() => locationSaves.length).toBe(1);
      expect(locationSaves[0]).toMatchObject({
        name: "Room",
        parentId: null,
        entityTypeId: "location-type",
        tagIds: [],
      });
      await expect(page).toHaveURL(/\/location\/room\/edit$/);
      const createTrigger = page.getByRole("button", { name: "Create", exact: true });
      if (width < 768) await page.getByRole("button", { name: "Toggle sidebar" }).click();
      await createTrigger.click();
      await page.getByRole("menuitem", { name: /Item \/ Asset/ }).click();
      const dialog = page
        .getByRole("dialog")
        .filter({ has: page.getByRole("heading", { name: "Create", exact: true }) });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByLabel(/Item Name/)).toBeVisible();
      await dialog.getByLabel(/Item Name/).fill("Draft");
      await dialog.getByRole("button", { name: "Create and Add Another" }).scrollIntoViewIfNeeded();
      await expect(dialog.getByRole("button", { name: "Create and Add Another" })).toBeVisible();
      await expect(dialog.getByRole("textbox", { name: "Item Description", exact: true })).toBeVisible();
      await dialog.getByRole("button", { name: "Create and Add Another" }).click();
      await expect.poll(() => creations.length).toBe(1);
      await expect(dialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue("Draft");
      expect(creations[0]).toMatchObject({
        name: "Draft",
        quantity: 1,
        parentId: "room",
        entityTypeId: "item-type",
        tagIds: [],
      });
      failCreate = false;
      await dialog.getByRole("button", { name: "Create and Add Another" }).click();
      await expect.poll(() => creations.length).toBe(2);
      await expect(dialog).toBeVisible();
      await expect(dialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue("");
      await page.screenshot({ path: testInfo.outputPath("create-form.png") });
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(createTrigger).toBeFocused();
    });
  }
}
