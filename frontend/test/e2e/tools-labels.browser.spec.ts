import { expect, test, type Page } from "@playwright/test";

// Presentation fixture only: no live auth, exports, or QR generation is exercised.
async function fixture(page: Page, baseURL: string, theme: string) {
  await page.context().addCookies([{ name: "hb.auth.session", value: "true", url: baseURL }]);
  await page.addInitScript(theme => {
    localStorage.setItem("homebox/preferences/location", JSON.stringify({ theme, language: "en" }));
  }, theme);
  await page.route("**/api/v1/**", route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    if (path === "/qrcode")
      return route.fulfill({
        contentType: "image/svg+xml",
        body: "<svg xmlns='http://www.w3.org/2000/svg' width='100' height='100'><rect width='100' height='100' fill='white'/><rect x='10' y='10' width='30' height='30' fill='black'/></svg>",
      });
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

for (const theme of ["light", "cinematic"]) {
  test(`${theme}: label configuration and print isolation`, async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await fixture(page, baseURL!, theme);
    await page.goto("/reports/label-generator");
    await expect(page.locator(".hb-label-output section").first()).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    await page.locator("#input-assetRangeMax").fill("4");
    await page.locator("#input-skipLabels").fill("1");
    await page.getByRole("button", { name: "Generate", exact: false }).click();
    expect(await page.locator(".hb-label-settings").evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    const geometry = () =>
      page
        .locator(".hb-label-output section")
        .first()
        .evaluate(el => {
          const card = el.querySelector(".flex.border-2")!;
          const qr = el.querySelector("img")!;
          const style = getComputedStyle(el);
          return {
            width: el.getBoundingClientRect().width,
            cardWidth: card.getBoundingClientRect().width,
            cardHeight: card.getBoundingClientRect().height,
            qrWidth: qr.getBoundingClientRect().width,
            bg: style.backgroundColor,
            fg: style.color,
            text: el.textContent,
          };
        });
    const screen = await geometry();
    expect(screen.bg).toBe("rgb(255, 255, 255)");
    expect(screen.fg).toBe("rgb(0, 0, 0)");
    await page.emulateMedia({ media: "print" });
    await expect(page.locator(".hb-label-settings")).toBeHidden();
    const printed = await geometry();
    // Existing screen-only sheet borders reduce available flex-row width.
    // Compare print dimensions to physical settings, not that bordered preview.
    expect({ ...printed, cardWidth: screen.cardWidth }).toEqual(screen);
    expect(printed.cardWidth).toBeCloseTo(2.63 * 96, 0);
    expect(screen.cardHeight).toBe(96);
    expect(screen.qrWidth).toBeCloseTo(0.9 * 96, 0);
    expect(screen.width).toBeCloseTo(8.5 * 96, 0);
    await expect(page.locator(".hb-label-output section .flex.border-2").first().locator("img")).toHaveCount(0);
    await expect(page.locator(".hb-label-output img").first()).toHaveAttribute("src", /qrcode.*000-001/);
    await page.screenshot({ path: `test-results/labels-print-${theme}.png`, fullPage: true });
    await page.emulateMedia({ media: "screen" });
    await page.screenshot({ path: `test-results/labels-screen-${theme}.png`, fullPage: true });
  });

  test(`${theme}: tools expose separate existing controls`, async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await fixture(page, baseURL!, theme);
    await page.goto("/collection/tools");
    const restore = page.getByRole("button", { name: "Restore", exact: false });
    await expect(restore).toHaveCount(1);
    expect(await restore.evaluate(el => el.parentElement!.closest("button"))).toBeNull();
    await restore.focus();
    await expect(restore).toBeFocused();
    expect(await restore.evaluate(el => getComputedStyle(el).boxShadow)).not.toBe("none");
    await expect(page.getByRole("link", { name: "Label Generator", exact: true })).toBeVisible();
  });
}

test("metric label settings retain physical dimensions", async ({ page, baseURL }) => {
  await fixture(page, baseURL!, "cinematic");
  await page.goto("/reports/label-generator");
  await expect(page.locator(".hb-label-output section").first()).toBeVisible();
  for (const [key, value] of Object.entries({
    assetRangeMax: 4,
    measure: "mm",
    cardHeight: 25.4,
    cardWidth: 66.802,
    pageWidth: 215.9,
    pageHeight: 279.4,
    pageTopPadding: 13.208,
    pageBottomPadding: 10.668,
    pageLeftPadding: 6.35,
    pageRightPadding: 2.54,
  }))
    await page.locator(`#input-${key}`).fill(String(value));
  await page.getByRole("button", { name: "Generate Page", exact: true }).click();
  await page.emulateMedia({ media: "print" });
  const size = await page.locator(".hb-label-output section .flex.border-2").first().boundingBox();
  expect(size!.width).toBeCloseTo(2.63 * 96, 0);
  expect(size!.height).toBeCloseTo(96, 0);
});
