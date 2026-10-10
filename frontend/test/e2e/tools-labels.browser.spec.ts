import { expect, test } from "@playwright/test";

import { fixture } from "./remaining-surfaces-fixture";

// API/camera fixtures must not be bypassed by the production PWA service worker.
test.use({ serviceWorkers: "block" });

for (const width of [390, 834, 1440]) {
  for (const theme of ["light", "cinematic"]) {
    test(`${theme}/${width}: label configuration and print isolation`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 900 });
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
      // Engines round physical units differently at subpixel precision.
      expect({ ...printed, cardWidth: screen.cardWidth, qrWidth: screen.qrWidth }).toEqual(screen);
      expect(printed.qrWidth).toBeCloseTo(screen.qrWidth, 2);
      expect(printed.cardWidth).toBeCloseTo(2.63 * 96, 0);
      expect(screen.cardHeight).toBe(96);
      expect(screen.qrWidth).toBeCloseTo(0.9 * 96, 0);
      expect(screen.width).toBeCloseTo(8.5 * 96, 0);
      await expect(page.locator(".hb-label-output section .flex.border-2").first().locator("img")).toHaveCount(0);
      await expect(page.locator(".hb-label-output img").first()).toHaveAttribute("src", /qrcode.*000-001/);
      await page.screenshot({
        path: `test-results/labels-print-${theme}.png`,
        fullPage: true,
      });
      await page.emulateMedia({ media: "screen" });
      await page.screenshot({
        path: `test-results/labels-screen-${theme}.png`,
        fullPage: true,
      });
    });

    test(`${theme}/${width}: tools expose separate existing controls`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width, height: 900 });
      await fixture(page, baseURL!, theme);
      await page.goto("/collection/tools");
      const restore = page.getByRole("button", {
        name: "Restore",
        exact: false,
      });
      await expect(restore).toHaveCount(1);
      expect(await restore.evaluate(el => el.parentElement!.closest("button"))).toBeNull();
      await restore.focus();
      await expect(restore).toBeFocused();
      expect(await restore.evaluate(el => getComputedStyle(el).boxShadow)).not.toBe("none");
      await expect(page.getByRole("link", { name: "Label Generator", exact: true })).toBeVisible();
    });
  }
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
