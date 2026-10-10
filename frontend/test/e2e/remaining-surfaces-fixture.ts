import type { Page } from "@playwright/test";

// Presentation fixture only: no live auth, exports, or QR generation is exercised.
export async function fixture(page: Page, baseURL: string, theme: string) {
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
