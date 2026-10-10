import { expect, test, type Page } from "@playwright/test";

const rootId = "11111111-1111-1111-1111-111111111111";
const childId = "22222222-2222-2222-2222-222222222222";
const longName = "Maintenance " + "UnbrokenOrganizationName".repeat(10);

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

for (const theme of ["cinematic", "light", "homebox"]) {
  for (const width of [390, 834, 1440]) {
    test(`maintenance ${theme} ${width}: scope, status, CRUD, failure and keyboard focus`, async ({
      page,
      baseURL,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      await fixture(page, baseURL!, theme);
      const record = {
        id: "entry",
        itemID: rootId,
        itemName: "Stored item",
        name: longName,
        description: "Readable notes",
        cost: "12.50",
        scheduledDate: "2026-11-15",
        completedDate: "",
      };
      let entries = [record];
      let failedLoad = true;
      let failedSave = true;
      let failedDelete = true;
      const creates: Record<string, unknown>[] = [];
      const updates: Record<string, unknown>[] = [];
      const scopes: string[] = [];
      let deletions = 0;
      let releaseSave: (() => void) | undefined;
      let saveGate: Promise<void> | undefined;
      await page.route("**/api/v1/**maintenance**", async route => {
        const req = route.request();
        const url = new URL(req.url());
        if (req.method() === "GET") {
          scopes.push(url.pathname);
          if (failedLoad) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
          const status = url.searchParams.get("status");
          return route.fulfill({
            json: entries.filter(e => status === "both" || !!e.completedDate === (status === "completed")),
          });
        }
        if (req.method() === "POST") {
          creates.push(req.postDataJSON());
          if (failedSave) return route.fulfill({ status: 422, json: { message: "Invalid entry" } });
          const created = { ...record, ...req.postDataJSON(), id: "created" };
          entries = [created];
          return route.fulfill({ json: created });
        }
        if (req.method() === "PUT") {
          updates.push(req.postDataJSON());
          if (saveGate) await saveGate;
          if (failedSave) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
          Object.assign(entries[0]!, req.postDataJSON());
          return route.fulfill({ json: entries[0] });
        }
        if (req.method() === "DELETE") {
          deletions++;
          if (failedDelete) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
          entries = [];
          return route.fulfill({ status: 204 });
        }
        return route.fallback();
      });
      const type = { id: "item-type", name: "Item", isLocation: false };
      await page.route(`**/api/v1/entities/${rootId}`, route =>
        route.fulfill({
          json: {
            id: rootId,
            name: "Stored item",
            entityType: type,
            quantity: 1,
            parent: null,
            location: null,
            children: [],
            tags: [],
            fields: [],
            attachments: [],
            purchasePrice: 0,
            soldPrice: 0,
            description: "",
            notes: "",
            assetId: "000001",
            insured: false,
            archived: false,
          },
        })
      );
      await page.goto("/maintenance");
      await expect(page.getByRole("alert")).toContainText("Unable to load maintenance");
      failedLoad = false;
      await page.getByRole("button", { name: "Retry", exact: true }).click();
      await expect(page.getByText(longName, { exact: true })).toBeVisible();
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await page.screenshot({ path: test.info().outputPath("maintenance-long-name.png"), fullPage: true });
      const duplicate = page.getByRole("button", { name: "Duplicate", exact: true });
      await duplicate.focus();
      await page.keyboard.press("Enter");
      const copied = page.getByRole("dialog", { name: "New Entry" });
      await expect(copied.getByRole("textbox", { name: "Entry Name" })).toHaveValue(longName);
      await expect(copied.getByRole("textbox", { name: "Cost", exact: true })).toHaveValue("12.50");
      await page.keyboard.press("Escape");
      await expect(copied).toBeHidden();
      await expect(duplicate).toBeFocused();
      await expect(page.getByText("$12.50", { exact: true }).last()).toBeVisible();
      await expect(page.getByText("Total Entries", { exact: true }).locator("../..")).toContainText("1");
      await expect(page.getByText("November 15th, 2026", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Scheduled", exact: true })).toHaveAttribute(
        "aria-pressed",
        "true"
      );
      await page.getByRole("button", { name: "Completed", exact: true }).click();
      await expect(page.getByText("No maintenance entries match this status.")).toBeVisible();
      await page.getByRole("button", { name: "Both", exact: true }).click();
      await expect(page.getByText(longName, { exact: true })).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.getByRole("link", { name: "Stored item", exact: true }).click();
      await expect(page.getByRole("button", { name: "New", exact: true })).toBeVisible();
      await expect.poll(() => scopes).toContain(`/api/v1/entities/${rootId}/maintenance`);
      const edit = page.getByRole("button", { name: "Edit", exact: true }).last();
      await edit.focus();
      await page.keyboard.press("Enter");
      const dialog = page.getByRole("dialog", { name: "Edit Entry" });
      await expect(dialog.getByRole("textbox", { name: "Entry Name" })).toHaveValue(longName);
      await expect(dialog.getByRole("textbox", { name: "Cost", exact: true })).toHaveValue("12.50");
      // Stress actual controls with long translated labels without modifying saved locale preferences.
      await dialog.locator("label").evaluateAll(labels =>
        labels.forEach(label => {
          label.textContent = "Ausführliche Beschreibung der geplanten Instandhaltungsarbeiten und Kosten";
        })
      );
      await expect.poll(() => dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      saveGate = new Promise<void>(resolve => {
        releaseSave = resolve;
      });
      await dialog.getByRole("button", { name: "Update", exact: true }).press("Enter");
      await expect(dialog.getByRole("button", { name: "Loading…", exact: true })).toBeDisabled();
      await expect(dialog.locator("form")).toHaveAttribute("aria-busy", "true");
      releaseSave!();
      saveGate = undefined;
      await expect(dialog.getByRole("alert")).toHaveText("Failed to update entry");
      expect(updates[0]).toMatchObject({
        name: longName,
        cost: "12.50",
        scheduledDate: "2026-11-15",
        completedDate: "",
      });
      failedSave = false;
      await dialog.getByRole("button", { name: "Update", exact: true }).click();
      await expect(dialog).toBeHidden();
      await expect(edit).toBeFocused();
      const complete = page.getByRole("button", { name: "Complete", exact: true });
      await complete.click();
      await expect(page.getByText("No maintenance entries match this status.")).toBeVisible();
      expect(updates.at(-1)?.completedDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      await page.getByRole("button", { name: "Completed", exact: true }).click();
      await expect(page.getByText(longName, { exact: true })).toBeVisible();
      await expect(complete).toHaveCount(0);
      const remove = page.getByRole("button", { name: "Delete", exact: true }).last();
      await remove.focus();
      await page.keyboard.press("Enter");
      await page.getByRole("alertdialog").getByRole("button", { name: "Cancel", exact: true }).click();
      await expect(remove).toBeFocused();
      expect(deletions).toBe(0);
      await remove.click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.getByText("Failed to delete entry", { exact: true })).toBeVisible();
      await expect(remove).toBeFocused();
      failedDelete = false;
      await remove.click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.getByText("No maintenance entries match this status.")).toBeVisible();
      await expect(page.getByRole("button", { name: "Scheduled", exact: true })).toBeFocused();
      const create = page.getByRole("button", { name: "New", exact: true });
      await create.focus();
      await page.keyboard.press("Enter");
      const newDialog = page.getByRole("dialog", { name: "New Entry" });
      await newDialog.getByRole("textbox", { name: "Entry Name" }).fill("Created maintenance");
      await newDialog.getByRole("textbox", { name: "Cost", exact: true }).fill("42.25");
      failedSave = true;
      await newDialog.getByRole("button", { name: "Create", exact: true }).click();
      await expect(newDialog.getByRole("alert")).toHaveText("Failed to create entry");
      await expect(newDialog.getByRole("textbox", { name: "Entry Name" })).toHaveValue("Created maintenance");
      failedSave = false;
      await newDialog.getByRole("button", { name: "Create", exact: true }).click();
      await expect(newDialog).toBeHidden();
      await expect(create).toBeFocused();
      expect(creates.at(-1)).toMatchObject({
        name: "Created maintenance",
        cost: "42.25",
        completedDate: "",
        scheduledDate: "",
      });
      await page.getByRole("button", { name: "Scheduled", exact: true }).click();
      await expect(page.getByText("Created maintenance", { exact: true })).toBeVisible();
      await page.goto("/maintenance");
      await page.reload();
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await expect(page.getByText("Created maintenance", { exact: true })).toBeVisible();
      await page.screenshot({ path: test.info().outputPath("maintenance.png"), fullPage: true });
    });
  }
}

for (const theme of ["cinematic", "light", "homebox"]) {
  test(`organization ${theme}: measured shared surface and control contrast`, async ({ page, baseURL }) => {
    await fixture(page, baseURL!, theme);
    await page.goto("/maintenance");
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    const ratios = await page.evaluate(() => {
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
        ["foreground", "background", 4.5],
        ["card-foreground", "card", 4.5],
        ["muted-foreground", "card", 4.5],
        ["primary-foreground", "primary", 4.5],
        ["destructive-foreground", "destructive", 4.5],
        ["popover-foreground", "popover", 4.5],
        ["ring", "background", 3],
        ["input", "popover", 3],
      ] as const;
      const results = pairs.map(([fg, bg, threshold]) => {
        const a = luminance(rgb(fg));
        const b = luminance(rgb(bg));
        return { fg, bg, threshold, ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05) };
      });
      probe.remove();
      return results;
    });
    for (const result of ratios)
      expect(result.ratio, `${result.fg}/${result.bg}`).toBeGreaterThanOrEqual(result.threshold);
    await test
      .info()
      .attach("measured-contrast", { body: JSON.stringify(ratios, null, 2), contentType: "application/json" });
  });
}
