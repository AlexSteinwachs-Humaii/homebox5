import { expect, test, type Page, type Locator } from "@playwright/test";

const longName = "LongAccountName".repeat(25);
const fakeToken = "hbk_browser_fixture_only";

async function fixture(page: Page, baseURL: string, theme: string, onlyMember = false) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  const groups = [
    { id: "first", name: "First collection", currency: "USD" },
    { id: "second", name: "Second collection", currency: "EUR" },
  ];
  let settings: Record<string, unknown> = {
    theme,
    language: "en",
    overrideFormatLocale: "en-GB",
    displayLegacyHeader: false,
    legacyImageFit: true,
    quickActions: { enabled: false },
    duplicateSettings: {
      copyMaintenance: true,
      copyAttachments: false,
      copyCustomFields: true,
      copyPrefixOverride: "Saved prefix",
    },
  };
  let keys = [{ id: "key", name: longName, createdAt: "2026-01-01", expiresAt: null, lastUsedAt: null }];
  const writes: { path: string; method: string; body: unknown }[] = [];
  let failedPath = "";
  await page.route("**/api/v1/**", route => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace("/api/v1", "");
    const method = req.method();
    if (path === "/users/self/settings") {
      if (method !== "GET") settings = req.postDataJSON();
      return route.fulfill({ json: { item: settings } });
    }
    if (method !== "GET") {
      const body = req.postData() ? req.postDataJSON() : null;
      writes.push({ path, method, body });
      if (path === failedPath) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
      if (path === "/users/self/api-keys" && method === "POST") {
        keys.push({ id: "new", name: body.name, createdAt: "2026-01-01", expiresAt: body.expiresAt, lastUsedAt: null });
        return route.fulfill({ json: { ...keys.at(-1), token: fakeToken } });
      }
      if (path.startsWith("/users/self/api-keys/")) keys = keys.filter(k => !path.endsWith(k.id));
      return route.fulfill({ status: 204 });
    }
    if (path === failedPath) return route.fulfill({ status: 500, json: { message: "Unavailable" } });
    if (path === "/status")
      return route.fulfill({ json: { build: { version: "v1.0.0" }, latest: { version: "v1.0.0" } } });
    if (path === "/users/self")
      return route.fulfill({
        json: {
          item: { id: "user", name: longName, email: "account@example.com", defaultGroupId: "first", group: groups[0] },
        },
      });
    if (path === "/groups/all") return route.fulfill({ json: groups });
    if (path === "/groups/members")
      return route.fulfill({ json: onlyMember ? [{ id: "user" }] : [{ id: "user" }, { id: "other" }] });
    if (path === "/users/self/api-keys") return route.fulfill({ json: keys });
    if (path === "/groups") return route.fulfill({ json: groups[req.headers()["x-tenant"] === "second" ? 1 : 0] });
    return route.fulfill({ json: [] });
  });
  return { writes, fail: (path: string) => (failedPath = path), settings: () => settings };
}

