import { expect, test, type Page } from "@playwright/test";
import { themes } from "../../lib/data/themes";

const html = (page: Page) => page.locator("html");
async function login(page: Page, email = "demo@example.com", password = "demodemo") {
  await page.goto("/");
  await page.waitForLoadState("networkidle");
  await page.fill("input[type='text']", email);
  await page.fill("input[type='password']", password);
  await page.click("button[type='submit']");
  await expect(page).toHaveURL("/home");
  if ((page.viewportSize()?.width ?? 1440) >= 768) {
    await expect(page.getByRole("combobox", { name: "Select Collection", exact: true })).not.toContainText(
      "Select Collection"
    );
  }
}

// Existing-account snapshots only: isolate preferences from the shared demo user.
// Registration, persistence and collection switching below use a real disposable account.
async function settingsFixture(page: Page, theme?: string) {
  await page.route("**/api/v1/users/self/settings", async route => {
    if (route.request().method() !== "GET") {
      await route.fulfill({ json: { item: route.request().postDataJSON() } });
      return;
    }
    await route.fulfill({ json: { item: { displayLegacyHeader: false, ...(theme ? { theme } : {}) } } });
  });
}

for (const saved of [undefined, "homebox", "light", "cinematic"]) {
  test(`existing account on clean storage keeps ${saved ?? "implicit HomeBox"}`, async ({ page }) => {
    await settingsFixture(page, saved);
    await login(page);
    await expect(html(page)).toHaveAttribute("data-theme", saved ?? "homebox");
    await page.reload();
    await expect(html(page)).toHaveAttribute("data-theme", saved ?? "homebox");
  });
}

