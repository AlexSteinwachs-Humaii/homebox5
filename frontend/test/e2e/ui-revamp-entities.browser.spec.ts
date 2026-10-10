import { expect, test } from "@playwright/test";

// Original deterministic image bytes, served at the normal authenticated attachment URL.
const photo = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAGAAAABACAIAAABqVuVZAAAA/ElEQVR4nO3QMXEEARADwQPh2PHDMf86Lv5cAtAbaGoQ9PP+fk798/ee+uEiAxrQgAZ0aC4yoAENaECHHtCABjSgAQ1oQAMa0JG5yIAGNKABHXpAAxrQgAY0oAENaEBH5iIJ9H+s51oaJNMelQbJtEelQTLtUWmQTHtUGiTTHpUGybRHpUEy7VFpkEx7VBok0x6VBsm0R6VBMu1RaZBMe1QaJNMelQbJtEelQTLtUWmQTHtUGiTTHpUGybRHpUEy7VFpkEx7VBok0x6VBsm0R6VBMu1RaZBMe1QaJNMelQbJtEelQTLtUWmQTHtUGiTTHpUGybRHpUEy7ZF9AesnO+GjOuVmAAAAAElFTkSuQmCC",
  "base64"
);
for (const theme of ["cinematic", "homebox", "light"]) {
  for (const width of [390, 1440]) {
    test(`${theme} ${width}px truthful media and retryable operations`, async ({ page, baseURL }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await page.context().addCookies([
        { name: "hb.auth.session", value: "true", url: baseURL! },
        { name: "hb.auth.attachment_token", value: "fixture-token", url: baseURL! },
      ]);
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
        attachments: [
          {
            id: "photo",
            title: "Accurate photo",
            type: "photo",
            mimeType: "image/png",
            path: "photo.png",
            primary: true,
          },
          {
            id: "manual",
            title: "Manual" + "LongFileName".repeat(12),
            type: "manual",
            mimeType: "application/pdf",
            path: "manual.pdf",
          },
        ],
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
      let brokenPhoto = false;
      let failUpload = true;
      let failUpdate = true;
      let failDelete = true;
      let uploads = 0;
      let deletions = 0;
      const updates: Record<string, unknown>[] = [];
      await page.route("**/api/v1/**", route => {
        const request = route.request();
        const url = new URL(request.url());
        const path = url.pathname.replace("/api/v1", "");
        if (
          ["/entities/record/attachments", "/entities/created/attachments"].includes(path) &&
          request.method() === "POST"
        ) {
          uploads++;
          if (failUpload) return route.fulfill({ status: 413, json: { message: "Too large" } });
          record.attachments.push({
            id: "new",
            title: "new.png",
            type: "photo",
            mimeType: "image/png",
            path: "new.png",
            primary: false,
          });
          return route.fulfill({ json: record });
        }
        if (path.startsWith("/entities/record/attachments/")) {
          const id = path.split("/").pop();
          if (request.method() === "DELETE") {
            deletions++;
            if (failDelete) return route.fulfill({ status: 500, json: { message: "Delete failed" } });
            record.attachments = record.attachments.filter(a => a.id !== id);
            return route.fulfill({ status: 204 });
          }
          if (request.method() === "PUT") {
            updates.push(request.postDataJSON());
            if (failUpdate) return route.fulfill({ status: 500, json: { message: "Update failed" } });
            Object.assign(
              record.attachments.find(a => a.id === id)!,
              request.postDataJSON()
            );
            return route.fulfill({ json: record });
          }
          if (brokenPhoto) return route.fulfill({ status: 404 });
          return route.fulfill({ contentType: "image/png", body: photo });
        }
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
      await page.goto("/item/record");
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      const imageButton = page.getByRole("button", { name: "Photo", exact: true });
      await imageButton.focus();
      await page.keyboard.press("Enter");
      const viewer = page.getByRole("dialog", { name: "Photo", exact: true });
      await expect(viewer).toBeVisible();
      const viewerBounds = await viewer.boundingBox();
      expect(viewerBounds!.x).toBeGreaterThanOrEqual(0);
      expect(viewerBounds!.x + viewerBounds!.width).toBeLessThanOrEqual(width);
      await page.screenshot({ path: testInfo.outputPath("photo-viewer.png") });
      const image = viewer.locator("img");
      await expect.poll(() => image.evaluate(el => (el as HTMLImageElement).naturalWidth)).toBe(96);
      expect(
        await image.evaluate(el => {
          const canvas = document.createElement("canvas");
          canvas.width = 96;
          canvas.height = 64;
          const context = canvas.getContext("2d")!;
          context.drawImage(el as HTMLImageElement, 0, 0);
          return [...context.getImageData(0, 0, 1, 1).data];
        })
      ).toEqual([220, 30, 40, 255]);
      expect(
        await image.evaluate(el => {
          const css = getComputedStyle(el);
          return { filter: css.filter, opacity: css.opacity, mixBlendMode: css.mixBlendMode, objectFit: css.objectFit };
        })
      ).toEqual({ filter: "none", opacity: "1", mixBlendMode: "normal", objectFit: "contain" });
      await expect(viewer.getByRole("link", { name: "Open in new tab", exact: true })).toHaveAttribute(
        "href",
        /entities\/record\/attachments\/photo/
      );
      await expect(viewer.getByRole("link", { name: "Download", exact: true })).toHaveAttribute("download", "");
      await viewer.getByRole("button", { name: "Delete", exact: true }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Cancel", exact: true }).press("Enter");
      expect(deletions).toBe(0);
      await expect(viewer.getByRole("button", { name: "Delete", exact: true })).toBeFocused();
      await page.keyboard.press("Escape");
      await expect(viewer).toBeHidden();
      await expect(imageButton).toBeFocused();
      brokenPhoto = true;
      await page.reload();
      await imageButton.click();
      await expect(viewer.getByRole("status")).toContainText("Photo could not be loaded");
      await expect(viewer.getByRole("link", { name: "Download", exact: true })).toBeVisible();
      await page.keyboard.press("Escape");
      brokenPhoto = false;
      const download = page.getByRole("link", { name: /^Download: Manual/ });
      await expect(download).toHaveAttribute("href", /entities\/record\/attachments\/manual/);
      await expect(download).toHaveAttribute("download", /^Manual/);
      await page.reload();
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await page.goto("/item/record/edit");
      const name = page.getByRole("textbox", { name: "Name", exact: true }).first();
      await name.fill("Unsaved input retained");
      await page.getByRole("button", { name: /^Edit: Manual/ }).click();
      const editor = page.getByRole("dialog", { name: "Attachment Edit", exact: true });
      await expect(editor).toBeVisible();
      await editor.getByRole("textbox").fill("Renamed manual");
      await editor.getByRole("combobox", { name: "Attachment Type", exact: true }).click();
      await page.getByRole("option", { name: "Receipt", exact: true }).click();
      await editor.getByRole("button", { name: "Update", exact: true }).click();
      await expect(page.getByText("Failed to update attachment", { exact: true })).toBeVisible();
      await expect(editor.getByRole("textbox")).toHaveValue("Renamed manual");
      await expect(editor.getByRole("button", { name: "Update", exact: true })).toBeEnabled();
      failUpdate = false;
      await editor.getByRole("button", { name: "Update", exact: true }).click();
      await expect(editor).toBeHidden();
      await expect(page.getByRole("button", { name: "Edit: Renamed manual", exact: true })).toBeFocused();
      expect(updates[1]).toMatchObject({ title: "Renamed manual", type: "receipt" });
      await page.locator("input[type=file]").setInputFiles({ name: "new.png", mimeType: "image/png", buffer: photo });
      await expect(page.getByText("Attachment is too large", { exact: false })).toBeVisible();
      await expect(name).toHaveValue("Unsaved input retained");
      expect(uploads).toBe(1);
      failUpload = false;
      failSave = false;
      await page.locator("input[type=file]").setInputFiles({ name: "new.png", mimeType: "image/png", buffer: photo });
      await expect.poll(() => uploads).toBe(2);
      await expect(page.getByText("Attachment uploaded", { exact: true })).toBeVisible();
      await expect(name).toHaveValue("Unsaved input retained");
      await page.getByRole("button", { name: "Delete: Accurate photo", exact: true }).click();
      const confirmation = page.getByRole("alertdialog");
      await confirmation.getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.getByText("Failed to delete attachment", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Delete: Accurate photo", exact: true })).toBeVisible();
      failDelete = false;
      await page.getByRole("button", { name: "Delete: Accurate photo", exact: true }).click();
      await confirmation.getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.getByRole("button", { name: "Delete: Accurate photo", exact: true })).toBeHidden();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.screenshot({ path: testInfo.outputPath("media-editor.png"), fullPage: true });
      const contrast = await page.evaluate(() => {
        const probe = document.createElement("span");
        document.body.append(probe);
        const rgb = (token: string) => {
          probe.style.color = `hsl(var(--${token}))`;
          return getComputedStyle(probe)
            .color.match(/[\d.]+/g)!
            .slice(0, 3)
            .map(Number);
        };
        const luminance = (values: number[]) =>
          values
            .map(v => v / 255)
            .map(v => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
            .reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i]!, 0);
        const pairs = [
          ["foreground", "card", 4.5],
          ["muted-foreground", "card", 4.5],
          ["foreground", "popover", 4.5],
          ["muted-foreground", "popover", 4.5],
          ["primary", "background", 4.5],
          ["primary-foreground", "primary", 4.5],
          ["destructive-foreground", "destructive", 4.5],
          ["ring", "card", 3],
          ["input", "background", 3],
        ] as const;
        const result = pairs.map(([fg, bg, threshold]) => {
          const [lo, hi] = [luminance(rgb(fg)), luminance(rgb(bg))].sort((a, b) => a - b);
          return { pair: `${fg}/${bg}`, ratio: (hi! + 0.05) / (lo! + 0.05), threshold };
        });
        probe.remove();
        return result;
      });
      console.log(`${theme} ${width}px contrast: ${JSON.stringify(contrast)}`);
      await testInfo.attach("contrast.json", {
        body: JSON.stringify(contrast, null, 2),
        contentType: "application/json",
      });
      for (const { pair, ratio, threshold } of contrast) expect(ratio, pair).toBeGreaterThanOrEqual(threshold);
      record.attachments = record.attachments.filter(a => a.type !== "photo");
      await page.goto("/item/record");
      await expect(page.getByText("No photos added", { exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: "Download: Renamed manual", exact: true })).toBeVisible();
      await page.goto("/location/room");
      const createTrigger = page.getByRole("button", { name: "Create", exact: true });
      if (width < 768) await page.getByRole("button", { name: "Toggle sidebar" }).click();
      await createTrigger.click();
      await page.getByRole("menuitem", { name: /Item \/ Asset/ }).click();
      const createDialog = page
        .getByRole("dialog")
        .filter({ has: page.getByRole("heading", { name: /^Create(?: Item)?$/, exact: true }) });
      await createDialog.getByLabel(/Item Name/).fill("Photo draft");
      const picker = createDialog.locator("input[type=file]");
      await picker.focus();
      await expect
        .poll(() => picker.evaluate(el => getComputedStyle(el.parentElement!).boxShadow !== "none"))
        .toBe(true);
      await picker.setInputFiles({ name: "draft.png", mimeType: "image/png", buffer: photo });
      await expect(createDialog.getByRole("img", { name: "Uploaded Photo", exact: true })).toBeVisible();
      // No nested button triggers; each preview operation remains independently keyboard-accessible.
      expect(
        await createDialog
          .getByRole("img", { name: "Uploaded Photo", exact: true })
          .locator("../..")
          .locator("button button")
          .count()
      ).toBe(0);
      await createDialog.getByRole("button", { name: "Create and Add Another" }).click();
      await expect.poll(() => creations.length).toBe(1);
      await expect(createDialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue("Photo draft");
      await expect(createDialog.getByRole("img", { name: "Uploaded Photo", exact: true })).toBeVisible();
      await createDialog.getByRole("button", { name: "Rotate photo", exact: true }).focus();
      await page.keyboard.press("Enter");
      await expect(createDialog.getByRole("img", { name: "Uploaded Photo", exact: true })).toBeVisible();
      failCreate = false;
      failUpload = true;
      await createDialog.getByRole("button", { name: "Create and Add Another" }).click();
      await expect(createDialog.getByRole("status")).toContainText("The record was created");
      await expect(createDialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue("Photo draft");
      await expect(createDialog.getByRole("textbox", { name: "Item Name", exact: true })).toBeDisabled();
      await expect(createDialog.getByRole("img", { name: "Uploaded Photo", exact: true })).toBeVisible();
      expect(creations).toHaveLength(2);
      failUpload = false;
      await createDialog.getByRole("button", { name: "Retry Photos and Add Another", exact: true }).click();
      await expect(createDialog.getByRole("img", { name: "Uploaded Photo", exact: true })).toBeHidden();
      await expect(createDialog.getByRole("textbox", { name: "Item Name", exact: true })).toBeEnabled();
      await expect(createDialog.getByRole("textbox", { name: "Item Name", exact: true })).toHaveValue("");
      expect(creations).toHaveLength(2);
      expect(uploads).toBe(4);
      await picker.setInputFiles({ name: "draft.png", mimeType: "image/png", buffer: photo });
      await createDialog.getByRole("button", { name: "Delete photo", exact: true }).press("Enter");
      await expect(createDialog.getByRole("img", { name: "Uploaded Photo", exact: true })).toBeHidden();
      await page.keyboard.press("Escape");
      await expect(createDialog).toBeHidden();
      await expect(createTrigger).toBeFocused();
    });
  }
}
