import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { dashboardFixture, itemId, longName } from "./dashboard-fixture";

test.setTimeout(60000);

async function contrastEvidence(page: Page) {
  return page.evaluate(() => {
    const rgb = (value: string) => (value.match(/[\d.]+/g) || []).map(Number);
    const blend = (fg: number[], bg: number[]) =>
      fg.slice(0, 3).map((v, i) => v * (fg[3] ?? 1) + bg[i]! * (1 - (fg[3] ?? 1)));
    const background = (el: Element | null): number[] => {
      if (!el) return [255, 255, 255];
      return blend(rgb(getComputedStyle(el).backgroundColor), background(el.parentElement));
    };
    const luminance = (color: number[]) =>
      color.slice(0, 3).reduce((sum, value, i) => {
        const v = value / 255;
        return sum + (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4) * [0.2126, 0.7152, 0.0722][i]!;
      }, 0);
    const ratio = (fg: number[], bg: number[]) => {
      const [lo, hi] = [luminance(fg), luminance(bg)].sort((a, b) => a - b);
      return (hi! + 0.05) / (lo! + 0.05);
    };
    const results: { label: string; ratio: number; threshold: number }[] = [];
    for (const el of document.querySelectorAll(
      "[data-dashboard] header *, [data-dashboard] section *, input[type=search]"
    )) {
      const css = getComputedStyle(el);
      if (!el.getClientRects().length || css.visibility === "hidden" || el.closest("svg")) continue;
      const text = Array.from(el.childNodes)
        .filter(n => n.nodeType === Node.TEXT_NODE)
        .map(n => n.textContent)
        .join("")
        .trim();
      if (text || el.matches("input")) {
        const bg = background(el);
        results.push({
          label: `${el.tagName}: ${text || "search text"}`,
          ratio: ratio(blend(rgb(css.color), bg), bg),
          threshold: 4.5,
        });
      }
    }
    // Resolve focus and input tokens through computed CSS and compare with the actual surfaces.
    const probe = document.createElement("span");
    document.body.append(probe);
    for (const token of ["ring", "input"]) {
      probe.style.color = `hsl(var(--${token}))`;
      for (const el of document.querySelectorAll("section a, input[type=search]")) {
        if (!el.getClientRects().length) continue;
        results.push({
          label: `${token}/${el.tagName}`,
          ratio: ratio(rgb(getComputedStyle(probe).color), background(el.parentElement)),
          threshold: 3,
        });
      }
    }
    probe.remove();
    return results;
  });
}