async function noOverflow(locator: Locator) {
  expect(await locator.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
}

async function contrast(locator: Locator) {
  return locator.evaluate(el => {
    const rgb = (v: string) => (v.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number);
    const luminance = (c: number[]) =>
      c
        .map(x => x / 255)
        .map(x => (x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4))
        .reduce((sum, x, i) => sum + x * [0.2126, 0.7152, 0.0722][i]!, 0);
    let parent: Element | null = el;
    let bg = "";
    while (parent) {
      bg = getComputedStyle(parent).backgroundColor;
      if (bg !== "rgba(0, 0, 0, 0)" && bg !== "transparent") break;
      parent = parent.parentElement;
    }
    const a = luminance(rgb(getComputedStyle(el).color));
    const b = luminance(rgb(bg));
    return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
  });
}

for (const theme of ["light", "cinematic"]) {
  for (const width of [390, 1440]) {
    test(`profile ${theme} ${width}: labels, contrast and saved preferences`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 1000 });
      const api = await fixture(page, baseURL!, theme);
      await page.goto("/profile");
      await expect(page.getByRole("article")).toContainText(longName);
      await noOverflow(page.locator("main"));
      const headingContrast = await contrast(page.getByRole("heading", { name: "Profile", exact: true }));
      console.log(`${theme} profile heading contrast: ${headingContrast.toFixed(2)}:1`);
      expect(headingContrast).toBeGreaterThanOrEqual(4.5);
      expect(await contrast(page.getByRole("button", { name: "Change Password", exact: true }))).toBeGreaterThanOrEqual(
        4.5
      );
      await page.getByRole("button", { name: "Duplicate Settings", exact: true }).press("Enter");
      const dialog = page.getByRole("dialog");
      await expect(dialog.getByRole("switch", { name: "Copy Maintenance", exact: true })).toBeChecked();
      await expect(dialog.getByRole("textbox", { name: "Copy Prefix", exact: true })).toHaveValue("Saved prefix");
      await dialog.getByRole("textbox", { name: "Copy Prefix", exact: true }).fill("Changed prefix");
      await noOverflow(dialog);
      await page.keyboard.press("Escape");
      await page.getByRole("button", { name: "Enable Legacy Header", exact: true }).click();
      await page.getByRole("button", { name: "Disable Legacy Fit: Fit Image with Bars", exact: true }).click();
      await page.locator('[data-set-theme="cinematic"]').click();
      await expect
        .poll(() => api.settings())
        .toMatchObject({
          theme: "cinematic",
          displayLegacyHeader: true,
          legacyImageFit: false,
          quickActions: { enabled: false },
          duplicateSettings: { copyPrefixOverride: "Changed prefix" },
          overrideFormatLocale: "en-GB",
          language: "en",
        });
      await page.reload();
      await expect(page.locator('[data-set-theme="cinematic"]')).toHaveAttribute("aria-pressed", "true");
      await expect(page.getByRole("button", { name: "Disable Legacy Header", exact: true })).toBeVisible();
      await expect(page.getByRole("combobox", { name: "Language", exact: true })).toContainText("English");
      await expect(
        page.getByRole("combobox", { name: "Override Date and Currency Language", exact: true })
      ).toContainText("English");
      await page.getByRole("button", { name: "Duplicate Settings", exact: true }).click();
      await expect(dialog.getByRole("textbox", { name: "Copy Prefix", exact: true })).toHaveValue("Changed prefix");
      await page.keyboard.press("Escape");
      await noOverflow(page.locator("main"));
      await expect(dialog).toBeHidden();
      await page.screenshot({ path: test.info().outputPath("profile-preferences.png"), fullPage: true });
    });
  }

  test(`account controls ${theme}: validation, failed saves, one-time key and confirmation focus`, async ({
    page,
    baseURL,
  }) => {
    await page.setViewportSize({ width: 390, height: 1000 });
    await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
    const api = await fixture(page, baseURL!, theme);
    await page.goto("/profile");
    await page.getByRole("button", { name: "Change Password", exact: true }).press("Enter");
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByLabel("Current Password", { exact: true })).toBeFocused();
    await expect(dialog.getByRole("button", { name: "Submit", exact: true })).toBeDisabled();
    await dialog.getByLabel("Current Password", { exact: true }).fill("Old-password-123!");
    await dialog.getByLabel(/^New Password/).fill("Distinct-phrase-123!");
    const reveal = dialog.getByRole("button", { name: "Toggle Password Show" }).first();
    await reveal.press("Enter");
    await expect(reveal).toHaveAttribute("aria-pressed", "true");
    await expect(dialog.getByLabel("Current Password", { exact: true })).toHaveAttribute("type", "text");
    api.fail("/users/self/change-password");
    await dialog.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("Failed to change password");
    const errorContrast = await contrast(dialog.getByRole("alert"));
    console.log(`${theme} error contrast: ${errorContrast.toFixed(2)}:1`);
    expect(errorContrast).toBeGreaterThanOrEqual(4.5);
    await expect(dialog.getByLabel(/^New Password/)).toHaveValue("Distinct-phrase-123!");
    api.fail("");
    await dialog.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(api.writes.at(-1)?.body).toEqual({ current: "Old-password-123!", new: "Distinct-phrase-123!" });
    await page.getByRole("button", { name: "Create API Key", exact: true }).press("Enter");
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).toBeFocused();
    await expect(dialog.getByRole("button", { name: "Create", exact: true })).toBeDisabled();
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("Integration");
    await dialog.getByRole("checkbox", { name: "Set a custom expiration date" }).check();
    await dialog.getByLabel("Expires", { exact: true }).fill("2099-01-01");
    api.fail("/users/self/api-keys");
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("Failed to create API key");
    await expect(dialog.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("Integration");
    api.fail("");
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect(dialog.getByRole("textbox", { name: "Your new API key", exact: true })).toHaveValue(fakeToken);
    expect(api.writes.at(-1)?.body).toEqual({ name: "Integration", expiresAt: "2099-01-01T23:59:59.000Z" });
    await dialog.getByRole("button", { name: "Copy", exact: true }).click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(fakeToken);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    expect(
      await page
        .locator("input")
        .evaluateAll(inputs => inputs.some(el => (el as HTMLInputElement).value === "hbk_browser_fixture_only"))
    ).toBe(false);
    const actions = [
      page.getByRole("article").filter({ hasText: "Integration" }).getByRole("button", { name: "Delete", exact: true }),
      page.getByRole("button", { name: "Log Out of All Devices", exact: true }),
      page.getByRole("button", { name: "Delete Account", exact: true }),
    ];
    for (const action of actions) {
      const before = api.writes.length;
      await action.press("Enter");
      const confirmation = page.getByRole("alertdialog");
      await expect(confirmation.getByRole("button", { name: "Cancel", exact: true })).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(confirmation).toBeHidden();
      await expect(action).toBeFocused();
      expect(api.writes).toHaveLength(before);
    }
    api.fail("/users/self/api-keys/new");
    await actions[0]!.click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByText("Failed to revoke API key.", { exact: true })).toBeVisible();
    await expect(page.getByRole("article").filter({ hasText: "Integration" })).toHaveCount(1);
    api.fail("");
    await actions[0]!.click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByRole("article").filter({ hasText: "Integration" })).toHaveCount(0);
  });
}

