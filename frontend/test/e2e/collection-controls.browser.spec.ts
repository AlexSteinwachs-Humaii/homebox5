import { expect, test, type Page } from "@playwright/test";

const longName = "Gemeinsame Sammlung " + "LangerübersetzterSammlungsname".repeat(7);

async function fixture(page: Page, baseURL: string, theme: string, onlyMember = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const groups = [
    { id: "first", name: longName, currency: "USD" },
    { id: "second", name: "Second collection", currency: "EUR" },
  ];
  const updates: unknown[] = [];
  const creates: unknown[] = [];
  const invites: unknown[] = [];
  const joins: string[] = [];
  const reads: string[] = [];
  let deletes = 0;
  let failSave = false;
  await page.route("**/api/v1/**", route => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace("/api/v1", "");
    const current = req.headers()["x-tenant"] === "second" ? groups[1]! : groups[0]!;
    if (req.method() === "DELETE") {
      deletes++;
      return route.fulfill({ status: 204 });
    }
    if (path === "/status")
      return route.fulfill({ json: { build: { version: "v1.0.0" }, latest: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({
        json: { item: { id: "user", name: "Member", defaultGroupId: "first", group: groups[0] } },
      });
    if (path === "/users/self/settings")
      return route.fulfill({ json: { item: { theme, displayLegacyHeader: false } } });
    if (path === "/groups/all") return route.fulfill({ json: groups });
    if (path === "/groups/members") {
      reads.push(`members:${current.id}`);
      return route.fulfill({ json: onlyMember ? [{ id: "user" }] : [{ id: "user" }, { id: "other" }] });
    }
    if (path === "/currencies")
      return route.fulfill({
        json: [
          { code: "USD", name: "US Dollar" },
          { code: "EUR", name: "Euro" },
        ],
      });
    if (path === "/groups") {
      if (req.method() === "PUT") {
        updates.push(req.postDataJSON());
        if (failSave) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
        Object.assign(current, req.postDataJSON());
      } else if (req.method() === "POST") {
        creates.push(req.postDataJSON());
        return route.fulfill({ status: 422, json: { message: "Unavailable" } });
      } else reads.push(`settings:${current.id}`);
      return route.fulfill({ json: current });
    }
    if (path === "/groups/invitations" && req.method() === "POST") {
      invites.push(req.postDataJSON());
      return route.fulfill({ status: 500, json: { message: "Unavailable" } });
    }
    if (path.startsWith("/groups/invitations/") && req.method() === "POST") {
      joins.push(path);
      return route.fulfill({ status: 422, json: { message: "Unavailable" } });
    }
    return route.fulfill({ json: [] });
  });
  return { updates, creates, invites, joins, reads, deletes: () => deletes, failSave: () => (failSave = true) };
}

async function noOverflow(page: Page) {
  expect(await page.locator("main").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
}

for (const theme of ["light", "cinematic"]) {
  for (const width of [390, 834, 1440]) {
    test(`collection settings ${theme} ${width}: tabs, saving, errors and confirmation`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 1000 });
      const api = await fixture(page, baseURL!, theme, width === 834);
      await page.goto("/collection/settings");
      await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue(longName);
      const nav = page.getByRole("navigation", { name: "Collection Options", exact: true });
      await expect(nav.getByRole("link")).toHaveCount(6);
      await expect(nav.getByRole("link", { name: "Settings", exact: true })).toHaveAttribute("aria-current", "page");
      const action = page.getByRole("button", {
        name: width === 834 ? "Delete Collection" : "Leave Collection",
        exact: true,
      });
      await action.focus();
      await expect.poll(() => action.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
      await action.press("Enter");
      const confirm = page.getByRole("alertdialog");
      await expect(confirm).toBeVisible();
      await confirm.getByRole("button", { name: "Cancel", exact: true }).press("Enter");
      expect(api.deletes()).toBe(0);
      await noOverflow(page);
      await page.getByRole("textbox", { name: "Name", exact: true }).fill("Updated collection");
      await page.getByRole("combobox", { name: "Currency Format", exact: true }).click();
      await page.getByRole("option", { name: "Euro", exact: true }).click();
      await page.getByRole("button", { name: "Update Group", exact: true }).press("Enter");
      await expect.poll(() => api.updates.length).toBe(1);
      expect(api.updates[0]).toEqual({ name: "Updated collection", currency: "EUR" });
      await expect(page.getByRole("heading", { name: "Manage Collection - Updated collection" })).toBeVisible();
      api.failSave();
      await page.getByRole("button", { name: "Update Group", exact: true }).click();
      await expect(page.getByRole("alert").filter({ hasText: "Failed to update group" }).first()).toBeVisible();
    });
  }

  test(`collection switching and entry dialogs ${theme}`, async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    const api = await fixture(page, baseURL!, theme);
    await page.goto("/collection/settings");
    const selector = page.getByRole("combobox", { name: "Select Collection", exact: true });
    await expect(selector).toContainText(longName);
    await selector.click();
    await page.getByRole("option", { name: "Second collection", exact: true }).click();
    await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("Second collection");
    expect(api.reads).toContain("settings:second");
    expect(api.reads).toContain("members:second");
    await selector.click();
    await page.getByText("Create New Collection", { exact: true }).click();
    let dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("textbox")).toBeFocused();
    await dialog.getByRole("textbox").fill("  New collection  ");
    await dialog.getByRole("button", { name: "Create", exact: true }).press("Enter");
    await expect(dialog.getByRole("alert")).toBeVisible();
    expect(api.creates[0]).toEqual({ name: "New collection" });
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await selector.click();
    await page.getByText("Join Existing Collection", { exact: true }).click();
    dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox").fill("invalid");
    await dialog.getByRole("button", { name: "Join Collection", exact: true }).press("Enter");
    await expect(dialog.getByRole("alert")).toBeVisible();
    expect(api.joins).toHaveLength(0);
    await dialog.getByRole("textbox").fill("https://example.com?token=AYQ4W4K5MT4CZOPB2PZRCZ4PTY");
    await dialog.getByRole("button", { name: "Join Collection", exact: true }).click();
    await expect.poll(() => api.joins.length).toBe(1);
    expect(api.joins[0]).toBe("/groups/invitations/AYQ4W4K5MT4CZOPB2PZRCZ4PTY");
    await page.keyboard.press("Escape");
    await page.goto("/collection/invites");
    await page.getByRole("button", { name: "Create Invite", exact: true }).click();
    dialog = page.getByRole("dialog", { name: "Create Invite", exact: true });
    await expect(dialog.getByRole("textbox", { name: "Expires", exact: true })).toBeVisible();
    await dialog.getByRole("spinbutton", { name: "Uses", exact: true }).fill("101");
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect(dialog.getByRole("alert")).toBeVisible();
    expect(api.invites).toHaveLength(0);
    await dialog.getByRole("spinbutton", { name: "Uses", exact: true }).fill("2");
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect.poll(() => api.invites.length).toBe(1);
    expect(api.invites[0]).toMatchObject({ uses: 2 });
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });
}

