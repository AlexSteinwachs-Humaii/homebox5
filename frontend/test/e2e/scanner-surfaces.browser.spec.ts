import { expect, test, type Page } from "@playwright/test";

// API/camera fixtures must not be bypassed by the production PWA service worker.
test.use({ serviceWorkers: "block" });

async function fixture(
  page: Page,
  baseURL: string,
  theme: string,
  unsupported = false,
  allowPreflight = false,
  failed = false
) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  await page.addInitScript(
    ({ theme, unsupported, allowPreflight, failed }) => {
      let calls = 0;
      localStorage.setItem("homebox/preferences/location", JSON.stringify({ theme, language: "en" }));
      if (unsupported) Object.defineProperty(navigator, "mediaDevices", { value: undefined });
      else
        Object.defineProperty(navigator, "mediaDevices", {
          // WebKit can return a new native MediaDevices wrapper on each access.
          // Replace the device surface, not just one wrapper's method.
          value: {
            enumerateDevices: async () => [],
            addEventListener: () => {},
            removeEventListener: () => {},
            getUserMedia: async () => {
              // The header requests permission before opening the modal. Allow only that preflight.
              if (allowPreflight && calls++ === 0) return { getTracks: () => [] };
              throw new DOMException("Camera failure fixture", failed ? "NotReadableError" : "NotAllowedError");
            },
          },
        });
    },
    { theme, unsupported, allowPreflight, failed }
  );
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
      return route.fulfill({
        json: { item: { theme, displayLegacyHeader: false } },
      });
    if (path === "/groups/all") return route.fulfill({ json: [{ id: "group", name: "Collection" }] });
    if (path === "/groups")
      return route.fulfill({
        json: { id: "group", name: "Collection", currency: "USD" },
      });
    if (path === "/entities") return route.fulfill({ json: { items: [], total: 0 } });
    if (path === "/entity-types")
      return route.fulfill({
        json: [
          { id: "item-type", name: "Item", isLocation: false },
          { id: "location-type", name: "Location", isLocation: true },
        ],
      });
    return route.fulfill({ json: [] });
  });
}

for (const width of [390, 834, 1440]) {
  for (const theme of ["light", "cinematic"]) {
    for (const state of ["denied", "unsupported", "failed"]) {
      const unsupported = state === "unsupported";
      test(`${theme}/${width}: AR ${state} feedback and focus at narrow width`, async ({ page, baseURL }) => {
        await page.setViewportSize({ width, height: 900 });
        await fixture(page, baseURL!, theme, unsupported, false, state === "failed");
        await page.goto("/scanner-ar");
        await expect(page.getByRole("alert")).toContainText(
          unsupported
            ? "not supported"
            : state === "failed"
              ? "An error occurred while scanning"
              : "Camera permission denied"
        );
        const retry = page.getByRole("button", { name: "Retry", exact: true });
        await retry.focus();
        await expect(retry).toBeFocused();
        expect(await retry.evaluate(el => getComputedStyle(el).boxShadow)).not.toBe("none");
        await retry.press("Enter");
        await expect(page.getByRole("alert")).toBeVisible();
        await expect(page.getByRole("button", { name: "Back", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "Switch Camera", exact: true })).toBeVisible();
        expect(await page.locator(".hb-scanner-surface").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
        const contrast = await page.getByRole("alert").evaluate(el => {
          const luminance = (color: string) => {
            const channels = color
              .match(/[\d.]+/g)!
              .slice(0, 3)
              .map(Number)
              .map(c => {
                const s = c / 255;
                return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
              });
            return channels[0]! * 0.2126 + channels[1]! * 0.7152 + channels[2]! * 0.0722;
          };
          const fg = luminance(getComputedStyle(el).color);
          const bg = luminance(getComputedStyle(el.closest(".hb-scanner-surface")!).backgroundColor);
          return (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05);
        });
        expect(contrast).toBeGreaterThanOrEqual(4.5);
        await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
        await page.screenshot({
          path: `test-results/scanner-${theme}-${unsupported ? "unsupported" : "denied"}.png`,
        });
      });
    }

    test(`${theme}/${width}: manual barcode entry, feedback and keyboard result selection at narrow width`, async ({
      page,
      baseURL,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      await fixture(page, baseURL!, theme);
      let searches = 0;
      await page.route("**/api/v1/products/search-from-barcode*", route => {
        searches++;
        return route.fulfill({
          json: [
            {
              barcode: "1234567890123",
              imageBase64: "data:image/gif;base64,R0lGODlhAQABAAAAACw=",
              item: { name: "Fixture camera", quantity: 1 },
              manufacturer: "Fixture",
              modelNumber: "Camera",
              search_engine_name: "example.com",
            },
          ],
        });
      });
      await page.goto("/home");
      await page.getByRole("button", { name: "Toggle sidebar", exact: true }).click();
      await page.getByRole("button", { name: "Create", exact: true }).click();
      await page.getByRole("menuitem", { name: "Item / Asset", exact: true }).click();
      await page
        .getByRole("button", {
          name: "Autofill with a manually provided barcode",
          exact: true,
        })
        .click();
      const dialog = page.getByRole("dialog", {
        name: "Import product",
        exact: true,
      });
      await dialog.getByLabel("Product's barcode", { exact: true }).fill("invalid");
      await dialog.getByRole("button", { name: "Search product", exact: true }).click();
      await expect(dialog.getByRole("alert")).toContainText("Invalid barcode provided");
      expect(searches).toBe(0);
      await dialog.getByLabel("Product's barcode", { exact: true }).fill("1234567890123");
      await dialog.getByRole("button", { name: "Search product", exact: true }).click();
      const select = dialog.getByRole("button", {
        name: "Select product: Fixture camera",
        exact: true,
      });
      await page.keyboard.press("Tab");
      await select.focus();
      await expect(select).toBeFocused();
      expect(await select.evaluate(el => getComputedStyle(el).boxShadow)).not.toBe("none");
      await select.press("Enter");
      await expect(select).toHaveAttribute("aria-pressed", "true");
      await expect(dialog.getByRole("button", { name: "Import selected", exact: true })).toBeEnabled();
      expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
      await dialog.getByRole("button", { name: "Close", exact: true }).click();
      await expect(dialog).toHaveCount(0);
    });

    test(`${theme}/${width}: modal permission feedback retains camera picker and AR entry`, async ({
      page,
      baseURL,
    }) => {
      await fixture(page, baseURL!, theme, false, true);
      await page.goto("/home");
      await page.getByRole("button", { name: "Scanner", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog.getByRole("alert")).toContainText("Camera permission denied");
      await expect(dialog.getByRole("combobox", { name: "Pick a video source" })).toBeVisible();
      await expect(dialog.getByRole("button", { name: "Close", exact: true })).toBeVisible();
      await dialog.getByRole("button", { name: "AR Mode", exact: true }).click();
      await expect(page).toHaveURL(/scanner-ar/);
      await expect(page.getByRole("alert")).toContainText("Camera permission denied");
    });
  }
}