for (const onlyMember of [false, true]) {
  test(`collection action visibility: ${onlyMember ? "sole member" : "shared member"}`, async ({ page, baseURL }) => {
    await fixture(page, baseURL!, "light", onlyMember);
    await page.goto("/collection/members");
    await expect(page.getByRole("main").getByRole("link", { name: "Members", exact: true })).toBeVisible();
    await expect(page.getByRole("main").getByRole("link", { name: "Settings", exact: true })).toBeVisible();
    await expect(
      page.getByRole("button", { name: onlyMember ? "Delete Collection" : "Leave Collection", exact: true })
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: onlyMember ? "Leave Collection" : "Delete Collection", exact: true })
    ).toHaveCount(0);
  });
}

for (const [action, path, failure, success] of [
  ["Log Out of All Devices", "/users/logout/all", "Failed to sign out of all devices.", "Signed out of all devices."],
  ["Delete Account", "/users/self", "Failed to delete your account.", "Your account has been deleted."],
]) {
  test(`${action}: failure preserves session, success clears it`, async ({ page, baseURL }) => {
    const api = await fixture(page, baseURL!, "light");
    await page.goto("/profile");
    await expect(page.getByRole("button", { name: action, exact: true })).toBeVisible();
    await expect(page.getByRole("article")).toContainText(longName);
    api.fail(path!);
    await page.getByRole("button", { name: action, exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByText(failure!, { exact: true })).toBeVisible();
    await expect(page).toHaveURL(/\/profile\/?$/);
    api.fail("");
    await page.getByRole("button", { name: action, exact: true }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(page.getByText(success!, { exact: true })).toBeVisible();
    await expect(page).not.toHaveURL(/\/profile\/?$/);
    expect((await page.context().cookies()).find(c => c.name === "hb.auth.session")?.value).not.toBe("true");
  });
}

test("API-key loading failure is not an empty list", async ({ page, baseURL }) => {
  const api = await fixture(page, baseURL!, "light");
  api.fail("/users/self/api-keys");
  await page.goto("/profile");
  await expect(page.getByRole("alert").filter({ hasText: "Backend API call failed" }).first()).toBeVisible();
  await expect(page.getByText("No API keys yet", { exact: true })).toBeHidden();
});

test("language, formatting and duplication toggles save after reload", async ({ page, baseURL }) => {
  const api = await fixture(page, baseURL!, "light");
  await page.goto("/profile");
  await expect(page.getByRole("article")).toContainText(longName);
  await page.getByRole("button", { name: "Duplicate Settings", exact: true }).click();
  await page.getByRole("switch", { name: "Copy Attachments", exact: true }).check();
  await page.keyboard.press("Escape");
  await page.locator("#overrideLocale").click();
  await page.getByRole("option", { name: "German - (Deutsch)", exact: true }).click();
  await page.locator("#language").click();
  await page.getByRole("option", { name: "German (Deutsch)", exact: true }).click();
  await expect
    .poll(() => api.settings())
    .toMatchObject({ language: "de", overrideFormatLocale: "de", duplicateSettings: { copyAttachments: true } });
  await page.reload();
  await expect(page.locator("#language")).toContainText("Deutsch");
  await expect(page.locator("#overrideLocale")).toContainText("Deutsch");
  expect(api.settings().quickActions).toEqual({ enabled: false });
});

for (const theme of ["light", "cinematic"]) {
  test(`admin representative text contrast ${theme}`, async ({ page, baseURL }) => {
    await fixture(page, baseURL!, theme);
    await page.goto("/collection/members");
    const heading = page.getByRole("heading", { name: "Members", exact: true });
    await expect(heading).toBeVisible();
    const activeTab = page
      .getByRole("navigation", { name: "Collection Options" })
      .getByRole("link", { name: "Members", exact: true });
    for (const target of [heading, activeTab]) {
      const ratio = await contrast(target);
      console.log(`${theme} admin text contrast: ${ratio.toFixed(2)}:1`);
      expect(ratio).toBeGreaterThanOrEqual(4.5);
    }
  });
}