for (const theme of ["homebox", "light", "cinematic"]) {
  for (const width of [390, 834, 1440]) {
    for (const empty of [false, true]) {
      test(`${theme} ${width}px ${empty ? "empty" : "populated/photo/missing-photo"} dashboard AA`, async ({
        page,
      }, testInfo) => {
        await page.setViewportSize({ width, height: 1100 });
        await dashboardFixture(page, empty, theme);
        await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
        await expect(page.locator("section .hb-section-title")).toHaveText([
          "Quick Statistics",
          "Recently Added",
          "Storage Locations",
          "Tags",
        ]);
        await expect(page.locator("header svg[aria-hidden=true]")).toHaveAttribute("focusable", "false");
        await expect(page.locator("section a a, section a button")).toHaveCount(0);
        const search = page.getByRole("searchbox", {
          name: "Search",
          exact: true,
        });
        await search.focus();
        await expect(search).toBeFocused();
        await expect.poll(() => search.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
        const evidence = await contrastEvidence(page);
        const links = page.locator("section a");
        for (let i = 0; i < (await links.count()); i++) {
          const link = links.nth(i);
          await expect(link).toHaveAccessibleName(/\S/);
          await link.hover();
          evidence.push(
            ...(await contrastEvidence(page)).map(result => ({ ...result, label: `hover ${i}: ${result.label}` }))
          );
          await page.keyboard.press("Tab");
          await link.focus();
          await expect.poll(() => link.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
          evidence.push(
            ...(await contrastEvidence(page)).map(result => ({ ...result, label: `focus ${i}: ${result.label}` }))
          );
          if (await link.locator("img").count()) {
            await link.screenshot({ path: testInfo.outputPath("focused-photo-link.png") });
          }
        }
        expect(evidence.length).toBeGreaterThan(10);
        const evidencePath = testInfo.outputPath("computed-contrast.json");
        await writeFile(evidencePath, JSON.stringify(evidence, null, 2));
        await testInfo.attach("computed-contrast.json", {
          path: evidencePath,
          contentType: "application/json",
        });
        for (const result of evidence) expect(result.ratio, result.label).toBeGreaterThanOrEqual(result.threshold);
        await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        await page.reload();
        await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
        await expect(page.locator("section .hb-section-title")).toHaveCount(4);
        await expect(page.locator("section").filter({ hasText: "Recently Added" })).toContainText(
          empty ? "No Items Found" : longName
        );
        await page.evaluate(() => window.scrollTo(0, 0));
        await page.screenshot({
          path: testInfo.outputPath("dashboard.png"),
          fullPage: true,
        });
      });
    }
  }
}

for (const width of [390, 1440]) {
  test(`keyboard-only item/location/tag destinations at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1100 });
    await dashboardFixture(page, false);
    for (const path of [`/item/${itemId}`, "/location/overview-location", "/tag/overview-tag"]) {
      await page.goto("/home");
      await expect(page.locator(`section a[href='${path}']`).first()).toBeVisible();
      await page.getByRole("searchbox", { name: "Search", exact: true }).focus();
      let reached = false;
      for (let i = 0; i < 150; i++) {
        await page.keyboard.press("Tab");
        reached = await page.evaluate(target => document.activeElement?.getAttribute("href") === target, path);
        if (reached) break;
      }
      expect(reached, `Tab reaches ${path}`).toBe(true);
      const focused = page.locator(":focus");
      await expect(focused).toHaveAccessibleName(/\S/);
      await expect.poll(() => focused.evaluate(el => getComputedStyle(el).boxShadow !== "none")).toBe(true);
      await page.keyboard.press("Enter");
      await expect(page).toHaveURL(path);
    }
  });
}

for (const theme of ["light", "cinematic"]) {
  test(`${theme} server-saved preference survives clean storage and real collection switching`, async ({ page }) => {
    // A disposable account avoids mutating the demo user's saved theme or collections.
    const email = `dashboard-${theme}-${Date.now()}@example.com`;
    await page.goto("/");
    await page.getByTestId("register-button").click();
    await page.getByTestId("email-input").locator("input").fill(email);
    await page.getByTestId("name-input").locator("input").fill("Dashboard regression");
    await page.getByTestId("password-input").locator("input").fill("DashboardSecret42!");
    await page.getByTestId("confirm-register-button").click();
    await expect(page).toHaveURL("/");
    await page.locator("#login-username").fill(email);
    await page.locator("#login-password").fill("DashboardSecret42!");
    await page.getByRole("button", { name: "Login", exact: true }).click();
    await expect(page).toHaveURL("/home");
    await page.locator("[data-sidebar=menu-button][href='/profile']").first().click();
    const saved = page.waitForResponse(
      r =>
        r.url().endsWith("/users/self/settings") &&
        r.request().method() === "PUT" &&
        r.request().postDataJSON()?.theme === theme &&
        r.ok()
    );
    await page.locator(`[data-set-theme='${theme}']`).focus();
    await page.keyboard.press("Enter");
    await saved;
    // Clear the device cache before app startup, not while its sync watcher is active.
    await page.addInitScript(() => {
      if (!sessionStorage.getItem("dashboard-cache-cleared")) {
        localStorage.removeItem("homebox/preferences/location");
        sessionStorage.setItem("dashboard-cache-cleared", "true");
      }
    });
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    await page.goto("/home");
    const selector = page.getByRole("combobox", {
      name: "Select Collection",
      exact: true,
    });
    await expect(selector).not.toContainText("Select Collection");
    const original = (await selector.innerText()).trim();
    await selector.click();
    await page.getByText("Create New Collection", { exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("textbox").fill("Dashboard second collection");
    await dialog.getByRole("button", { name: "Create", exact: true }).click();
    await expect(selector).toContainText("Dashboard second collection");
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    await selector.click();
    await page.getByRole("option", { name: original, exact: true }).click();
    await expect(selector).toContainText(original);
    await page.reload();
    await expect(selector).toContainText(original);
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
  });
}