test("new registration, keyboard theme selection, server persistence and real collection switching", async ({
  page,
}) => {
  const email = `foundation-${Date.now()}-${test.info().workerIndex}@example.com`;
  await page.goto("/");
  await page.getByTestId("register-button").click();
  await page.getByTestId("email-input").locator("input").fill(email);
  await page.getByTestId("name-input").locator("input").fill("Foundation regression");
  await page.getByTestId("password-input").locator("input").fill("FoundationSecret42!");
  await page.getByTestId("confirm-register-button").click();
  await expect(page).toHaveURL("/");
  await login(page, email, "FoundationSecret42!");
  await expect(html(page)).toHaveAttribute("data-theme", "cinematic");
  await page.goto("/profile");
  for (const theme of themes) {
    await expect(page.locator(`[data-set-theme='${theme.value}']`)).toHaveAccessibleName(theme.label);
  }
  const light = page.locator("[data-set-theme='homebox']");
  await light.focus();
  await expect(light).toBeFocused();
  await expect.poll(() => light.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
  const saved = page.waitForResponse(
    response =>
      response.url().endsWith("/users/self/settings") && response.request().method() === "PUT" && response.ok()
  );
  await light.press("Enter");
  await expect(light).toHaveAttribute("aria-pressed", "true");
  await saved;
  // Preserve authentication cookies, but remove device-local preferences entirely.
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await expect(html(page)).toHaveAttribute("data-theme", "homebox");

  await page.goto("/home");
  const selector = page.getByRole("combobox", {
    name: "Select Collection",
    exact: true,
  });
  await expect(selector).not.toContainText("Select Collection");
  const original = await selector.innerText();
  await selector.click();
  await page.getByText("Create New Collection", { exact: true }).click();
  const dialog = page.getByRole("dialog");
  const collection = "Zweite gemeinsame Sammlung mit einem sehr langen übersetzten Namen";
  await dialog.getByRole("textbox").fill(collection);
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  await expect(selector).toContainText(collection);
  await selector.click();
  await page.getByRole("option", { name: original.trim(), exact: true }).click();
  await expect(selector).toContainText(original.trim());
  await page.reload();
  await expect(selector).toContainText(original.trim());
});

for (const theme of ["cinematic", "homebox"]) {
  for (const width of [1440, 834, 390]) {
    test(`${theme} ${width}px shell and dialogs tolerate long translated labels`, async ({ page }) => {
      await page.setViewportSize({ width, height: 1100 });
      await settingsFixture(page, theme);
      await login(page);
      await expect(html(page)).toHaveAttribute("data-theme", theme);
      const toggle = page.getByRole("button", {
        name: "Toggle sidebar",
        exact: true,
      });
      if (width < 768) await toggle.click();
      await expect(page.getByRole("combobox", { name: "Select Collection", exact: true })).not.toContainText(
        "Select Collection"
      );
      await expect
        .poll(
          async () =>
            (await page.locator("[data-sidebar='sidebar']").filter({ visible: true }).first().boundingBox())?.x
        )
        .toBeGreaterThanOrEqual(0);
      // Translation stress without changing the global locale or shared account data.
      await page.locator("[data-sidebar='menu-button'] > span:last-child").evaluateAll(labels => {
        for (const label of labels) label.textContent = "Gemeinsame Inventarverwaltung und Sammlungseinstellungen";
      });
      const sidebar = page.locator("[data-sidebar='sidebar']").filter({ visible: true }).first();
      await expect.poll(() => sidebar.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      await page.screenshot({ path: test.info().outputPath("shell.png"), fullPage: true });
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      // Quick menu remains reachable without relying on the stressed labels.
      if (width < 768) await page.keyboard.press("Escape");
      await toggle.focus();
      await page.keyboard.press("Control+Backquote");
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog).toHaveAccessibleName(/number keys/i);
      await dialog
        .getByRole("option")
        .first()
        .evaluate(el => {
          el.textContent = "Einen neuen Inventargegenstand mit einem sehr langen übersetzten Namen erstellen";
        });
      const box = await dialog.boundingBox();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(width);
      await expect.poll(() => dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      for (let tab = 0; tab < 5; tab++) {
        await page.keyboard.press("Tab");
        await expect.poll(() => dialog.evaluate(el => el.contains(document.activeElement))).toBe(true);
      }
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(toggle).toBeFocused();
    });
  }
}

// Resolve actual CSS custom properties through the browser (not copied RGB constants).
for (const theme of ["cinematic", "homebox"]) {
  test(`${theme} shared text, hover states, input boundaries and focus contrast meet AA`, async ({ page }) => {
    await settingsFixture(page, theme);
    await login(page);
    await expect(html(page)).toHaveAttribute("data-theme", theme);
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
      const result: { label: string; ratio: number; threshold: number }[] = [];
      const check = (foreground: string, background: string, threshold: number, opacity = 1) => {
        const bg = rgb(background);
        const fg = rgb(foreground).map((v, i) => v * opacity + bg[i]! * (1 - opacity));
        const [lo, hi] = [luminance(fg), luminance(bg)].sort((a, b) => a - b);
        result.push({
          label: `${foreground}/${background} @${opacity}`,
          ratio: (hi! + 0.05) / (lo! + 0.05),
          threshold,
        });
      };
      for (const surface of ["background", "card", "popover", "muted", "background-accent"]) {
        check("foreground", surface, 4.5);
        check("muted-foreground", surface, 4.5);
      }
      for (const role of ["primary", "secondary", "accent", "destructive", "sidebar-primary", "sidebar-accent"])
        check(`${role}-foreground`, role, 4.5);
      check("sidebar-foreground", "sidebar-background", 4.5);
      check("primary", "background", 4.5);
      for (const surface of ["background", "card", "popover", "muted", "sidebar-background"]) {
        check("ring", surface, 3);
        check("input", surface, 3);
      }
      check("sidebar-ring", "sidebar-background", 3);
      // Button hover uses a 90% primary fill composited over background.
      const bg = rgb("background");
      const hover = rgb("primary").map((v, i) => v * 0.9 + bg[i]! * 0.1);
      const [lo, hi] = [luminance(rgb("primary-foreground")), luminance(hover)].sort((a, b) => a - b);
      result.push({
        label: "primary button hover",
        ratio: (hi! + 0.05) / (lo! + 0.05),
        threshold: 4.5,
      });
      probe.remove();
      return result;
    });
    await test.info().attach(`${theme}-contrast.json`, {
      body: JSON.stringify(ratios, null, 2),
      contentType: "application/json",
    });
    for (const { label, ratio, threshold } of ratios) expect(ratio, label).toBeGreaterThanOrEqual(threshold);
    const search = page.getByRole("searchbox", { name: "Search", exact: true });
    await search.focus();
    await expect(search).toBeFocused();
    await expect.poll(() => search.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
    await expect(page.getByRole("button", { name: "Scanner", exact: true })).toBeVisible();
  });
}

for (const path of [
  "/locations",
  "/tags",
  "/items",
  "/templates",
  "/maintenance",
  "/profile",
  "/collection/members",
  "/collection/invites",
  "/collection/notifiers",
  "/collection/settings",
  "/collection/entity-types",
  "/collection/tools",
  "/home",
]) {
  test(`shell destination ${path} retains its route and active state`, async ({ page }) => {
    await settingsFixture(page, "cinematic");
    await login(page);
    const link = page.locator(`[data-sidebar='menu-button'][href='${path}']`).first();
    await expect(link).toHaveAccessibleName(/\S/);
    await link.click();
    await expect(page).toHaveURL(path);
    await expect(page.locator(`[data-sidebar='menu-button'][href='${path}']`).first()).toHaveAttribute(
      "aria-current",
      "page"
    );
  });
}

test("Create actions and scanner remain keyboard accessible with focus return", async ({ page }) => {
  await settingsFixture(page, "cinematic");
  await login(page);
  for (const [name, title] of [
    ["Item / Asset", /Create\s*Item/],
    ["Location", /Create\s*Location/],
    ["Tag", /Create\s*Tag/],
  ] as const) {
    const trigger = page.getByRole("button", { name: "Create", exact: true });
    await trigger.focus();
    await trigger.press("Enter");
    await page.getByRole("menuitem", { name: new RegExp(name) }).click();
    const dialog = page.getByRole("dialog", { includeHidden: true });
    await expect(dialog.getByRole("heading")).toHaveText(title);
    if (name === "Location") {
      // An expanded nested selector, not the parent dialog, consumes Escape.
      const selector = dialog.getByRole("combobox", { includeHidden: true }).first();
      await selector.click();
      await expect(selector).toHaveAttribute("aria-expanded", "true");
      await page.keyboard.press("Escape");
      await expect(selector).toHaveAttribute("aria-expanded", "false");
      await expect(dialog).toBeVisible();
    }
    await page.keyboard.press("Tab");
    await expect.poll(() => dialog.evaluate(el => el.contains(document.activeElement))).toBe(true);
    await dialog.getByRole("textbox").first().focus();
    await expect(page.getByRole("tooltip")).toBeHidden();
    await page.keyboard.press("Escape");

    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
  }
  // Exercise permission denial, then a granted media request without hardware.
  await page.evaluate(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      throw new DOMException("Denied by test", "NotAllowedError");
    };
  });
  const scanner = page.getByRole("button", { name: "Scanner", exact: true });
  await scanner.click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(page.locator("[data-sonner-toast]").last()).toContainText(/permission/i);
  await page.evaluate(() => {
    navigator.mediaDevices.getUserMedia = async () => new MediaStream();
  });
  await scanner.focus();
  await scanner.press("Enter");
  const dialog = page.getByRole("dialog", { includeHidden: true });
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAccessibleName(/\S/);
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(scanner).toBeFocused();
});
