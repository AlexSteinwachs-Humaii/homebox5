import { expect, test } from "@playwright/test";
import { contrast } from "./remaining-surfaces-contrast";

// API/camera fixtures must not be bypassed by the production PWA service worker.
test.use({ serviceWorkers: "block" });

for (const width of [390, 834, 1440]) {
  for (const theme of ["light", "cinematic"]) {
    test(`${theme}/${width}: standalone entry and recovery preserve preferences at narrow widths`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript(theme => {
        localStorage.setItem(
          "homebox/preferences/location",
          JSON.stringify({ theme, language: "de", itemsPerTablePage: 24 })
        );
      }, theme);
      const requests: { path: string; body: unknown }[] = [];
      let fail = true;
      await page.route("**/api/v1/**", route => {
        const path = new URL(route.request().url()).pathname;
        if (path.endsWith("/status"))
          return route.fulfill({
            json: {
              allowRegistration: true,
              demo: false,
              build: { version: "v1.0", commit: "fixture" },
              oidc: {
                enabled: true,
                allowLocal: true,
                autoRedirect: false,
                buttonText: "Long identity provider sign-in text ".repeat(4),
              },
            },
          });
        if (route.request().method() === "POST") {
          requests.push({ path, body: route.request().postDataJSON() });
          return route.fulfill({
            status: fail ? 400 : 204,
            json: fail ? { message: "Invalid" } : undefined,
          });
        }
        return route.fulfill({ json: [] });
      });
      const fit = async () => {
        await expect(page.locator("[data-sidebar=sidebar]")).toHaveCount(0);
        expect(
          await page.locator("form, .hb-panel").evaluateAll(els => els.every(el => el.scrollWidth <= el.clientWidth))
        ).toBe(true);
        expect(await page.evaluate(() => JSON.parse(localStorage.getItem("homebox/preferences/location")!).theme)).toBe(
          theme
        );
        expect(
          await page.evaluate(() => JSON.parse(localStorage.getItem("homebox/preferences/location")!).itemsPerTablePage)
        ).toBe(24);
      };
      await page.goto("/");
      await expect(page.locator("#login-form")).toBeVisible();
      await fit();
      await contrast(page.locator("label[for=login-username]"));
      await contrast(page.locator("#login-form button[type=submit]"));
      await page.locator("#login-username").fill("fixture@example.com");
      await page.locator("#login-password").fill("WrongPassword123");
      const toggle = page.locator("#login-form button[aria-pressed]");
      await toggle.focus();
      await expect(toggle).toBeFocused();
      expect(await toggle.evaluate(el => getComputedStyle(el).boxShadow)).not.toBe("none");
      await toggle.press("Enter");
      await expect(page.locator("#login-password")).toHaveAttribute("type", "text");
      await page.locator("#login-form button[type=submit]").click();
      await expect(page.locator(".login-error").first()).toBeVisible();
      await page.getByTestId("register-button").click();
      await expect(page.locator("#register-form")).toBeVisible();
      await fit();
      await page.goto("/forgot-password");
      await contrast(page.locator("label[for=forgot-email]"));
      await page.locator("#forgot-email").fill("fixture@example.com");
      await page.locator("button[type=submit]").click();
      await expect(page.locator("[data-sonner-toast]").first()).toBeVisible();
      fail = false;
      await page.locator("button[type=submit]").click();
      await expect(page.locator("#forgot-password-form")).toHaveCount(0);
      await fit();
      await page.goto("/reset-password");
      await expect(page.locator("button[type=submit]")).toBeDisabled();
      await fit();
      await page.goto("/reset-password?token=fixture-token");
      await page.locator("#reset-password-new").fill("ThisIsAStrongDemoPass123!");
      await expect(page.locator("button[type=submit]")).toBeEnabled();
      fail = true;
      await page.locator("button[type=submit]").click();
      await expect(page.locator("[data-sonner-toast]").first()).toBeVisible();
      await expect(page).toHaveURL(/reset-password/);
      fail = false;
      await page.locator("button[type=submit]").click();
      await expect(page).toHaveURL("/");
      expect(requests).toContainEqual({
        path: "/api/v1/users/reset-password",
        body: { token: "fixture-token", password: "ThisIsAStrongDemoPass123!" },
      });
    });
  }
}

test("disabled registration and OIDC-only availability remain server controlled", async ({ page }) => {
  await page.route("**/api/v1/status", route =>
    route.fulfill({
      json: {
        allowRegistration: false,
        build: { version: "v1", commit: "fixture" },
        oidc: {
          enabled: true,
          allowLocal: false,
          autoRedirect: false,
          buttonText: "Identity sign-in",
        },
      },
    })
  );
  await page.goto("/");
  await expect(page.locator("#login-username")).toHaveCount(0);
  await expect(page.getByTestId("register-button")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Identity sign-in" })).toBeVisible();
});

test("no-collection entry retains create/join dialogs and keyboard access", async ({ page, baseURL }) => {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL! }]);
  await page.route("**/api/v1/**", route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    if (path === "/status")
      return route.fulfill({
        json: {
          build: { version: "v1", commit: "fixture" },
          latest: { version: "v1" },
        },
      });
    if (path === "/users/self")
      return route.fulfill({
        json: {
          item: {
            id: "fixture",
            name: "Fixture",
            email: "fixture@example.com",
          },
        },
      });
    if (path === "/users/self/settings")
      return route.fulfill({
        json: { item: { theme: "cinematic", language: "en" } },
      });
    return route.fulfill({ json: [] });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/no-collections");
  const create = page.getByRole("button", {
    name: "Create a new collection",
    exact: true,
  });
  await expect(create).toBeVisible();
  await create.focus();
  await create.press("Enter");
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("dialog").locator("input").first()).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const join = page.getByRole("button", {
    name: "Join an existing collection",
    exact: true,
  });
  await join.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  expect(await page.getByRole("dialog").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
  await page.keyboard.press("Escape");
  await expect(page).toHaveURL(/\/no-collections\/?$/);
});
