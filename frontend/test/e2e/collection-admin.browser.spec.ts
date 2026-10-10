import { expect, test, type Page } from "@playwright/test";

const longName = "Administration".repeat(30);
const token = "A".repeat(220);

async function fixture(page: Page, baseURL: string, theme: string) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const group = { id: "first", name: "Shared collection", currency: "USD" };
  let members = [
    { id: "user", name: "Current member", email: "member@example.com" },
    { id: "other", name: longName, email: longName + "@example.com" },
  ];
  let invites = [
    { id: "visible", token, expiresAt: "2099-01-01T00:00:00Z", uses: 3 },
    { id: "hidden", expiresAt: "2099-01-01T00:00:00Z", uses: 2 },
  ];
  let notifiers = [
    { id: "notifier", name: longName, url: "https://example.com/notify", isActive: false, createdAt: "2026-01-01" },
  ];
  let types = [
    { id: "type", name: longName, icon: "", isLocation: false, defaultTemplate: { id: "template", name: longName } },
  ];
  const writes: { path: string; method: string; body: unknown }[] = [];
  let failPath = "";
  let failReads = false;
  await page.route("**/api/v1/**", route => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace("/api/v1", "");
    const method = req.method();
    if (method !== "GET" && path === "/users/self/settings")
      return route.fulfill({ json: { item: req.postDataJSON() } });
    if (method !== "GET") {
      writes.push({ path, method, body: req.postData() ? req.postDataJSON() : null });
      if (path === failPath) return route.fulfill({ status: 403, json: { message: "Not permitted" } });
      if (method === "DELETE") {
        members = members.filter(m => !path.endsWith(m.id));
        invites = invites.filter(i => !path.endsWith(i.id));
        notifiers = notifiers.filter(n => !path.endsWith(n.id));
        types = types.filter(et => !path.endsWith(et.id));
        return route.fulfill({ status: 204 });
      }
      if (path === "/notifiers" && method === "POST")
        notifiers.push({ id: "new", createdAt: "2026-01-01", ...req.postDataJSON() });
      if (path === "/notifiers/notifier" && method === "PUT") Object.assign(notifiers[0]!, req.postDataJSON());
      if (path === "/entity-types" && method === "POST") types.push({ id: "new", ...req.postDataJSON() });
      if (path === "/entity-types/type" && method === "PUT") Object.assign(types[0]!, req.postDataJSON());
      return route.fulfill({ json: { id: "new", ...req.postDataJSON() } });
    }
    if (failReads && ["/groups/members", "/groups/invitations", "/notifiers", "/entity-types"].includes(path))
      return route.fulfill({ status: 500, json: { message: "Unavailable" } });
    if (path === "/status")
      return route.fulfill({ json: { build: { version: "v1.0.0" }, latest: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({ json: { item: { id: "user", name: "Member", defaultGroupId: "first", group } } });
    if (path === "/users/self/settings")
      return route.fulfill({ json: { item: { theme, displayLegacyHeader: false } } });
    if (path === "/groups/all") return route.fulfill({ json: [group] });
    if (path === "/groups/members") return route.fulfill({ json: members });
    if (path === "/groups/invitations") return route.fulfill({ json: invites });
    if (path === "/notifiers") return route.fulfill({ json: notifiers });
    if (path === "/entity-types") return route.fulfill({ json: types });
    if (path === "/templates") return route.fulfill({ json: [{ id: "template", name: longName }] });
    return route.fulfill({ json: [] });
  });
  return { writes, fail: (path: string) => (failPath = path), failReads: () => (failReads = true) };
}

async function noOverflow(page: Page) {
  expect(await page.locator("main").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
}

async function cancelDelete(page: Page) {
  const confirm = page.getByRole("alertdialog");
  await expect(confirm).toBeVisible();
  await confirm.getByRole("button", { name: "Cancel", exact: true }).press("Enter");
}

for (const theme of ["light", "cinematic"]) {
  for (const width of [390, 1440]) {
    test(`admin lists ${theme} ${width}: long names, tokens and confirmed removal`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 1000 });
      const api = await fixture(page, baseURL!, theme);
      await page.goto("/collection/members");
      await expect(page.getByRole("cell", { name: longName, exact: true })).toBeVisible();
      await noOverflow(page);
      const remove = page
        .getByRole("row")
        .filter({ hasText: "Current member" })
        .getByRole("button", { name: "Delete", exact: true });
      await remove.press("Enter");
      await cancelDelete(page);
      expect(api.writes).toHaveLength(0);
      await remove.press("Enter");
      await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
      await expect(page.getByText("Current member", { exact: true })).toBeHidden();
      expect(api.writes[0]?.method).toBe("DELETE");
      await page.goto("/collection/invites");
      await expect(page.getByRole("button", { name: "Copy Invite Link", exact: true })).toHaveCount(1);
      await expect(page.getByText(token, { exact: true })).toBeVisible();
      await noOverflow(page);
      await page
        .getByRole("row")
        .filter({ hasText: token })
        .getByRole("button", { name: "Delete", exact: true })
        .click();
      await cancelDelete(page);
      expect(api.writes).toHaveLength(1);
      await page.goto("/collection/notifiers");
      await expect(page.getByRole("article")).toContainText(longName);
      await expect(page.getByRole("article").getByRole("button", { name: "Edit", exact: true })).toBeVisible();
      await noOverflow(page);
      await page.goto("/collection/entity-types");
      await expect(page.getByText(longName, { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
      await noOverflow(page);
    });
  }

  test(`notifier dialog ${theme}: edit, test, failed create and successful retry`, async ({ page, baseURL }) => {
    const api = await fixture(page, baseURL!, theme);
    await page.goto("/collection/notifiers");
    await page.getByRole("button", { name: "Edit", exact: true }).press("Enter");
    let dialog = page.getByRole("dialog");
    await expect(dialog).toHaveAccessibleName("Edit Notifier");
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).toHaveValue(longName);
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("Edited notifier");
    await dialog.getByRole("button", { name: "Test", exact: true }).click();
    await expect.poll(() => api.writes.length).toBe(1);
    expect(api.writes[0]?.body).toEqual({ url: "https://example.com/notify" });
    await dialog.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(api.writes[1]?.body).toEqual({
      name: "Edited notifier",
      url: "https://example.com/notify",
      isActive: false,
    });
    await page.getByRole("main").getByRole("button", { name: "Create", exact: true }).click();
    dialog = page.getByRole("dialog");
    await expect(dialog).toHaveAccessibleName("Create Notifier");
    await expect(dialog.getByRole("button", { name: "Test", exact: true })).toBeDisabled();
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("New notifier");
    await dialog.getByRole("textbox", { name: "URL", exact: true }).fill("https://example.com/new");
    api.fail("/notifiers");
    await dialog.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(dialog.getByRole("alert")).toBeVisible();
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("New notifier");
    api.fail("");
    await dialog.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole("article").filter({ hasText: "New notifier" })).toBeVisible();
  });

  test(`entity type dialog ${theme}: default template payload and conversion cancellation`, async ({
    page,
    baseURL,
  }) => {
    const api = await fixture(page, baseURL!, theme);
    await page.goto("/collection/entity-types");
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("combobox")).toContainText(longName);
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("Edited type");
    await dialog.getByRole("button", { name: "Update", exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(api.writes[0]?.body).toMatchObject({
      id: "type",
      name: "Edited type",
      isLocation: false,
      defaultTemplateId: "template",
    });
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    await dialog.getByRole("checkbox").check();
    await expect(dialog.getByRole("combobox")).toHaveCount(0);
    await dialog.getByRole("button", { name: "Update", exact: true }).click();
    await cancelDelete(page);
    expect(api.writes).toHaveLength(1);
    await expect(dialog).toBeVisible();
    await page.keyboard.press("Escape");
  });
}

test("list failures are alerts, not empty states", async ({ page, baseURL }) => {
  const api = await fixture(page, baseURL!, "light");
  api.failReads();
  for (const path of ["members", "invites", "notifiers", "entity-types"]) {
    await page.goto(`/collection/${path}`);
    await expect(page.getByRole("alert").filter({ hasText: "Backend API call failed" }).first()).toBeVisible();
  }
});

for (const theme of ["light", "cinematic"]) {
  test(`admin action failures and mobile dialogs ${theme}`, async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 390, height: 1000 });
    await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
    const api = await fixture(page, baseURL!, theme);
    await page.goto("/collection/invites");
    await page.getByRole("button", { name: "Copy Invite Link", exact: true }).click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(`${baseURL}?token=${token}`);
    const invite = page.getByRole("row").filter({ hasText: token });
    await invite.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(invite).toBeHidden();
    await expect(page.getByRole("button", { name: "Copy Invite Link", exact: true })).toHaveCount(0);

    await page.goto("/collection/notifiers");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await cancelDelete(page);
    const beforeDelete = api.writes.length;
    api.fail("/notifiers/notifier");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByText("Failed to delete notifier.", { exact: true })).toBeVisible();
    await expect(page.getByRole("article")).toHaveCount(1);
    expect(api.writes.length).toBe(beforeDelete + 1);
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    let dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("Retained name");
    await dialog.getByRole("button", { name: "Submit", exact: true }).press("Enter");
    await expect(dialog.getByRole("alert")).toBeVisible();
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("Retained name");
    api.fail("/notifiers/test");
    await dialog.getByRole("button", { name: "Test", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("Failed to test notifier");
    expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    api.fail("");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByRole("article")).toHaveCount(0);
    await expect(page.getByText("No notifiers configured", { exact: true })).toBeVisible();

    await page.goto("/collection/entity-types");
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("combobox")).toContainText(longName);
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).toBeFocused();
    await page.screenshot({ path: test.info().outputPath("entity-type-mobile.png"), fullPage: true });
    expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await cancelDelete(page);
    api.fail("/entity-types/type");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(
      page.getByText("Failed to delete entity type. Make sure no entities are using it.", { exact: true })
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
    api.fail("");
    await page.getByRole("main").getByRole("button", { name: "Create", exact: true }).click();
    dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("New container");
    await dialog.getByRole("checkbox").check();
    await expect(dialog.getByRole("combobox")).toHaveCount(0);
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(api.writes.at(-1)?.body).toEqual({ name: "New container", icon: "", isLocation: true });
    await noOverflow(page);
  });
}