for (const theme of ["light", "cinematic"]) {
  test(`collection entry drawers ${theme} 390: focus, errors and wrapping`, async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 390, height: 1000 });
    await fixture(page, baseURL!, theme);
    await page.goto("/collection/settings");
    await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue(longName);
    await page.getByRole("button", { name: "Toggle sidebar", exact: true }).click();
    await page.getByRole("combobox", { name: "Select Collection", exact: true }).click();
    await page.getByText("Join Existing Collection", { exact: true }).click();
    let dialog = page.getByRole("dialog", { name: "Join Collection", exact: true });
    await expect(dialog.getByRole("textbox")).toBeVisible();
    for (let i = 0; i < 12 && !(await dialog.getByRole("textbox").evaluate(el => el === document.activeElement)); i++) {
      await page.keyboard.press("Tab");
    }
    await expect(dialog.getByRole("textbox")).toBeFocused();
    await dialog.getByRole("textbox").fill("invalid");
    const join = dialog.getByRole("button", { name: "Join Collection", exact: true });
    await join.focus();
    await expect.poll(() => join.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
    await join.press("Enter");
    await expect(dialog.getByRole("alert")).toBeVisible();
    expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await dialog.getByRole("button", { name: "No invite code? Create a new collection instead", exact: true }).click();
    dialog = page.getByRole("dialog", { name: "Create Collection", exact: true });
    await expect(dialog.getByRole("textbox")).toBeFocused();
    await dialog.getByRole("textbox").fill(longName);
    expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await page.screenshot({ path: test.info().outputPath("collection-create-mobile.png"), fullPage: true });
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await page.goto("/collection/invites");
    await page.getByRole("button", { name: "Create Invite", exact: true }).click();
    dialog = page.getByRole("dialog", { name: "Create Invite", exact: true });
    await expect(dialog.getByRole("textbox", { name: "Expires", exact: true })).toBeVisible();
    expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await page.screenshot({ path: test.info().outputPath("collection-invite-mobile.png"), fullPage: true });
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });
}
